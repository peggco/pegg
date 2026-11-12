import { describe, expect, it } from 'vitest';
import {
  addAsk,
  addPermission,
  appendUser,
  applyUpdate,
  emptyChat,
  finalizeRestoredTasks,
  MAX_CHAT_ITEMS,
  pruneChat,
  resolveAsk,
  resolvePermission,
  setAskAnswer,
} from './chat';
import type { SessionUpdate } from '../types';

const chunk = (text: string, messageId?: string): SessionUpdate => ({
  sessionUpdate: 'agent_message_chunk',
  messageId,
  content: { type: 'text', text },
});

describe('chat reducer', () => {
  it('streams assistant text into one item', () => {
    let state = emptyChat();
    state = applyUpdate(state, chunk('Hello'));
    state = applyUpdate(state, chunk(' world'));
    expect(state.items).toHaveLength(1);
    expect(state.items[0]).toMatchObject({ kind: 'assistant', text: 'Hello world' });
  });

  it('merges tool updates and captures diffs', () => {
    let state = emptyChat();
    state = applyUpdate(state, {
      sessionUpdate: 'tool_call',
      toolCallId: 't1',
      name: 'edit',
      title: 'Edit main.go',
      kind: 'edit',
      status: 'pending',
      rawInput: { path: 'main.go' },
    });
    state = applyUpdate(state, {
      sessionUpdate: 'tool_call_update',
      toolCallId: 't1',
      status: 'completed',
      content: [{ type: 'diff', diff: { path: 'main.go', oldText: 'a', newText: 'b' } }],
    });
    const item = state.items[0];
    expect(item.kind).toBe('tool');
    if (item.kind === 'tool') {
      expect(item.tool.status).toBe('completed');
      expect(item.tool.diff?.path).toBe('main.go');
    }
  });

  it('tracks plan and usage', () => {
    let state = emptyChat();
    state = applyUpdate(state, {
      sessionUpdate: 'plan',
      entries: [{ content: 'Inspect', status: 'completed' }],
    });
    state = applyUpdate(state, {
      sessionUpdate: 'usage_update',
      used: 1200,
      size: 200000,
      cost: { amount: 0.02, currency: 'USD' },
    });
    expect(state.plan[0].content).toBe('Inspect');
    expect(state.usage.used).toBe(1200);
    expect(state.usage.cost).toBe(0.02);
  });

  it('queues and resolves permissions and questions', () => {
    let state = emptyChat();
    state = addPermission(state, 'p1', { sessionId: 's', options: [], _meta: { 'com.pegg/question': 'Allow?' } });
    expect(state.permissions).toHaveLength(1);
    state = resolvePermission(state, 'p1');
    expect(state.permissions).toHaveLength(0);

    state = addAsk(state, 'a1', { sessionId: 's', questions: [{ id: 'q', question: 'Name?', type: 'text' }] });
    state = setAskAnswer(state, 'a1', { q: 'Pegg' });
    expect(state.asks[0].answers.q).toBe('Pegg');
    state = resolveAsk(state, 'a1');
    expect(state.asks).toHaveLength(0);
  });
});

describe('subagent routing', () => {
  it('keeps subagent messages out of the main transcript', () => {
    let state = emptyChat();
    state = applyUpdate(state, {
      sessionUpdate: 'subagent',
      _meta: { 'com.pegg/agentId': 'sub-1', 'com.pegg/agentName': 'Explorer' },
      title: 'Map the API',
      status: 'running',
    });
    state = applyUpdate(state, {
      sessionUpdate: 'agent_message_chunk',
      _meta: { 'com.pegg/agentId': 'sub-1', 'com.pegg/agentName': 'Explorer' },
      content: { type: 'text', text: 'Looking around' },
    });
    expect(state.items).toHaveLength(1);
    expect(state.items[0].kind).toBe('subagent');
    expect(state.subagents['sub-1'].items).toHaveLength(1);
    expect(state.subagents['sub-1'].items[0]).toMatchObject({ kind: 'assistant', text: 'Looking around' });
  });
});

