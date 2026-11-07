import type {
  AskParams,
  AttachmentInfo,
  ContentBlock,
  PermissionParams,
  PlanEntry,
  SessionMode,
  SessionUpdate,
  SlashCommand,
  ToolCallContent,
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
  files?: AttachmentInfo[];
  agentId?: string;
  agentName?: string;
}

function mergeFiles(
  existing: AttachmentInfo[] | undefined,
  incoming: AttachmentInfo[],
): AttachmentInfo[] {
  if (!existing?.length) return incoming;
  const seen = new Set(existing.map((file) => `${file.name}|${file.path ?? ''}`));
  const merged = [...existing];
  for (const file of incoming) {
    if (seen.has(`${file.name}|${file.path ?? ''}`)) continue;
    merged.push(file);
  }
  return merged;
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
  if (!content) return '';
  if (Array.isArray(content)) {
    return content
      .map((item) => (item.type === 'content' && item.content ? contentText(item.content) : ''))
      .join('');
  }
  const block = content as ContentBlock;
  return block.type === 'text' ? (block.text ?? '') : '';
}

function toolContents(update: SessionUpdate): ToolCallContent[] {
  return Array.isArray(update.content) ? update.content : [];
}

function peggMeta(update: SessionUpdate, key: string): unknown {
  return update._meta?.[`com.pegg/${key}`];
}

function peggString(update: SessionUpdate, key: string): string | undefined {
  const value = peggMeta(update, key);
  return typeof value === 'string' ? value : undefined;
}

function peggNumber(update: SessionUpdate, key: string): number | undefined {
  const value = peggMeta(update, key);
  return typeof value === 'number' ? value : undefined;
}

