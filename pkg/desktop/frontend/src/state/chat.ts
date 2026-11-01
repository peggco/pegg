import type {
  AskParams,
  ContentBlock,
  PermissionParams,
  PlanEntry,
  SessionMode,
  SessionUpdate,
  SlashCommand,
} from '../types';

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
  agentId?: string;
  agentName?: string;
}

export type ChatItem = (
  | { kind: 'user'; id: string; text: string; createdAt: number; messageId?: string; agentId?: string }
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

export interface SubagentRun {
  agentId: string;
  agentName?: string;
  status: 'running' | 'finished' | 'error';
  task?: string;
  output?: string;
  error?: string;
  items: ChatItem[];
  counter: number;
}

export interface PendingPermission {
  requestId: string;
  params: PermissionParams;
}

export interface PendingAsk {
  requestId: string;
  params: AskParams;
  answers: Record<string, string>;
}

export interface ChatState {
  acpId?: string;
  peggId?: string;
  title: string;
  cwd?: string;
  mode: SessionMode;
  provider?: string;
  model?: string;
  reasoningEffort?: string;
  permissionMode?: string;
  running: boolean;
  loading: boolean;
  items: ChatItem[];
  plan: PlanEntry[];
  usage: UsageState;
  commands: SlashCommand[];
  permissions: PendingPermission[];
  asks: PendingAsk[];
  subagents: Record<string, SubagentRun>;
  error?: string;
  counter: number;
  historyOldestSeq: number;
  historyHasMore: boolean;
  historyLoading: boolean;
  stickToBottom: boolean;
}

export function emptyChat(): ChatState {
  return {
    title: 'New session',
    mode: 'auto',
    running: false,
    loading: false,
    items: [],
    plan: [],
    usage: { used: 0, size: 0 },
    commands: [],
    permissions: [],
    asks: [],
    subagents: {},
    counter: 0,
    historyOldestSeq: 0,
    historyHasMore: false,
    historyLoading: false,
    stickToBottom: true,
  };
}

function nextId(state: ChatState, prefix: string): string {
  state.counter += 1;
  return `${prefix}-${state.counter}`;
}

function contentText(content: SessionUpdate['content']): string {
  if (typeof content === 'string') return content;
  if (!content) return '';
  const block = content as ContentBlock;
  return block.type === 'text' ? (block.text ?? '') : '';
}

function inferToolKind(kind: string | undefined, title: string | undefined, previous?: string): string | undefined {
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

function isTerminalStatus(status: ToolState['status']): boolean {
  return status === 'completed' || status === 'failed' || status === 'cancelled';
}

function findIndex<T extends ChatItem['kind']>(items: ChatItem[], id: string, kind: T): number {
  return items.findIndex((item) => item.id === id && item.kind === kind);
}

function settleTools(items: ChatItem[], status: ToolState['status']): ChatItem[] {
  let changed = false;
  const next = items.map((item) => {
    if (item.kind !== 'tool') return item;
    if (item.tool.status !== 'pending' && item.tool.status !== 'in_progress') return item;
    changed = true;
    return { ...item, tool: { ...item.tool, status } };
  });
  return changed ? next : items;
}

export function settlePendingTools(state: ChatState, status: ToolState['status'] = 'completed'): ChatState {
  const subagents: Record<string, SubagentRun> = {};
  let subChanged = false;
  for (const [id, run] of Object.entries(state.subagents)) {
    const items = settleTools(run.items, status);
    if (items !== run.items) subChanged = true;
    subagents[id] = items === run.items ? run : { ...run, items };
  }
  return {
    ...state,
    items: settleTools(state.items, status),
    ...(subChanged ? { subagents } : {}),
  };
}

export const MAX_CHAT_ITEMS = 160;

export function pruneChat(state: ChatState): ChatState {
  if (state.items.length <= MAX_CHAT_ITEMS) return state;
  let drop = state.items.length - MAX_CHAT_ITEMS;
  let i = 0;
  let droppedSeq = 0;
  while (drop > 0 && i < state.items.length && state.items[i].seq !== undefined) {
    droppedSeq = state.items[i].seq ?? droppedSeq;
    i++;
    drop--;
  }
  if (i === 0) return state;
  const items = state.items.slice(i);
  const firstSeq = items.find((item) => item.seq !== undefined)?.seq;
  return {
    ...state,
    items,
    historyOldestSeq: firstSeq ?? (droppedSeq > 0 ? droppedSeq + 1 : state.historyOldestSeq),
  };
}

export function applyUpdate(input: ChatState, update: SessionUpdate): ChatState {
  if (update.agentId && update.sessionUpdate !== 'subagent' && input.subagents[update.agentId]) {
    return applySubagentUpdate(input, update, update.agentId);
  }
  let state: ChatState = { ...input, items: input.items, plan: input.plan };
  const now = Date.now();
  const seq = update.seq && update.seq > 0 ? update.seq : undefined;

  const replaceItem = (index: number, next: ChatItem) => {
    const items = state.items.slice();
    items[index] = next;
    state = { ...state, items };
  };
  const pushItem = (next: ChatItem) => {
    state = { ...state, items: [...state.items, next] };
  };

  switch (update.sessionUpdate) {
    case 'agent_message_chunk':
    case 'user_message_chunk': {
      const text = contentText(update.content);
      if (!text) return state;
      const kind = update.sessionUpdate === 'user_message_chunk' ? 'user' : 'assistant';
      const id = update.messageId || nextId(state, 'message');
      const idx = findIndex(state.items, id, kind);
      const last = state.items[state.items.length - 1];
      if (idx >= 0) {
        const item = state.items[idx] as Extract<ChatItem, { kind: 'user' | 'assistant' }>;
        replaceItem(idx, { ...item, text: item.text + text });
      } else if (
        (last?.kind === 'user' && kind === 'user') ||
        (last?.kind === 'assistant' &&
          kind === 'assistant' &&
          (last.agentName ?? '') === (update.agentName ?? ''))
      ) {
        replaceItem(state.items.length - 1, { ...last, text: last.text + text } as ChatItem);
      } else {
        if (kind === 'assistant') {
          state = { ...state, items: settleTools(state.items, 'completed') };
        }
        pushItem({
          kind,
          id,
          text,
          agentId: update.agentId,
          agentName: update.agentName,
          createdAt: now,
          seq,
          ...(kind === 'user' && update.messageId ? { messageId: update.messageId } : {}),
        } as ChatItem);
      }
      return state;
    }
    case 'agent_thought_chunk': {
      const text = contentText(update.content);
      if (!text) return state;
      const id = update.messageId || nextId(state, 'thought');
      const idx = findIndex(state.items, id, 'reasoning');
      const last = state.items[state.items.length - 1];
      if (idx >= 0) {
        const item = state.items[idx] as Extract<ChatItem, { kind: 'reasoning' }>;
        replaceItem(idx, { ...item, text: item.text + text });
      } else if (last?.kind === 'reasoning' && (last.agentName ?? '') === (update.agentName ?? '')) {
        replaceItem(state.items.length - 1, { ...last, text: last.text + text });
      } else {
        pushItem({
          kind: 'reasoning',
          id,
          text,
          agentId: update.agentId,
          agentName: update.agentName,
          createdAt: now,
          seq,
        });
      }
      return state;
    }
    case 'tool_call':
    case 'tool_call_update': {
      const id = update.toolCallId || nextId(state, 'tool');
      const idx = findIndex(state.items, id, 'tool');
      const status = normalizeStatus(update.status);
      if (idx >= 0) {
        const item = state.items[idx] as Extract<ChatItem, { kind: 'tool' }>;
        let nextStatus = status || item.tool.status;
        if (isTerminalStatus(item.tool.status) && (!status || !isTerminalStatus(status))) {
          nextStatus = item.tool.status;
        }
        const tool: ToolState = {
          ...item.tool,
          title: update.title || item.tool.title,
          kind: inferToolKind(update.kind, update.title || item.tool.title, item.tool.kind),
          status: nextStatus,
        };
        if (update.rawInput !== undefined) tool.input = update.rawInput;
        if (update.rawOutput !== undefined) tool.output = update.rawOutput;
        if (update.error !== undefined) tool.error = update.error;
        if (update.locations) tool.locations = update.locations;
        if (update.toolName) tool.name = update.toolName;
        if (update.agentId) tool.agentId = update.agentId;
        if (update.agentName) tool.agentName = update.agentName;
        const diff = update.contents?.find((c) => c.type === 'diff' && c.diff)?.diff;
        if (diff) tool.diff = diff;
        replaceItem(idx, { ...item, tool });
      } else {
        pushItem({
          kind: 'tool',
          id,
          createdAt: now,
          seq,
          tool: {
            id,
            name: update.toolName,
            title: update.title || update.toolName || 'Tool call',
            kind: inferToolKind(update.kind, update.title),
            status: status ?? 'pending',
            input: update.rawInput,
            output: update.rawOutput,
            error: update.error,
            locations: update.locations,
            diff: update.contents?.find((c) => c.type === 'diff' && c.diff)?.diff,
            agentId: update.agentId,
            agentName: update.agentName,
          },
        });
      }
      return state;
    }
    case 'subagent': {
      const agentId = update.agentId ?? '';
      if (!agentId) return state;
      const status: 'running' | 'finished' | 'error' =
        update.status === 'finished' ? 'finished' : update.status === 'error' ? 'error' : 'running';
      const existingRun = state.subagents[agentId];
      const settleStatus: ToolState['status'] | undefined =
        status === 'finished' ? 'completed' : status === 'error' ? 'failed' : undefined;
      const run: SubagentRun = existingRun
        ? {
            ...existingRun,
            status,
            agentName: update.agentName || existingRun.agentName,
            task: update.title || existingRun.task,
            output: update.rawOutput !== undefined ? String(update.rawOutput ?? '') : existingRun.output,
            error: update.error || existingRun.error,
            items: settleStatus ? settleTools(existingRun.items, settleStatus) : existingRun.items,
          }
        : {
            agentId,
            agentName: update.agentName,
            status,
            task: update.title,
            output: update.rawOutput === undefined ? undefined : String(update.rawOutput ?? ''),
            error: update.error,
            items: [],
            counter: 0,
          };
      state = { ...state, subagents: { ...state.subagents, [agentId]: run } };
      const idx = state.items.findIndex((item) => item.kind === 'subagent' && item.agentId === agentId);
      if (idx >= 0) {
        const item = state.items[idx] as Extract<ChatItem, { kind: 'subagent' }>;
        replaceItem(idx, {
          ...item,
          status,
          agentName: update.agentName || item.agentName,
          task: update.title || item.task,
          output: update.rawOutput !== undefined ? String(update.rawOutput ?? '') : item.output,
          error: update.error || item.error,
        });
      } else {
        pushItem({
          kind: 'subagent',
          id: `subagent-${agentId}`,
          agentId,
          agentName: update.agentName,
          status,
          task: update.title,
          output: update.rawOutput === undefined ? undefined : String(update.rawOutput ?? ''),
          error: update.error,
          createdAt: now,
        });
      }
      return state;
    }
    case 'plan':
      if (update.entries) state = { ...state, plan: update.entries };
      return state;
    case 'usage_update':
      state = {
        ...state,
        usage: {
          used: update.used ?? state.usage.used,
          size: update.size ?? state.usage.size,
          cost: update.cost?.amount ?? state.usage.cost,
        },
      };
      return state;
    case 'current_mode_update':
      if (update.mode) state = { ...state, mode: update.mode as SessionMode };
      return state;
    case 'current_model_update':
      state = {
        ...state,
        provider: update.provider ?? state.provider,
        model: update.model ?? state.model,
      };
      return state;
    case 'available_commands_update':
      if (update.commands) state = { ...state, commands: update.commands };
      return state;
    case 'run_state_update':
      if (update.runState) {
        const running = /running|started|active/i.test(update.runState);
        if (running) {
          state = { ...state, running };
        } else {
          const status: ToolState['status'] = /cancel/i.test(update.runState)
            ? 'cancelled'
            : /error|fail/i.test(update.runState)
              ? 'failed'
              : 'completed';
          state = { ...settlePendingTools(state, status), running };
        }
      }
      return state;
    case 'error':
      state = {
        ...state,
        error: update.error,
        items: [
          ...state.items,
          {
            kind: 'error',
            id: nextId(state, 'error'),
            text: update.error || 'Unknown error',
            createdAt: now,
          },
        ],
      };
      return state;
    default:
      return state;
  }
}

function applySubagentUpdate(state: ChatState, update: SessionUpdate, agentId: string): ChatState {
  const run = state.subagents[agentId];
  if (!run) return state;
  const subState: ChatState = {
    ...state,
    items: run.items,
    plan: [],
    permissions: [],
    asks: [],
    subagents: {},
    counter: run.counter,
  };
  const next = applyUpdate(subState, update);
  const status: SubagentRun['status'] = update.sessionUpdate === 'error' ? 'error' : run.status;
  return {
    ...state,
    subagents: {
      ...state.subagents,
      [agentId]: {
        ...run,
        status,
        items: next.items,
        counter: next.counter,
        error: update.error || run.error,
      },
    },
  };
}

export function applyUpdates(state: ChatState, updates: SessionUpdate[]): ChatState {
  let next = state;
  for (const update of updates) next = applyUpdate(next, update);
  return next;
}

export function appendUser(state: ChatState, text: string, displayText?: string): ChatState {
  const counter = state.counter + 1;
  return {
    ...state,
    counter,
    items: [
      ...state.items,
      { kind: 'user', id: `user-${counter}`, text: displayText ?? text, createdAt: Date.now() },
    ],
  };
}

export function addNotice(
  state: ChatState,
  text: string,
  level: 'info' | 'warn' | 'error' = 'info',
): ChatState {
  const counter = state.counter + 1;
  return {
    ...state,
    counter,
    items: [...state.items, { kind: 'notice', id: `notice-${counter}`, text, level, createdAt: Date.now() }],
  };
}

export function addPermission(state: ChatState, requestId: string, params: PermissionParams): ChatState {
  if (state.permissions.some((p) => p.requestId === requestId)) return state;
  return { ...state, permissions: [...state.permissions, { requestId, params }] };
}

export function resolvePermission(state: ChatState, requestId: string): ChatState {
  return { ...state, permissions: state.permissions.filter((p) => p.requestId !== requestId) };
}

export function addAsk(state: ChatState, requestId: string, params: AskParams): ChatState {
  if (state.asks.some((a) => a.requestId === requestId)) return state;
  return { ...state, asks: [...state.asks, { requestId, params, answers: {} }] };
}

export function setAskAnswer(state: ChatState, requestId: string, answers: Record<string, string>): ChatState {
  return {
    ...state,
    asks: state.asks.map((a) => (a.requestId === requestId ? { ...a, answers } : a)),
  };
}

export function resolveAsk(state: ChatState, requestId: string): ChatState {
  return { ...state, asks: state.asks.filter((a) => a.requestId !== requestId) };
}
