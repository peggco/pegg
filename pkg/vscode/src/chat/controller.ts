import * as vscode from 'vscode';
import { EventEmitter } from 'node:events';
import { delimiter } from 'node:path';
import { AcpClient, contentText } from '../acp/client';
import {
  AskParams,
  ContentBlock,
  InitializeResult,
  ModelListResult,
  PermissionParams,
  PromptResult,
  SessionListItem,
  SessionListResult,
  SessionMode,
  SessionForkResult,
  SessionNewResult,
  SessionUpdate,
} from '../acp/types';
import { BinaryManager } from '../binary/manager';
import { searchDirs } from '../binary/paths';
import { getConfig, workspaceRoot } from '../config';
import { Logger } from '../logger';
import { activeContext, attachmentToBlock } from './context';
import {
  addNotice,
  appendUser,
  applyUpdate,
  ChatState,
  clearAttachments,
  ComposerAttachment,
  initialState,
  truncateFrom,
  userMessageOrdinal,
  PendingInteraction,
  PendingPermission,
  PendingQuestion,
} from './state';

interface SavedSession {
  acpSessionId?: string;
  peggSessionId?: string;
}

interface PendingResolver {
  resolve: (value: unknown) => void;
  reject: (err: Error) => void;
}

export interface PromptOptions {
  context?: string;
  attachments?: ComposerAttachment[];
  includeActiveContext?: boolean;
}

export class ChatController extends EventEmitter {
  state: ChatState = initialState();
  private client: AcpClient | undefined;
  private capabilities: InitializeResult['agentCapabilities'] | undefined;
  private starting: Promise<void> | undefined;
  private started = false;
  private resolvers = new Map<string, PendingResolver>();
  private uiVisible: () => boolean = () => false;
  private forceFallback = false;
  private legacyStreaming = false;
  private loadingSession = false;
  private binaryFingerprint: string | undefined;
  private voiceStart: Promise<void> | undefined;
  private disposed = false;

  constructor(
    private readonly context: vscode.ExtensionContext,
    private readonly binary: BinaryManager,
    private readonly log: Logger,
  ) {
    super();
  }

  setUiVisible(fn: () => boolean): void {
    this.uiVisible = fn;
  }

  setForceFallback(value: boolean): void {
    this.forceFallback = value;
  }

  async activate(): Promise<void> {
    await this.ensureStarted();
    if (this.state.acpSessionId) return;
    if (this.state.peggSessionId) {
      try {
        await this.resumeSession(this.state.peggSessionId);
        return;
      } catch (err) {
        this.log.warn('Could not resume the current session after the server restart', err);
      }
    }
    await this.newSession();
  }

  async ensureSession(): Promise<void> {
    await this.ensureStarted();
    if (!this.state.acpSessionId) await this.activate();
  }

  async newSession(): Promise<void> {
    await this.ensureStarted();
    const root = workspaceRoot();
    if (!root) throw new Error('Open a folder to use Pegg.');
    const cfg = getConfig();
    const previous = this.state;
    this.state = { ...initialState(), connected: true, mode: cfg.defaultMode };
    this.emitState();
    try {
      const result = await this.client!.request<SessionNewResult>('session/new', { cwd: root }, 60000);
      this.state = {
        ...this.state,
        acpSessionId: result.sessionId,
        peggSessionId: result.peggSessionId,
        mode: (result.mode as SessionMode) || cfg.defaultMode,
        provider: result.provider,
        model: result.model,
        permissionMode: result.permissionMode,
        commands: result.commands ?? [],
        title: 'New session',
      };
      if (this.legacyStreaming) {
        this.state = addNotice(
          this.state,
          'Using an older Pegg binary. Update it for streaming tool details and interactive approvals.',
          'warn',
        );
      }
      this.emitState();
      await this.saveSession();
      if (
        cfg.provider &&
        cfg.model &&
        this.capabilities?.pegg?.models &&
        (cfg.provider !== result.provider || cfg.model !== result.model)
      ) {
        try {
          await this.setModel(cfg.provider, cfg.model);
        } catch (err) {
          this.log.warn('Could not apply preferred model', err);
        }
      }
      this.emit('session', this.state);
    } catch (err) {
      this.state = addNotice(
        previous,
        err instanceof Error ? err.message : String(err),
        'error',
      );
      this.emitState();
      throw err;
    }
  }