describe('tool completion reconciliation', () => {
  it('settles pending tools when the assistant continues', () => {
    let state = emptyChat();
    state = applyUpdate(state, {
      sessionUpdate: 'tool_call',
      toolCallId: 't1',
      name: 'bash',
      status: 'pending',
    });
    state = applyUpdate(state, chunk('moved on'));

    const tool = state.items.find((item) => item.kind === 'tool');
    expect(tool?.kind).toBe('tool');
    if (tool?.kind === 'tool') expect(tool.tool.status).toBe('completed');
  });

  it('settles pending tools when the run ends', () => {
    let state = emptyChat();
    state = applyUpdate(state, {
      sessionUpdate: 'tool_call',
      toolCallId: 't1',
      name: 'write',
      status: 'pending',
    });
    state = applyUpdate(state, { sessionUpdate: 'run_state_update', _meta: { 'com.pegg/runState': 'finished' } });

    const tool = state.items.find((item) => item.kind === 'tool');
    expect(tool?.kind).toBe('tool');
    if (tool?.kind === 'tool') expect(tool.tool.status).toBe('completed');
    expect(state.running).toBe(false);
  });

  it('settles subagent tools when the subagent finishes', () => {
    let state = emptyChat();
    state = applyUpdate(state, {
      sessionUpdate: 'subagent',
      _meta: { 'com.pegg/agentId': 'sub-1', 'com.pegg/agentName': 'Developer' },
      title: 'Edit files',
      status: 'running',
    });
    state = applyUpdate(state, {
      sessionUpdate: 'tool_call',
      _meta: { 'com.pegg/agentId': 'sub-1' },
      toolCallId: 't2',
      name: 'edit',
      status: 'pending',
    });
    state = applyUpdate(state, {
      sessionUpdate: 'subagent',
      _meta: { 'com.pegg/agentId': 'sub-1', 'com.pegg/agentName': 'Developer' },
      status: 'finished',
    });

    const sub = state.subagents['sub-1'];
    expect(sub.items).toHaveLength(1);
    const tool = sub.items[0];
    expect(tool.kind).toBe('tool');
    if (tool.kind === 'tool') expect(tool.tool.status).toBe('completed');
  });
});

describe('tool status precedence', () => {
  it('does not regress a completed tool when a late pending event arrives', () => {
    let state = emptyChat();
    state = applyUpdate(state, {
      sessionUpdate: 'tool_call',
      toolCallId: 't1',
      name: 'bash',
      status: 'pending',
    });
    state = applyUpdate(state, {
      sessionUpdate: 'tool_call_update',
      toolCallId: 't1',
      status: 'completed',
    });
    state = applyUpdate(state, {
      sessionUpdate: 'tool_call',
      toolCallId: 't1',
      status: 'pending',
    });

    const tool = state.items[0];
    expect(tool.kind).toBe('tool');
    if (tool.kind === 'tool') expect(tool.tool.status).toBe('completed');
  });
});

describe('user attachments', () => {
  it('keeps attachments on locally appended user messages', () => {
    const state = appendUser(emptyChat(), 'look', undefined, [
      { name: 'shot.png', mediaType: 'image/png', type: 'image', path: '/tmp/pegg/shot.png' },
    ]);
    expect(state.items[0]).toMatchObject({
      kind: 'user',
      text: 'look',
      attachments: [{ name: 'shot.png', type: 'image' }],
    });
  });

  it('renders attachment-only history messages and merges later text', () => {
    let state = emptyChat();
    state = applyUpdate(state, {
      sessionUpdate: 'user_message_chunk',
      messageId: 'm1',
      content: { type: 'text', text: '' },
      _meta: { 'com.pegg/attachments': [{ name: 'report.pdf', mediaType: 'application/pdf', type: 'file' }] },
    });
    expect(state.items).toHaveLength(1);
    expect(state.items[0]).toMatchObject({
      kind: 'user',
      text: '',
      attachments: [{ name: 'report.pdf' }],
    });

    state = applyUpdate(state, {
      sessionUpdate: 'user_message_chunk',
      messageId: 'm1',
      content: { type: 'text', text: 'see?' },
    });
    expect(state.items).toHaveLength(1);
    expect(state.items[0]).toMatchObject({
      kind: 'user',
      text: 'see?',
      attachments: [{ name: 'report.pdf' }],
    });
  });
});

describe('history window', () => {
  it('drops the oldest history items and keeps a reload cursor', () => {
    let state = emptyChat();
    for (let seq = 1; seq <= MAX_CHAT_ITEMS + 40; seq += 1) {
      state = applyUpdate(state, {
        sessionUpdate: 'tool_call',
        toolCallId: `t-${seq}`,
        name: 'read',
        status: 'completed',
        _meta: { 'com.pegg/seq': seq },
      });
      state = { ...state, historyOldestSeq: seq };
    }
    const pruned = pruneChat(state);
    expect(pruned.items).toHaveLength(MAX_CHAT_ITEMS);
    expect(pruned.items[0].seq).toBe(41);
    expect(pruned.historyOldestSeq).toBe(41);
    expect(pruned.historyHasMore).toBe(true);
  });
});


