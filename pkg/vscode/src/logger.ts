import * as vscode from 'vscode';
import { getConfig } from './config';

const SECRET_PATTERNS: Array<[RegExp, string]> = [
  [/(sk-[A-Za-z0-9_-]{12,})/g, 'sk-***'],
  [/(Bearer\s+)[A-Za-z0-9._-]{8,}/gi, '$1***'],
  [/(api[_-]?key["'\s:=]+)[A-Za-z0-9._-]{8,}/gi, '$1***'],
  [/(eyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,})/g, '***JWT***'],
];

export function redact(value: string): string {
  let out = value;
  for (const [re, replacement] of SECRET_PATTERNS) out = out.replace(re, replacement);
  return out;
}

export class Logger {
  readonly channel: vscode.LogOutputChannel;

  constructor() {
    this.channel = vscode.window.createOutputChannel('Pegg', { log: true });
  }

  private level(): number {
    const levels = { error: 0, warn: 1, info: 2, debug: 3 } as const;
    return levels[getConfig().logLevel];
  }

  debug(message: string, ...args: unknown[]): void {
    if (this.level() >= 3) this.channel.debug(redact(format(message, args)));
  }

  info(message: string, ...args: unknown[]): void {
    if (this.level() >= 2) this.channel.info(redact(format(message, args)));
  }

  warn(message: string, ...args: unknown[]): void {
    if (this.level() >= 1) this.channel.warn(redact(format(message, args)));
  }

  error(message: string, ...args: unknown[]): void {
    this.channel.error(redact(format(message, args)));
  }

  show(): void {
    this.channel.show(true);
  }

  dispose(): void {
    this.channel.dispose();
  }
}

function format(message: string, args: unknown[]): string {
  if (args.length === 0) return message;
  return `${message} ${args
    .map((a) => {
      if (a instanceof Error) return a.stack || a.message;
      if (typeof a === 'string') return a;
      try {
        return JSON.stringify(a);
      } catch {
        return String(a);
      }
    })
    .join(' ')}`;
}