  async resumeSession(peggSessionId: string): Promise<void> {
    await this.ensureStarted();
    const root = workspaceRoot();
    if (!root) throw new Error('Open a folder to use Pegg.');
    this.state = {
      ...initialState(),
      acpSessionId: peggSessionId,
      peggSessionId,
      connected: true,
    };
    this.emitState();
    this.loadingSession = true;
    try {
      await this.client!.request('session/load', { sessionId: peggSessionId, cwd: root }, 60000);
    } finally {
      this.loadingSession = false;
    }
    const sessions = await this.listSessions(false).catch(() => []);
    const meta = sessions.find((s) => s.id === peggSessionId);
    this.state = {
      ...this.state,
      title: meta?.title || 'Session',
      provider: meta?.provider,
      model: meta?.model,
      mode: (meta?.mode as SessionMode) || this.state.mode,
      usage: meta?.usage
        ? {
            used: meta.usage.total_tokens ?? 0,
            size: 0,
            cost: meta.usage.cost,
          }
        : this.state.usage,
    };
    if (this.legacyStreaming) {
      this.state = addNotice(
        this.state,
        'Using an older Pegg binary. Update it for streaming tool details and interactive approvals.',
        'warn',
      );
    }
    this.emitState();
    await this.saveSession();
    this.emit('session', this.state);
  }

  async listSessions(all = false): Promise<SessionListItem[]> {
    await this.ensureStarted();
    const root = workspaceRoot();
    const result = await this.client!.request<SessionListResult>(
      'session/list',
      { cwd: root, all, limit: 100 },
      60000,
    );
    return result.sessions ?? [];
  }

  async deleteSession(peggSessionId: string): Promise<void> {
    await this.ensureStarted();
    if (this.state.peggSessionId === peggSessionId) {
      if (this.state.acpSessionId) {
        await this.client!.request('session/close', { sessionId: this.state.acpSessionId }).catch(() => undefined);
      }
    }
    await this.client!.request('session/delete', { sessionId: peggSessionId }, 60000);
    if (this.state.peggSessionId === peggSessionId) {
      await this.newSession();
    }
    this.emit('sessions');
  }

  async renameSession(peggSessionId: string, title: string): Promise<void> {
    await this.ensureStarted();
    const sessionId = this.state.peggSessionId === peggSessionId ? this.state.acpSessionId : peggSessionId;
    await this.client!.request('session/rename', { sessionId, title }, 60000);
    if (this.state.peggSessionId === peggSessionId) {
      this.state = { ...this.state, title };
      this.emitState();
    }
    this.emit('sessions');
  }

  async sendPrompt(text: string, options: PromptOptions = {}): Promise<void> {
    await this.ensureSession();
    if (this.state.running) throw new Error('Pegg is already working.');
    if (!text.trim() && !(options.attachments?.length ?? 0)) return;

    const cfg = getConfig();
    const display = text.trim() || '(attachments)';
    this.state = appendUser(this.state, display);
    this.state = { ...this.state, running: true, error: undefined };
    this.emitState();

    const blocks: ContentBlock[] = [];
    const promptParts: string[] = [];
    if (text.trim()) promptParts.push(text);
    const context = options.context ?? (options.includeActiveContext === false ? '' : activeContext());
    if (context) promptParts.push(context);
    if (promptParts.length > 0) blocks.push(contentText(promptParts.join('\n\n')));

    for (const attachment of options.attachments ?? this.state.composerAttachments) {
      try {
        const block = await attachmentToBlock(attachment);
        if (block) blocks.push(block);
      } catch (err) {
        this.log.warn('Failed to attach file', attachment.name, err);
        blocks.push(contentText(`[Could not read attachment ${attachment.name}]`));
      }
    }
    this.state = clearAttachments(this.state);

    try {
      const result = await this.client!.request<PromptResult>('session/prompt', {
        sessionId: this.state.acpSessionId,
        prompt: blocks,
      });
      this.finishRun(result);
      if (cfg.notifyOnComplete && !vscode.window.state.focused) {
        const summary = this.lastAssistantText() || 'Task completed';
        void vscode.window
          .showInformationMessage(`Pegg finished: ${truncate(summary, 120)}`, 'Open Chat')
          .then((choice) => {
            if (choice === 'Open Chat') void vscode.commands.executeCommand('pegg.chat.open');
          });
      }
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      if (/Pegg server (exited|stopped)/i.test(message)) {
        this.log.warn('The Pegg server exited during the run; the session will resume on the next message.', err);
        this.state = { ...this.state, running: false, connected: false };
      } else {
        this.state = addNotice({ ...this.state, running: false }, message, 'error');
      }
      this.emitState();
      throw err;
    }
  }

