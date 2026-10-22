import { ContentBlock, PlanEntry, SessionMode, SessionUpdate, SlashCommand } from '../acp/types';

export interface UsageState {
  used: number;
  size: number;
  cost?: number;
}

export interface ToolState {
  id: string;
  title: string;
  kind?: string;
  status: 'pending' | 'in_progress' | 'completed' | 'failed' | 'cancelled';
  input?: unknown;
  output?: unknown;
  error?: string;
  locations?: { path: string; line?: number }[];
  diff?: { path: string; oldText?: string; newText: string };
  agentName?: string;
}

export type ChatItem =
  | { kind: 'user'; id: string; text: string; createdAt: number; messageId?: string }
  | { kind: 'assistant'; id: string; text: string; agentName?: string; createdAt: number }
  | { kind: 'reasoning'; id: string; text: string; agentName?: string; createdAt: number }
  | { kind: 'tool'; id: string; tool: ToolState; createdAt: number }
  | { kind: 'notice'; id: string; text: string; level: 'info' | 'warn' | 'error'; createdAt: number }
  | { kind: 'error'; id: string; text: string; createdAt: number };

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
  const block = content as ContentBlock;
  if (block.type === 'text') return block.text ?? '';
  return '';
}

export function applyUpdate(input: ChatState, update: SessionUpdate): ChatState {
  const state = { ...input, items: [...input.items], plan: [...input.plan] };
  const now = Date.now();

  switch (update.sessionUpdate) {
    case 'agent_message_chunk':
    case 'user_message_chunk': {
      const text = contentText(update.content);
      if (!text) return state;
      const id = update.messageId || nextId(state, 'message');
      const kind = update.sessionUpdate === 'user_message_chunk' ? 'user' : 'assistant';
      const storedId = update.messageId;
      const existing = state.items.find(
        (item) => item.id === id && (item.kind === 'assistant' || item.kind === 'user'),
      ) as Extract<ChatItem, { kind: 'assistant' | 'user' }> | undefined;
      const last = state.items[state.items.length - 1];
      if (existing) {
        existing.text += text;
      } else if (
        (last?.kind === 'user' && kind === 'user') ||
        (last?.kind === 'assistant' && kind === 'assistant' && (last.agentName ?? '') === (update.agentName ?? ''))
      ) {
        state.items = [
          ...state.items.slice(0, -1),
          { ...(last as Extract<ChatItem, { kind: 'user' | 'assistant' }>), text: last.text + text },
        ];
      } else {
        state.items.push({
          kind,
          id,
          text,
          agentName: update.agentName,
          createdAt: now,
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
      } else if (last?.kind === 'reasoning' && (last.agentName ?? '') === (update.agentName ?? '')) {
        state.items = [
          ...state.items.slice(0, -1),
          { ...last, text: last.text + text },
        ];
      } else {
        state.items.push({
          kind: 'reasoning',
          id,
          text,
          agentName: update.agentName,
          createdAt: now,
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
        if (update.error !== undefined) existing.tool.error = update.error;
        if (update.locations) existing.tool.locations = update.locations;
        if (update.agentName) existing.tool.agentName = update.agentName;
        const diff = update.contents?.find((c) => c.type === 'diff' && c.diff)?.diff;
        if (diff) existing.tool.diff = diff;
      } else {
        const diff = update.contents?.find((c) => c.type === 'diff' && c.diff)?.diff;
        state.items.push({
          kind: 'tool',
          id,
          createdAt: now,
          tool: {
            id,
            title: update.title || update.toolCallId || 'Tool',
            kind: inferToolKind(update.kind, update.title, undefined),
            status: status || 'pending',
            input: update.rawInput,
            output: update.rawOutput,
            error: update.error,
            locations: update.locations,
            diff,
            agentName: update.agentName,
          },
        });
      }
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
      if (update.mode) state.mode = update.mode as SessionMode;
      return state;
    case 'current_model_update':
      if (update.provider) state.provider = update.provider;
      if (update.model) state.model = update.model;
      return state;
    case 'available_commands_update':
      if (update.commands) state.commands = update.commands;
      return state;
    case 'error':
      state.items.push({
        kind: 'error',
        id: nextId(state, 'error'),
        text: update.error || 'Unknown error',
        createdAt: now,
      });
      state.error = update.error;
      return state;
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

export function appendUser(state: ChatState, text: string): ChatState {
  const next = { ...state, items: [...state.items], counter: state.counter + 1 };
  next.items.push({
    kind: 'user',
    id: `user-${next.counter}`,
    text,
    createdAt: Date.now(),
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

export function truncateFrom(state: ChatState, itemId: string): ChatState {
  const index = state.items.findIndex((item) => item.id === itemId);
  if (index < 0) return state;
  return { ...state, items: state.items.slice(0, index) };
}

export function userMessageOrdinal(state: ChatState, itemId: string): number | undefined {
  let ordinal = 0;
  for (const item of state.items) {
    if (item.kind !== 'user') continue;
    if (item.id === itemId) return ordinal;
    ordinal += 1;
  }
  return undefined;
}