describe('desktop task cards', () => {
  const taskCall = (background = false): SessionUpdate => ({
    sessionUpdate: 'tool_call',
    toolCallId: 'task-1',
    name: 'task',
    title: 'task: explore-api',
    kind: 'think',
    status: 'pending',
    rawInput: { name: 'explore-api', subagent_type: 'explorer', prompt: 'Map it', background },
  });

  it('creates a task from the call arguments', () => {
    const state = applyUpdate(emptyChat(), taskCall(true));
    const item = state.items[0];
    expect(item.kind).toBe('tool');
    if (item.kind !== 'tool') return;
    expect(item.tool.task).toMatchObject({
      name: 'explore-api',
      type: 'explorer',
      prompt: 'Map it',
      background: true,
      status: 'running',
    });
  });

  it('links the task to the subagent and hides the duplicate card', () => {
    let state = applyUpdate(emptyChat(), taskCall(false));
    state = applyUpdate(state, {
      sessionUpdate: 'subagent',
      _meta: {
        'com.pegg/agentId': 'sub-1',
        'com.pegg/agentName': 'explorer',
        'com.pegg/taskName': 'explore-api',
      },
      status: 'finished',
      rawOutput: 'final answer',
    });
    const tool = state.items.find((item) => item.kind === 'tool');
    const sub = state.items.find((item) => item.kind === 'subagent');
    expect(tool?.kind === 'tool' ? tool.tool.task?.agentId : undefined).toBe('sub-1');
    expect(tool?.kind === 'tool' ? tool.tool.task?.status : undefined).toBe('finished');
    expect(tool?.kind === 'tool' ? tool.tool.task?.output : undefined).toBe('final answer');
    expect(sub?.kind === 'subagent' ? sub.linkedTaskId : undefined).toBe('task-1');
    expect(state.subagents['sub-1']?.linkedTaskId).toBe('task-1');
  });

  it('finishes a linked task when the subagent completes without a task name', () => {
    let state = applyUpdate(emptyChat(), taskCall(true));
    state = applyUpdate(state, {
      sessionUpdate: 'subagent',
      _meta: {
        'com.pegg/agentId': 'sub-1',
        'com.pegg/agentName': 'explorer',
        'com.pegg/taskName': 'explore-api',
      },
      status: 'running',
    });
    let item = state.items.find((entry) => entry.kind === 'tool');
    expect(item?.kind === 'tool' ? item.tool.task?.agentId : undefined).toBe('sub-1');
    expect(item?.kind === 'tool' ? item.tool.task?.status : undefined).toBe('running');

    state = applyUpdate(state, {
      sessionUpdate: 'subagent',
      _meta: {
        'com.pegg/agentId': 'sub-1',
        'com.pegg/agentName': 'explorer',
      },
      status: 'finished',
      rawOutput: 'final answer',
    });
    item = state.items.find((entry) => entry.kind === 'tool');
    expect(item?.kind === 'tool' ? item.tool.task?.status : undefined).toBe('finished');
    expect(item?.kind === 'tool' ? item.tool.task?.output : undefined).toBe('final answer');
  });

  it('marks a linked task as failed when its subagent errors', () => {
    let state = applyUpdate(emptyChat(), taskCall(true));
    state = applyUpdate(state, {
      sessionUpdate: 'subagent',
      _meta: {
        'com.pegg/agentId': 'sub-1',
        'com.pegg/agentName': 'explorer',
        'com.pegg/taskName': 'explore-api',
      },
      status: 'running',
    });
    state = applyUpdate(state, {
      sessionUpdate: 'subagent',
      _meta: {
        'com.pegg/agentId': 'sub-1',
        'com.pegg/agentName': 'explorer',
        'com.pegg/error': 'boom',
      },
      status: 'error',
    });
    const item = state.items.find((entry) => entry.kind === 'tool');
    expect(item?.kind === 'tool' ? item.tool.task?.status : undefined).toBe('error');
    expect(item?.kind === 'tool' ? item.tool.task?.error : undefined).toBe('boom');
  });

  it('keeps a background task running after the start result', () => {
    let state = applyUpdate(emptyChat(), taskCall(true));
    state = applyUpdate(state, {
      sessionUpdate: 'tool_call_update',
      toolCallId: 'task-1',
      status: 'completed',
      rawOutput: 'Started subagent in background: explore-api\n',
    });
    const item = state.items[0];
    if (item.kind !== 'tool') throw new Error('task missing');
    expect(item.tool.task?.status).toBe('running');
    expect(item.tool.task?.output).toBeUndefined();
  });

  it('restores finished, background, and failed tasks', () => {
    const finished = finalizeRestoredTasks([
      {
        kind: 'tool',
        id: 'task-1',
        createdAt: 0,
        tool: {
          id: 'task-1',
          name: 'task',
          title: 'task: x',
          status: 'completed',
          input: { name: 'x', subagent_type: 'explorer', prompt: 'p' },
          task: { name: 'x', prompt: 'p', background: false, status: 'running' },
          output: 'Subagent "x" finished.\nResponse:\nanswer\n',
        },
      },
    ]);
    const tool = finished[0];
    if (tool.kind !== 'tool') throw new Error('task missing');
    expect(tool.tool.task?.status).toBe('finished');
    expect(tool.tool.task?.output).toBe('answer');
    expect(tool.tool.task?.agentId).toBeUndefined();
  });
});