  async cancel(): Promise<void> {
    if (!this.client || !this.state.acpSessionId) return;
    this.client.cancelPrompt(this.state.acpSessionId);
    this.rejectPending('Cancelled');
    this.state = { ...this.state, running: false };
    this.emitState();
  }

  async setMode(mode: SessionMode): Promise<void> {
    await this.ensureSession();
    await this.client!.request('session/set_mode', { sessionId: this.state.acpSessionId, modeId: mode }, 30000);
    this.state = { ...this.state, mode };
    this.emitState();
  }

  async setModel(provider: string, model: string, reasoningEffort?: string): Promise<void> {
    await this.ensureSession();
    if (!this.capabilities?.pegg?.models) {
      throw new Error('This Pegg binary does not support model switching. Update the Pegg binary first.');
    }
    await this.client!.request(
      'session/set_model',
      { sessionId: this.state.acpSessionId, provider, model, reasoningEffort },
      30000,
    );
    this.state = { ...this.state, provider, model };
    this.emitState();
    await this.saveSession();
  }

  async models(): Promise<ModelListResult['models']> {
    await this.ensureStarted();
    if (!this.capabilities?.pegg?.models) {
      throw new Error('This Pegg binary does not support model listing. Update the Pegg binary first.');
    }
    const result = await this.client!.request<ModelListResult>('pegg/models', {}, 60000);
    return result.models ?? [];
  }

  async startVoice(): Promise<void> {
    if (this.voiceStart) throw new Error('A recording is already in progress.');
    this.voiceStart = (async () => {
      await this.ensureStarted();
      await this.client!.request('stt/record/start', {}, 30000);
    })();
    try {
      await this.voiceStart;
    } catch (err) {
      this.voiceStart = undefined;
      throw err;
    }
  }

  async stopVoice(): Promise<string> {
    if (this.voiceStart) await this.voiceStart.catch(() => undefined);
    this.voiceStart = undefined;
    await this.ensureStarted();
    const result = await this.client!.request<{ text: string }>('stt/record/stop', {}, 120000);
    return result.text;
  }

  async transcribe(audio: string, mediaType: string, fileName?: string): Promise<string> {
    await this.ensureStarted();
    const result = await this.client!.request<{ text: string }>(
      'stt/transcribe',
      { audio, mediaType, fileName },
      120000,
    );
    return result.text;
  }

  get historySupported(): boolean {
    return Boolean(this.capabilities?.pegg?.history);
  }

  async forkMessage(itemId: string): Promise<void> {
    const item = this.state.items.find((entry) => entry.id === itemId);
    if (!item || item.kind !== 'user') throw new Error('Message not found');
    await this.ensureSession();
    if (this.state.running) throw new Error('Wait for Pegg to finish before forking.');
    if (!this.capabilities?.pegg?.history) {
      throw new Error('This Pegg binary does not support forking. Update it first.');
    }
    const result = await this.client!.request<SessionForkResult>('session/fork', this.messageRef(itemId), 60000);
    await this.resumeSession(result.sessionId);
    this.emit('composerText', item.text);
  }

  async revertMessage(itemId: string): Promise<void> {
    const item = this.state.items.find((entry) => entry.id === itemId);
    if (!item || item.kind !== 'user') throw new Error('Message not found');
    await this.ensureSession();
    if (this.state.running) throw new Error('Wait for Pegg to finish before reverting.');
    if (!this.capabilities?.pegg?.history) {
      throw new Error('This Pegg binary does not support reverting. Update it first.');
    }
    await this.client!.request('session/revert', this.messageRef(itemId), 60000);
    this.state = truncateFrom(this.state, itemId);
    this.state = { ...this.state, usage: { used: 0, size: 0 }, plan: [] };
    this.emitState();
    this.emit('session', this.state);
    this.emit('composerText', item.text);
  }

