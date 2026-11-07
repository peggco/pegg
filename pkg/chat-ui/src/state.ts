import {
  AttachmentInfo,
  ContentBlock,
  Meta,
  PlanEntry,
  SessionConfigOption,
  SessionMode,
  SessionUpdate,
  SlashCommand,
  ToolCallContent,
} from './types';

export type { AttachmentInfo } from './types';

export interface UsageState {
  used: number;
  size: number;
  cost?: number;
}

export interface ToolState {
  id: string;
  name?: string;
  title: string;
  kind?: string;
  status: 'pending' | 'in_progress' | 'completed' | 'failed' | 'cancelled';
  input?: unknown;
  output?: unknown;
  error?: string;
  locations?: { path: string; line?: number }[];
  diff?: { path: string; oldText?: string; newText: string };
  files?: AttachmentInfo[];
  agentId?: string;
  agentName?: string;
}

export type ChatItem = (
  | {
      kind: 'user';
      id: string;
      text: string;
      createdAt: number;
      messageId?: string;
      agentId?: string;
      attachments?: AttachmentInfo[];
    }
  | { kind: 'assistant'; id: string; text: string; agentId?: string; agentName?: string; createdAt: number }
  | { kind: 'reasoning'; id: string; text: string; agentId?: string; agentName?: string; createdAt: number }
  | { kind: 'tool'; id: string; tool: ToolState; createdAt: number }
  | {
      kind: 'subagent';
      id: string;
      agentId: string;
      agentName?: string;
      status: 'running' | 'finished' | 'error';
      task?: string;
      output?: string;
      error?: string;
      createdAt: number;
    }
  | { kind: 'notice'; id: string; text: string; level: 'info' | 'warn' | 'error'; createdAt: number }
  | { kind: 'error'; id: string; text: string; createdAt: number }
) & { seq?: number };

export interface ComposerAttachment {
  id: string;
  name: string;
  mediaType: string;
  size?: number;
  path?: string;
  data?: string;
  text?: string;
}

export interface PendingPermission {
  kind: 'permission';
  id: string;
  question: string;
  options: { optionId: string; name: string; kind: string }[];
}

export interface PendingQuestion {
  kind: 'question';
  id: string;
  agentName?: string;
  questions: {
    id: string;
    question: string;
    type: string;
    options?: string[];
    required?: boolean;
  }[];
  answers: Record<string, string>;
}

export type PendingInteraction = PendingPermission | PendingQuestion;

export interface ChatState {
  acpSessionId?: string;
  peggSessionId?: string;
  title: string;
  mode: SessionMode;
  provider?: string;
  model?: string;
  permissionMode?: string;
  connected: boolean;
  running: boolean;
  items: ChatItem[];
  plan: PlanEntry[];
  usage: UsageState;
  commands: SlashCommand[];
  pending: PendingInteraction[];
  composerAttachments: ComposerAttachment[];
  error?: string;
  counter: number;
  historySupported?: boolean;
  historyOldestSeq: number;
  historyHasMore: boolean;
  historyLoading: boolean;
}

export function initialState(): ChatState {
  return {
    title: 'New session',
    mode: 'auto',
    connected: false,
    running: false,
    items: [],
    plan: [],
    usage: { used: 0, size: 0 },
    commands: [],
    pending: [],
    composerAttachments: [],
    counter: 0,
    historySupported: false,
    historyOldestSeq: 0,
    historyHasMore: false,
    historyLoading: false,
  };
}

function nextId(state: ChatState, prefix: string): string {
  state.counter += 1;
  return `${prefix}-${state.counter}`;
}

function findItem<T extends ChatItem>(state: ChatState, id: string, kind: T['kind']): T | undefined {
  return state.items.find((item) => item.id === id && item.kind === kind) as T | undefined;
}

function contentText(content: SessionUpdate['content']): string {
  if (typeof content === 'string') return content;
  if (!content) return '';
  if (Array.isArray(content)) {
    return content
      .map((entry) => {
        const block = (entry as ToolCallContent).content;
        return block?.type === 'text' ? block.text ?? '' : '';
      })
      .join('');
  }
  const block = content as ContentBlock;
  if (block.type === 'text') return block.text ?? '';
  return '';
}

function toolContents(content: SessionUpdate['content']): ToolCallContent[] | undefined {
  return Array.isArray(content) ? (content as ToolCallContent[]) : undefined;
}

function metaString(meta: Meta | undefined, key: string): string | undefined {
  const value = meta?.[key];
  return typeof value === 'string' ? value : undefined;
}

function metaNumber(meta: Meta | undefined, key: string): number | undefined {
  const value = meta?.[key];
  return typeof value === 'number' ? value : undefined;
}