function peggAttachments(update: SessionUpdate): AttachmentInfo[] | undefined {
  const value = peggMeta(update, 'attachments');
  return Array.isArray(value) ? (value as AttachmentInfo[]) : undefined;
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
  const agentId = peggString(update, 'agentId');
  const agentName = peggString(update, 'agentName');
  if (agentId && update.sessionUpdate !== 'subagent' && input.subagents[agentId]) {
    return applySubagentUpdate(input, update, agentId);
  }
  let state: ChatState = { ...input, items: input.items, plan: input.plan };
  const now = Date.now();
  const seqValue = peggNumber(update, 'seq');
  const seq = seqValue && seqValue > 0 ? seqValue : undefined;

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
      const metaAttachments = peggAttachments(update);
      const attachments = metaAttachments?.length ? metaAttachments : undefined;
      if (!text && !attachments) return state;
      const kind = update.sessionUpdate === 'user_message_chunk' ? 'user' : 'assistant';
      const id = update.messageId || nextId(state, 'message');
      const idx = findIndex(state.items, id, kind);
      const last = state.items[state.items.length - 1];
      if (idx >= 0) {
        const item = state.items[idx] as Extract<ChatItem, { kind: 'user' | 'assistant' }>;
        replaceItem(idx, {
          ...item,
          text: item.text + text,
          ...(attachments ? { attachments: mergeAttachments(item, attachments) } : {}),
        } as ChatItem);
      } else if (
        (last?.kind === 'user' && kind === 'user') ||
        (last?.kind === 'assistant' &&
          kind === 'assistant' &&
          (last.agentName ?? '') === (agentName ?? ''))
      ) {
        replaceItem(state.items.length - 1, {
          ...last,
          text: last.text + text,
          ...(attachments && kind === 'user' ? { attachments: mergeAttachments(last, attachments) } : {}),
        } as ChatItem);
      } else {
        if (kind === 'assistant') {
          state = { ...state, items: settleTools(state.items, 'completed') };
        }
        pushItem({
          kind,
          id,
          text,
          agentId,
          agentName,
          createdAt: now,
          seq,
          ...(kind === 'user' && attachments ? { attachments } : {}),
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
      } else if (last?.kind === 'reasoning' && (last.agentName ?? '') === (agentName ?? '')) {
        replaceItem(state.items.length - 1, { ...last, text: last.text + text });
      } else {
        pushItem({
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
        const error = peggString(update, 'error');
        const attachments = peggAttachments(update);
        if (update.rawInput !== undefined) tool.input = update.rawInput;
        if (update.rawOutput !== undefined) tool.output = update.rawOutput;
        if (error !== undefined) tool.error = error;
        if (update.locations) tool.locations = update.locations;
        if (update.name) tool.name = update.name;
        if (agentId) tool.agentId = agentId;
        if (agentName) tool.agentName = agentName;
        if (attachments?.length) tool.files = mergeFiles(tool.files, attachments);
        const diff = toolContents(update).find((c) => c.type === 'diff' && c.diff)?.diff;
        if (diff) tool.diff = diff;
        replaceItem(idx, { ...item, tool });
      } else {
        const error = peggString(update, 'error');
        const attachments = peggAttachments(update);
        pushItem({
          kind: 'tool',
          id,
          createdAt: now,
          seq,
          tool: {
            id,
            name: update.name,
            title: update.title || update.name || 'Tool call',
            kind: inferToolKind(update.kind, update.title),
            status: status ?? 'pending',
            input: update.rawInput,
            output: update.rawOutput,
            error,
            locations: update.locations,
            diff: toolContents(update).find((c) => c.type === 'diff' && c.diff)?.diff,
            files: attachments?.length ? attachments : undefined,
            agentId,
            agentName,
          },
        });
      }
      return state;
    }
    case 'subagent': {
      const subId = peggString(update, 'agentId') ?? '';
      if (!subId) return state;
      const subName = peggString(update, 'agentName');
      const subError = peggString(update, 'error');
      const status: 'running' | 'finished' | 'error' =
        update.status === 'finished' ? 'finished' : update.status === 'error' ? 'error' : 'running';
      const existingRun = state.subagents[subId];
      const settleStatus: ToolState['status'] | undefined =
        status === 'finished' ? 'completed' : status === 'error' ? 'failed' : undefined;
      const run: SubagentRun = existingRun
        ? {
            ...existingRun,
            status,
            agentName: subName || existingRun.agentName,
            task: update.title || existingRun.task,
            output: update.rawOutput !== undefined ? String(update.rawOutput ?? '') : existingRun.output,
            error: subError || existingRun.error,
            items: settleStatus ? settleTools(existingRun.items, settleStatus) : existingRun.items,
          }
        : {
            agentId: subId,
            agentName: subName,
            status,
            task: update.title,
            output: update.rawOutput === undefined ? undefined : String(update.rawOutput ?? ''),
            error: subError,
            items: [],
            counter: 0,
          };
      state = { ...state, subagents: { ...state.subagents, [subId]: run } };
      const idx = state.items.findIndex((item) => item.kind === 'subagent' && item.agentId === subId);
      if (idx >= 0) {
        const item = state.items[idx] as Extract<ChatItem, { kind: 'subagent' }>;
        replaceItem(idx, {
          ...item,
          status,
          agentName: subName || item.agentName,
          task: update.title || item.task,
          output: update.rawOutput !== undefined ? String(update.rawOutput ?? '') : item.output,
          error: subError || item.error,
        });
      } else {
        pushItem({
          kind: 'subagent',
          id: `subagent-${subId}`,
          agentId: subId,
          agentName: subName,
          status,
          task: update.title,
          output: update.rawOutput === undefined ? undefined : String(update.rawOutput ?? ''),
          error: subError,
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
      if (update.currentModeId) state = { ...state, mode: update.currentModeId as SessionMode };
      return state;
    case 'config_option_update': {
      let mode = state.mode;
      let model = state.model;
      for (const option of update.configOptions ?? []) {
        if (option.id === 'mode' && typeof option.currentValue === 'string') {
          mode = option.currentValue as SessionMode;
        }
        if (option.id === 'model' && typeof option.currentValue === 'string') {
          model = option.currentValue;
        }
      }
      state = { ...state, mode, model };
      return state;
    }
    case 'available_commands_update':
      if (update.availableCommands) state = { ...state, commands: update.availableCommands };
      return state;
    case 'session_info_update':
      if (update.title) state = { ...state, title: update.title };
      return state;
    case 'run_state_update': {
      const runState = peggString(update, 'runState');
      if (runState) {
        const running = /running|started|active/i.test(runState);
        if (running) {
          state = { ...state, running };
        } else {
          const status: ToolState['status'] = /cancel/i.test(runState)
            ? 'cancelled'
            : /error|fail/i.test(runState)
              ? 'failed'
              : 'completed';
          state = { ...settlePendingTools(state, status), running };
        }
      }
      return state;
    }
    case 'run_ask_resolved':
      return { ...state, asks: [] };
    case 'agent_warning_clear':
      return { ...state, items: state.items.filter((item) => !(item.kind === 'notice' && item.level === 'warn')) };
    case 'notice': {
      const level: 'info' | 'warn' | 'error' =
        update.severity === 'error' ? 'error' : update.severity === 'warning' ? 'warn' : 'info';
      const text = [update.title, update.description].filter(Boolean).join(': ') || 'Notice';
      state = {
        ...state,
        ...(level === 'error' ? { error: text } : {}),
        items: [...state.items, { kind: 'notice', id: nextId(state, 'notice'), text, level, createdAt: now }],
      };
      return state;
    }
    case 'compaction_update': {
      const text = update.summary?.map((block) => block.text ?? '').join('') || update.error;
      if (!text) return state;
      const level: 'info' | 'warn' | 'error' = update.error ? 'error' : 'info';
      state = {
        ...state,
        items: [...state.items, { kind: 'notice', id: nextId(state, 'notice'), text, level, createdAt: now }],
      };
      return state;
    }
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
  const error = peggString(update, 'error');
  const status: SubagentRun['status'] =
    update.sessionUpdate === 'notice' && update.severity === 'error' ? 'error' : run.status;
  return {
    ...state,
    subagents: {
      ...state.subagents,
      [agentId]: {
        ...run,
        status,
        items: next.items,
        counter: next.counter,
        error: error || run.error,
      },
    },
  };
}

export function applyUpdates(state: ChatState, updates: SessionUpdate[]): ChatState {
  let next = state;
  for (const update of updates) next = applyUpdate(next, update);
  return next;
}

export function attachmentType(mediaType?: string): 'image' | 'audio' | 'video' | 'file' {
  const mt = (mediaType ?? '').toLowerCase();
  if (mt.startsWith('image/')) return 'image';
  if (mt.startsWith('audio/')) return 'audio';
  if (mt.startsWith('video/')) return 'video';
  return 'file';
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

export function appendUser(
  state: ChatState,
  text: string,
  displayText?: string,
  attachments?: AttachmentInfo[],
): ChatState {
  const counter = state.counter + 1;
  return {
    ...state,
    counter,
    items: [
      ...state.items,
      {
        kind: 'user',
        id: `user-${counter}`,
        text: displayText ?? text,
        createdAt: Date.now(),
        ...(attachments?.length ? { attachments } : {}),
      },
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
