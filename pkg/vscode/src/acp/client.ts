import { ChildProcessWithoutNullStreams, spawn } from 'node:child_process';
import { EventEmitter } from 'node:events';
import { createInterface, Interface } from 'node:readline';
import { ContentBlock, ServerRequestHandler } from './types';

const MAX_FRAME = 64 * 1024 * 1024;

interface Pending {
  resolve: (value: any) => void;
  reject: (err: Error) => void;
  timer?: NodeJS.Timeout;
  method: string;
}

export interface AcpLogger {
  debug(message: string, ...args: unknown[]): void;
  info(message: string, ...args: unknown[]): void;
  warn(message: string, ...args: unknown[]): void;
}

export interface AcpClientOptions {
  cwd: string;
  getBinary: () => Promise<string>;
  env?: Record<string, string>;
  log: AcpLogger;
  version?: string;
}

export class AcpClient extends EventEmitter {
  private child: ChildProcessWithoutNullStreams | undefined;
  private out: Interface | undefined;
  private err: Interface | undefined;
  private nextId = 1;
  private pending = new Map<string, Pending>();
  private requestHandler: ServerRequestHandler | undefined;
  private writeChain: Promise<void> = Promise.resolve();
  private started = false;
  private stopped = false;

  constructor(private readonly options: AcpClientOptions) {
    super();
  }

  setRequestHandler(handler: ServerRequestHandler): void {
    this.requestHandler = handler;
  }

  async start(): Promise<void> {
    if (this.started) return;
    const binary = await this.options.getBinary();
    const env: NodeJS.ProcessEnv = { ...process.env, ...(this.options.env ?? {}) };
    this.options.log.info('Starting ACP server', binary, this.options.cwd);
    this.child = spawn(binary, ['serve', '--acp', '--stdio'], {
      cwd: this.options.cwd,
      env,
      stdio: ['pipe', 'pipe', 'pipe'],
      windowsHide: true,
    });
    this.started = true;
    this.stopped = false;

    this.out = createInterface({ input: this.child.stdout, crlfDelay: Infinity });
    this.out.on('line', (line) => this.onLine(line));
    this.err = createInterface({ input: this.child.stderr, crlfDelay: Infinity });
    this.err.on('line', (line) => {
      if (line.trim()) this.options.log.warn('pegg stderr:', line);
    });

    this.child.on('error', (err) => this.failAll(err));
    this.child.on('exit', (code, signal) => {
      if (this.stopped) return;
      this.options.log.warn('ACP server exited', code, signal);
      this.failAll(new Error(`Pegg server exited (${signal ?? code ?? 'unknown'})`));
      this.emit('exit', code, signal);
    });

    this.child.stdin.on('error', (err) => this.options.log.warn('stdin error', err));
  }

  async initialize(): Promise<any> {
    return this.request('initialize', {
      protocolVersion: 1,
      clientInfo: { name: 'pegg-vscode', title: 'Pegg for VS Code', version: this.options.version ?? '0.2.2' },
      clientCapabilities: {
        terminal: true,
        fs: { readTextFile: true, writeTextFile: false },
      },
    });
  }

  request<T = any>(method: string, params: unknown, timeoutMs?: number): Promise<T> {
    if (!this.child || !this.started) {
      return Promise.reject(new Error('Pegg server is not running'));
    }
    const id = `pegg-${this.nextId++}`;
    return new Promise<T>((resolve, reject) => {
      const pending: Pending = { resolve, reject, method };
      if (timeoutMs && timeoutMs > 0) {
        pending.timer = setTimeout(() => {
          this.pending.delete(id);
          reject(new Error(`Pegg request timed out: ${method}`));
        }, timeoutMs);
      }
      this.pending.set(id, pending);
      this.write({ jsonrpc: '2.0', id, method, params }).catch((err) => {
        const p = this.pending.get(id);
        if (p?.timer) clearTimeout(p.timer);
        this.pending.delete(id);
        reject(err);
      });
    });
  }

  notify(method: string, params: unknown): void {
    if (!this.started) return;
    void this.write({ jsonrpc: '2.0', method, params }).catch((err) =>
      this.options.log.warn('notify failed', method, err),
    );
  }

  cancelPrompt(sessionId: string): void {
    this.notify('session/cancel', { sessionId });
  }

  async stop(): Promise<void> {
    this.stopped = true;
    this.started = false;
    this.failAll(new Error('Pegg server stopped'));
    if (this.out) this.out.close();
    if (this.err) this.err.close();
    const child = this.child;
    this.child = undefined;
    if (!child) return;
    child.stdin.end();
    await new Promise<void>((resolve) => {
      const timer = setTimeout(() => {
        child.kill('SIGKILL');
        resolve();
      }, 2000);
      child.once('exit', () => {
        clearTimeout(timer);
        resolve();
      });
      child.kill('SIGTERM');
    });
  }

  private write(message: unknown): Promise<void> {
    const text = `${JSON.stringify(message)}\n`;
    if (text.length > MAX_FRAME) {
      return Promise.reject(new Error('ACP frame exceeds maximum size'));
    }
    const child = this.child;
    if (!child) return Promise.reject(new Error('Pegg server is not running'));
    this.writeChain = this.writeChain.then(
      () =>
        new Promise<void>((resolve, reject) => {
          if (child.stdin.destroyed) {
            reject(new Error('Pegg stdin is closed'));
            return;
          }
          child.stdin.write(text, (err) => (err ? reject(err) : resolve()));
        }),
    );
    return this.writeChain;
  }

  private onLine(line: string): void {
    if (!line.trim()) return;
    if (line.length > MAX_FRAME) {
      this.options.log.warn('Dropping oversized ACP frame');
      return;
    }
    let message: any;
    try {
      message = JSON.parse(line);
    } catch (err) {
      this.options.log.warn('Invalid JSON from Pegg:', line.slice(0, 500), err);
      return;
    }
    this.options.log.debug('ACP <-', message.method ?? message.id ?? 'response');

    if (message.method && message.id !== undefined && message.id !== null) {
      void this.handleServerRequest(message);
      return;
    }
    if (message.method) {
      this.emit('notification', message.method, message.params);
      return;
    }

    const id = String(message.id ?? '');
    const pending = this.pending.get(id);
    if (!pending) return;
    this.pending.delete(id);
    if (pending.timer) clearTimeout(pending.timer);
    if (message.error) {
      pending.reject(new Error(message.error.message || `Pegg request failed: ${pending.method}`));
    } else {
      pending.resolve(message.result);
    }
  }

  private async handleServerRequest(message: any): Promise<void> {
    const handler = this.requestHandler;
    if (!handler) {
      await this.respondError(message.id, -32601, `Unsupported client method: ${message.method}`);
      return;
    }
    try {
      const result = await handler(message.method, message.params);
      await this.write({ jsonrpc: '2.0', id: message.id, result: result ?? {} });
    } catch (err) {
      await this.respondError(message.id, -32603, err instanceof Error ? err.message : String(err));
    }
  }

  private async respondError(id: unknown, code: number, message: string): Promise<void> {
    await this.write({ jsonrpc: '2.0', id, error: { code, message } }).catch((err) =>
      this.options.log.warn('failed to send error response', err),
    );
  }

  private failAll(err: Error): void {
    for (const [id, pending] of this.pending) {
      if (pending.timer) clearTimeout(pending.timer);
      pending.reject(err);
      this.pending.delete(id);
    }
  }
}

export function contentText(text: string): ContentBlock {
  return { type: 'text', text };
}