function metaAttachments(meta: Meta | undefined): AttachmentInfo[] | undefined {
  const value = meta?.['com.pegg/attachments'];
  return Array.isArray(value) ? (value as AttachmentInfo[]) : undefined;
}

function applyConfigOptions(state: ChatState, options: SessionConfigOption[] | undefined): void {
  if (!options) return;
  for (const option of options) {
    if (option.id === 'mode' && typeof option.currentValue === 'string') {
      state.mode = option.currentValue as SessionMode;
    }
    if (option.id === 'model' && typeof option.currentValue === 'string') {
      state.model = option.currentValue;
    }
  }
}

export function applyUpdate(input: ChatState, update: SessionUpdate): ChatState {
  const state = { ...input, items: [...input.items], plan: [...input.plan] };
  const now = Date.now();
  const meta = update._meta;
  const seq = metaNumber(meta, 'com.pegg/seq');
  const agentId = metaString(meta, 'com.pegg/agentId');
  const agentName = metaString(meta, 'com.pegg/agentName');
  const metaError = metaString(meta, 'com.pegg/error');
  const metaAttachmentsValue = metaAttachments(meta);

  switch (update.sessionUpdate) {
    case 'agent_message_chunk':
    case 'user_message_chunk': {
      const text = contentText(update.content);
      const attachments = metaAttachmentsValue?.length ? metaAttachmentsValue : undefined;
      if (!text && !attachments) return state;
      const id = update.messageId || nextId(state, 'message');
      const kind = update.sessionUpdate === 'user_message_chunk' ? 'user' : 'assistant';
      const storedId = update.messageId;
      const existing = state.items.find(
        (item) => item.id === id && (item.kind === 'assistant' || item.kind === 'user'),
      ) as Extract<ChatItem, { kind: 'assistant' | 'user' }> | undefined;
      const last = state.items[state.items.length - 1];
      if (existing) {
        state.items = state.items.map((item) =>
          item.id === existing.id && (item.kind === 'assistant' || item.kind === 'user')
            ? {
                ...item,
                text: item.text + text,
                ...(attachments ? { attachments: mergeAttachments(item, attachments) } : {}),
              }
            : item,
        );
      } else if (
        (last?.kind === 'user' && kind === 'user') ||
        (last?.kind === 'assistant' && kind === 'assistant' && (last.agentName ?? '') === (agentName ?? ''))
      ) {
        state.items = [
          ...state.items.slice(0, -1),
          {
            ...(last as Extract<ChatItem, { kind: 'user' | 'assistant' }>),
            text: last.text + text,
            ...(attachments && kind === 'user'
              ? { attachments: mergeAttachments(last as ChatItem, attachments) }
              : {}),
          },
        ];
      } else {
        state.items.push({
          kind,
          id,
          text,
          agentId,
          agentName,
          createdAt: now,
          seq,
          ...(kind === 'user' && attachments ? { attachments } : {}),
          ...(kind === 'user' && storedId ? { messageId: storedId } : {}),
        } as ChatItem);
      }
      return state;
    }
    case 'agent_thought_chunk': {
      const text = contentText(update.content);
      if (!text) return state;
      const id = update.messageId || nextId(state, 'thought');
      const existing = findItem<Extract<ChatItem, { kind: 'reasoning' }>>(state, id, 'reasoning');
      const last = state.items[state.items.length - 1];
      if (existing) {
        existing.text += text;
      } else if (last?.kind === 'reasoning' && (last.agentName ?? '') === (agentName ?? '')) {
        state.items = [
          ...state.items.slice(0, -1),
          { ...last, text: last.text + text },
        ];
      } else {
        state.items.push({
          kind: 'reasoning',
          id,
          text,
          agentId,
          agentName,
          createdAt: now,
          seq,
        });
      }
      return state;
    }
    case 'tool_call':
    case 'tool_call_update': {
      const id = update.toolCallId || nextId(state, 'tool');
      const existing = findItem<Extract<ChatItem, { kind: 'tool' }>>(state, id, 'tool');
      const status = normalizeStatus(update.status);
      if (existing) {
        existing.tool.title = update.title || existing.tool.title;
        existing.tool.kind = inferToolKind(update.kind, update.title || existing.tool.title, existing.tool.kind);
        existing.tool.status = status || existing.tool.status;
        if (update.rawInput !== undefined) existing.tool.input = update.rawInput;
        if (update.rawOutput !== undefined) existing.tool.output = update.rawOutput;
        if (metaError !== undefined) existing.tool.error = metaError;
        if (update.locations) existing.tool.locations = update.locations;
        if (update.name) existing.tool.name = update.name;
        if (agentId) existing.tool.agentId = agentId;
        if (agentName) existing.tool.agentName = agentName;
        if (metaAttachmentsValue?.length) {
          existing.tool.files = mergeFiles(existing.tool.files, metaAttachmentsValue);
        }
        const diff = toolContents(update.content)?.find((c) => c.type === 'diff' && c.diff)?.diff;
        if (diff) existing.tool.diff = diff;
      } else {
        const diff = toolContents(update.content)?.find((c) => c.type === 'diff' && c.diff)?.diff;
        state.items.push({
          kind: 'tool',
          id,
          createdAt: now,
          seq,
          tool: {
            id,
            name: update.name,
            title: update.title || update.toolCallId || 'Tool',
            kind: inferToolKind(update.kind, update.title, undefined),
            status: status || 'pending',
            input: update.rawInput,
            output: update.rawOutput,
            error: metaError,
            locations: update.locations,
            diff,
            files: metaAttachmentsValue?.length ? metaAttachmentsValue : undefined,
            agentId,
            agentName,
          },
        });
      }
      return state;
    }
    case 'subagent': {
      const subagentId = agentId ?? '';
      if (!subagentId) return state;
      const status: 'running' | 'finished' | 'error' =
        update.status === 'finished' ? 'finished' : update.status === 'error' ? 'error' : 'running';
      const existing = state.items.find(
        (item) => item.kind === 'subagent' && item.agentId === subagentId,
      ) as Extract<ChatItem, { kind: 'subagent' }> | undefined;
      if (existing) {
        existing.status = status;
        if (agentName) existing.agentName = agentName;
        if (update.title) {
          existing.task = update.title;
          pushSubagentPrompt(state, subagentId, update.title, now);
        }
        if (update.rawOutput !== undefined) existing.output = String(update.rawOutput ?? '');
        if (metaError) existing.error = metaError;
        return state;
      }
      state.items.push({
        kind: 'subagent',
        id: `subagent-${subagentId}`,
        agentId: subagentId,
        agentName,
        status,
        task: update.title,
        output: update.rawOutput === undefined ? undefined : String(update.rawOutput ?? ''),
        error: metaError,
        createdAt: now,
      });
      if (update.title) pushSubagentPrompt(state, subagentId, update.title, now);
      return state;
    }
    case 'plan':
      if (update.entries) state.plan = update.entries;
      return state;
    case 'usage_update':
      state.usage = {
        used: update.used ?? state.usage.used,
        size: update.size ?? state.usage.size,
        cost: update.cost?.amount ?? state.usage.cost,
      };
      return state;
    case 'current_mode_update':
      if (update.currentModeId) state.mode = update.currentModeId as SessionMode;
      return state;
    case 'config_option_update':
      applyConfigOptions(state, update.configOptions);
      return state;
    case 'available_commands_update':
      if (update.availableCommands) state.commands = update.availableCommands;
      return state;
    case 'session_info_update':
      if (update.title) state.title = update.title;
      return state;
    case 'notice': {
      const level: 'info' | 'warn' | 'error' =
        update.severity === 'error' ? 'error' : update.severity === 'warning' ? 'warn' : 'info';
      const text = update.description ? `${update.title ?? ''}: ${update.description}` : update.title ?? '';
      state.items.push({
        kind: 'notice',
        id: nextId(state, 'notice'),
        text,
        level,
        createdAt: now,
      });
      if (level === 'error') state.error = text;
      return state;
    }
    default:
      return state;
  }
}