  private messageRef(itemId: string): { sessionId: string; messageId?: string; userMessageIndex?: number } {
    const ref: { sessionId: string; messageId?: string; userMessageIndex?: number } = {
      sessionId: this.state.acpSessionId ?? '',
    };
    const item = this.state.items.find((entry) => entry.id === itemId);
    if (item?.kind === 'user' && item.messageId) {
      ref.messageId = item.messageId;
      return ref;
    }
    const ordinal = userMessageOrdinal(this.state, itemId);
    if (ordinal !== undefined) ref.userMessageIndex = ordinal;
    return ref;
  }

  async restart(): Promise<void> {
    this.log.info('Restarting Pegg server');
    this.rejectPending('Server restarting');
    if (this.client) await this.client.stop();
    this.client = undefined;
    this.started = false;
    this.starting = undefined;
    this.state = { ...this.state, connected: false, running: false };
    this.emitState();
    const peggId = this.state.peggSessionId;
    await this.ensureStarted();
    if (peggId) {
      await this.resumeSession(peggId).catch(async () => this.newSession());
    } else {
      await this.newSession();
    }
  }

  async dispose(): Promise<void> {
    this.disposed = true;
    this.voiceStart = undefined;
    this.rejectPending('Extension disposed');
    if (this.client) await this.client.stop();
  }

  addComposerAttachment(attachment: ComposerAttachment): void {
    this.state = {
      ...this.state,
      composerAttachments: [...this.state.composerAttachments, attachment],
    };
    this.emitState();
  }

  removeComposerAttachment(id: string): void {
    this.state = {
      ...this.state,
      composerAttachments: this.state.composerAttachments.filter((a) => a.id !== id),
    };
    this.emitState();
  }

  respondPermission(id: string, optionId: string): void {
    const item = this.state.pending.find((p): p is PendingPermission => p.kind === 'permission' && p.id === id);
    if (!item) return;
    const resolver = this.resolvers.get(id);
    this.resolvers.delete(id);
    this.state = { ...this.state, pending: this.state.pending.filter((p) => p.id !== id) };
    this.emitState();
    resolver?.resolve({ outcome: { outcome: 'selected', optionId } });
  }

  respondQuestion(id: string, answers: Record<string, string>): void {
    const item = this.state.pending.find((p): p is PendingQuestion => p.kind === 'question' && p.id === id);
    if (!item) return;
    const resolver = this.resolvers.get(id);
    this.resolvers.delete(id);
    this.state = { ...this.state, pending: this.state.pending.filter((p) => p.id !== id) };
    this.emitState();
    resolver?.resolve({ answers });
  }

  private async ensureStarted(): Promise<void> {
    if (this.disposed) throw new Error('Pegg controller is disposed');
    if (this.started) {
      const current = await this.binary.fingerprint();
      if (current && this.binaryFingerprint && current !== this.binaryFingerprint) {
        this.log.info('Pegg binary changed on disk; restarting ACP server');
        await this.restart();
        return;
      }
      return;
    }
    if (!this.starting) {
      this.starting = this.startClient().finally(() => {
        this.starting = undefined;
      });
    }
    await this.starting;
  }

  private async serverEnv(): Promise<Record<string, string>> {
    const env: Record<string, string> = {};
    try {
      env.PATH = (await searchDirs()).join(delimiter);
    } catch {
      // fall back to the inherited environment
    }
    return { ...env, ...getConfig().env };
  }

  private async startClient(): Promise<void> {
    const root = workspaceRoot();
    if (!root) throw new Error('Open a folder to use Pegg.');
    if (!vscode.workspace.isTrusted) {
      throw new Error('Pegg requires a trusted workspace.');
    }
    const client = new AcpClient({
      cwd: root,
      getBinary: async () => {
        const binaryPath = await this.binary.ensure();
        this.log.info('Using Pegg binary', binaryPath, this.binary.currentVersion());
        return binaryPath;
      },
      env: await this.serverEnv(),
      log: this.log,
      version: String(this.context.extension.packageJSON.version ?? '0.2.1'),
    });
    client.setRequestHandler((method, params) => this.onServerRequest(method, params));
    client.on('notification', (method: string, params: any) => this.onNotification(method, params));
    client.on('exit', () => {
      this.started = false;
      this.state = {
        ...this.state,
        acpSessionId: undefined,
        connected: false,
        running: false,
      };
      this.emitState();
    });
    this.client = client;
    try {
      await client.start();
      this.binaryFingerprint = await this.binary.fingerprint();
      const capabilities = await client.initialize();
      this.capabilities = capabilities.agentCapabilities;
      this.log.info('Pegg capabilities', JSON.stringify(capabilities.agentCapabilities?.pegg ?? {}));
      this.started = true;
      this.state = { ...this.state, connected: true };
      this.emitState();
      const pegg = capabilities.agentCapabilities?.pegg;
      this.legacyStreaming = !pegg;
      if (!pegg?.permissions) {
        this.log.warn('The installed Pegg CLI does not advertise interactive permission support.');
        void vscode.window.showWarningMessage(
          'This Pegg CLI is older than the extension. Update it with "Pegg: Install or Update Binary" for streaming tool details and interactive approvals.',
        );
      }
    } catch (err) {
      this.started = false;
      this.client = undefined;
      await client.stop().catch(() => undefined);
      this.state = addNotice(
        { ...this.state, connected: false },
        err instanceof Error ? err.message : String(err),
        'error',
      );
      this.emitState();
      throw err;
    }
  }

  private onNotification(method: string, params: any): void {
    if (method !== 'session/update') {
      this.log.debug('ACP notification', method);
      return;
    }
    const sessionId = params?.sessionId;
    const update = params?.update as SessionUpdate | undefined;
    if (!update) return;
    if (sessionId && this.state.acpSessionId && sessionId !== this.state.acpSessionId) return;

    if (update.sessionUpdate === 'run_ask_resolved') {
      this.rejectPending('Answered in another client');
      this.emit('update', update, this.state);
      return;
    }

    if (update.sessionUpdate === 'run_state_update') {
      this.state = { ...this.state, running: update.runState === 'running' };
      this.emit('update', update, this.state);
      this.emitState();
      return;
    }

    if (
      this.legacyStreaming &&
      !this.loadingSession &&
      update.sessionUpdate === 'agent_message_chunk' &&
      !update.agentName
    ) {
      const text = sessionUpdateText(update.content);
      const last = this.state.items[this.state.items.length - 1];
      if (text && last?.kind === 'assistant') {
        this.state = {
          ...this.state,
          items: [
            ...this.state.items.slice(0, -1),
            { ...last, text: last.text + text },
          ],
        };
        this.emit('update', update, this.state);
        this.emitState();
        return;
      }
    }

    this.state = applyUpdate(this.state, update);
    this.emit('update', update, this.state);
    this.emitState();
  }

  private async onServerRequest(method: string, params: unknown): Promise<unknown> {
    if (method === 'session/request_permission') {
      return this.requestPermission(params as PermissionParams);
    }
    if (method === 'pegg/ask') {
      return this.requestAnswers(params as AskParams);
    }
    throw new Error(`Unsupported client method: ${method}`);
  }

  private requestPermission(params: PermissionParams): Promise<unknown> {
    const id = `perm-${Date.now()}-${Math.random().toString(36).slice(2, 7)}`;
    const pending: PendingPermission = {
      kind: 'permission',
      id,
      question: params.question || params.toolCall?.title || 'Allow this tool call?',
      options: params.options ?? [
        { optionId: 'allow_once', name: 'Allow', kind: 'allow_once' },
        { optionId: 'allow_always', name: 'Allow All', kind: 'allow_always' },
        { optionId: 'reject_once', name: 'Reject', kind: 'reject_once' },
      ],
    };
    return new Promise((resolve, reject) => {
      this.resolvers.set(id, { resolve, reject });
      this.state = { ...this.state, pending: [...this.state.pending, pending] };
      this.emitState();
      if (this.forceFallback || !this.uiVisible()) void this.fallbackPermission(pending);
    });
  }