function inferToolKind(kind: string | undefined, title: string | undefined, previous: string | undefined): string | undefined {
  if (kind && kind !== 'other') return kind;
  const name = (title ?? '').toLowerCase();
  if (/^bash\b|\bbash\b/.test(name)) return 'execute';
  if (/^(read|list)\b/.test(name)) return 'read';
  if (/^(write|edit)\b/.test(name)) return 'edit';
  if (/^(glob|grep)\b/.test(name)) return 'search';
  if (/^(webfetch|websearch)\b/.test(name)) return 'fetch';
  if (/^(task|todo)/.test(name)) return 'think';
  return kind || previous;
}

function normalizeStatus(status: string | undefined): ToolState['status'] | undefined {
  switch (status) {
    case 'pending':
    case 'in_progress':
    case 'completed':
    case 'failed':
    case 'cancelled':
      return status;
    default:
      return undefined;
  }
}

function mergeAttachments(item: ChatItem, attachments: AttachmentInfo[]): AttachmentInfo[] {
  const existing = item.kind === 'user' && item.attachments ? item.attachments : [];
  if (!existing.length) return attachments;
  const seen = new Set(existing.map((att) => `${att.name}|${att.path ?? ''}`));
  const merged = [...existing];
  for (const att of attachments) {
    if (seen.has(`${att.name}|${att.path ?? ''}`)) continue;
    merged.push(att);
  }
  return merged;
}

function mergeFiles(existing: AttachmentInfo[] | undefined, incoming: AttachmentInfo[]): AttachmentInfo[] {
  if (!existing?.length) return incoming;
  const seen = new Set(existing.map((file) => `${file.name}|${file.path ?? ''}`));
  const merged = [...existing];
  for (const file of incoming) {
    if (seen.has(`${file.name}|${file.path ?? ''}`)) continue;
    merged.push(file);
  }
  return merged;
}

export function appendUser(
  state: ChatState,
  text: string,
  displayText?: string,
  attachments?: AttachmentInfo[],
): ChatState {
  const next = { ...state, items: [...state.items], counter: state.counter + 1 };
  next.items.push({
    kind: 'user',
    id: `user-${next.counter}`,
    text: displayText ?? text,
    createdAt: Date.now(),
    ...(attachments?.length ? { attachments } : {}),
  });
  return next;
}

export function addNotice(state: ChatState, text: string, level: 'info' | 'warn' | 'error' = 'info'): ChatState {
  const next = { ...state, items: [...state.items], counter: state.counter + 1 };
  next.items.push({
    kind: 'notice',
    id: `notice-${next.counter}`,
    text,
    level,
    createdAt: Date.now(),
  });
  return next;
}

export function attachmentType(mediaType?: string): 'image' | 'audio' | 'video' | 'file' {
  const mt = (mediaType ?? '').toLowerCase();
  if (mt.startsWith('image/')) return 'image';
  if (mt.startsWith('audio/')) return 'audio';
  if (mt.startsWith('video/')) return 'video';
  return 'file';
}

export function addAttachment(state: ChatState, attachment: ComposerAttachment): ChatState {
  return { ...state, composerAttachments: [...state.composerAttachments, attachment] };
}

export function removeAttachment(state: ChatState, id: string): ChatState {
  return { ...state, composerAttachments: state.composerAttachments.filter((a) => a.id !== id) };
}

export function clearAttachments(state: ChatState): ChatState {
  return { ...state, composerAttachments: [] };
}

export function clearConversation(state: ChatState): ChatState {
  return {
    ...state,
    items: [],
    plan: [],
    usage: { used: 0, size: 0 },
    error: undefined,
    counter: 0,
  };
}

export function applyHistory(state: ChatState, updates: SessionUpdate[]): ChatState {
  let history: ChatState = { ...state, items: [], plan: [], pending: [], composerAttachments: [] };
  for (const update of updates) history = applyUpdate(history, update);
  return {
    ...state,
    items: [...history.items, ...state.items],
    counter: Math.max(state.counter, history.counter),
  };
}

export function truncateFrom(state: ChatState, itemId: string): ChatState {
  const index = state.items.findIndex((item) => item.id === itemId);
  if (index < 0) return state;
  return { ...state, items: state.items.slice(0, index) };
}

export function userMessageOrdinal(state: ChatState, itemId: string): number | undefined {
  let ordinal = 0;
  for (const item of state.items) {
    if (item.kind !== 'user' || item.agentId) continue;
    if (item.id === itemId) return ordinal;
    ordinal += 1;
  }
  return undefined;
}

function pushSubagentPrompt(state: ChatState, agentId: string, text: string, now: number): void {
  const id = `subagent-input-${agentId}`;
  if (state.items.some((item) => item.id === id)) return;
  state.items.push({ kind: 'user', id, text, agentId, createdAt: now });
}

export function itemAgentId(item: ChatItem): string | undefined {
  switch (item.kind) {
    case 'user':
    case 'assistant':
    case 'reasoning':
    case 'subagent':
      return item.agentId;
    case 'tool':
      return item.tool.agentId;
    default:
      return undefined;
  }
}

export function filterConversation(
  items: ChatItem[],
  agentId: string | null,
  subagentIds: Set<string>,
): ChatItem[] {
  return items.filter((item) => {
    const owner = itemAgentId(item);
    const subagentOwned = Boolean(owner && subagentIds.has(owner)) || isNamedSubagent(item);
    if (agentId) {
      if (item.kind === 'subagent') return false;
      return owner === agentId;
    }
    if (item.kind === 'subagent') return true;
    if (subagentOwned) return false;
    return true;
  });
}

function isNamedSubagent(item: ChatItem): boolean {
  if (item.kind !== 'tool') return false;
  const name = item.tool.agentName;
  return Boolean(name) && name !== 'orchestrator';
}