  private requestAnswers(params: AskParams): Promise<unknown> {
    const id = `ask-${Date.now()}-${Math.random().toString(36).slice(2, 7)}`;
    const pending: PendingQuestion = {
      kind: 'question',
      id,
      agentName: params.agentName,
      questions: params.questions ?? [],
      answers: {},
    };
    return new Promise((resolve, reject) => {
      this.resolvers.set(id, { resolve, reject });
      this.state = { ...this.state, pending: [...this.state.pending, pending] };
      this.emitState();
      if (this.forceFallback || !this.uiVisible()) void this.fallbackQuestions(pending);
    });
  }

  private async fallbackPermission(pending: PendingPermission): Promise<void> {
    const choice = await vscode.window.showWarningMessage(
      pending.question,
      { modal: true },
      ...pending.options.map((o) => o.name),
    );
    const option = pending.options.find((o) => o.name === choice) ?? pending.options.find((o) => o.kind.startsWith('reject'));
    this.respondPermission(pending.id, option?.optionId ?? 'reject_once');
  }

  private async fallbackQuestions(pending: PendingQuestion): Promise<void> {
    const answers: Record<string, string> = {};
    for (const question of pending.questions) {
      if (question.type === 'text') {
        const value = await vscode.window.showInputBox({
          prompt: question.question,
          ignoreFocusOut: true,
        });
        answers[question.id] = value ?? '';
      } else if (question.type === 'boolean') {
        const value = await vscode.window.showQuickPick(['Yes', 'No'], { title: question.question });
        answers[question.id] = value === 'Yes' ? 'true' : 'false';
      } else {
        const picks = question.options ?? [];
        if (question.type === 'multiselect') {
          const values = await vscode.window.showQuickPick(picks, {
            title: question.question,
            canPickMany: true,
          });
          answers[question.id] = (values ?? []).join(', ');
        } else {
          const value = await vscode.window.showQuickPick(picks, { title: question.question });
          answers[question.id] = value ?? '';
        }
      }
    }
    this.respondQuestion(pending.id, answers);
  }

  private finishRun(result: PromptResult): void {
    this.state = { ...this.state, running: false };
    this.state = {
      ...this.state,
      items: this.state.items.map((item) => {
        if (item.kind !== 'tool') return item;
        if (item.tool.status !== 'pending' && item.tool.status !== 'in_progress') return item;
        return { ...item, tool: { ...item.tool, status: result.stopReason === 'cancelled' ? 'cancelled' : 'completed' } };
      }),
    };
    if (result.usage && result.usage.used > 0) {
      this.state = {
        ...this.state,
        usage: {
          used: result.usage.used,
          size: result.usage.size,
          cost: result.usage.cost?.amount,
        },
      };
    }
    if (result.stopReason === 'error') {
      this.state = addNotice(this.state, 'Pegg stopped with an error.', 'error');
    } else if (result.stopReason === 'cancelled') {
      this.state = addNotice(this.state, 'Run cancelled.', 'warn');
    }
    this.emitState();
    this.emit('runFinished', this.state);
  }

  private rejectPending(reason: string): void {
    for (const [id, resolver] of this.resolvers) {
      resolver.reject(new Error(reason));
      this.resolvers.delete(id);
    }
    if (this.state.pending.length > 0) {
      this.state = { ...this.state, pending: [] };
      this.emitState();
    }
  }

  private lastAssistantText(): string {
    for (let i = this.state.items.length - 1; i >= 0; i--) {
      const item = this.state.items[i];
      if (item.kind === 'assistant') return item.text;
    }
    return '';
  }

  private emitState(): void {
    this.state = { ...this.state, historySupported: this.historySupported };
    this.emit('state', this.state);
  }

  private async saveSession(): Promise<void> {
    await this.context.workspaceState.update('pegg.session', {
      acpSessionId: this.state.acpSessionId,
      peggSessionId: this.state.peggSessionId,
    } satisfies SavedSession);
  }
}

function sessionUpdateText(content: SessionUpdate['content']): string {
  if (typeof content === 'string') return content;
  if (content && typeof content === 'object' && content.type === 'text') return content.text ?? '';
  return '';
}

function truncate(text: string, max: number): string {
  const single = text.replace(/\s+/g, ' ').trim();
  return single.length > max ? `${single.slice(0, max - 1)}…` : single;
}

export type { PendingInteraction };
