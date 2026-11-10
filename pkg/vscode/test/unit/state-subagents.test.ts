import { describe, expect, it } from 'vitest';
import {
  appendUser,
  applyHistory,
  applyUpdate,
  filterConversation,
  initialState,
  userMessageOrdinal,
} from '../../src/chat/state';

describe('subagent and tool state', () => {
  it('tracks the subagent lifecycle in a single item', () => {
    let state = initialState();
    state = applyUpdate(state, {
      sessionUpdate: 'subagent',
      _meta: { 'com.pegg/agentId': 'a1', 'com.pegg/agentName': 'explorer' },
      status: 'running',
    });
    expect(state.items).toHaveLength(1);

    state = applyUpdate(state, {
      sessionUpdate: 'subagent',
      _meta: { 'com.pegg/agentId': 'a1', 'com.pegg/agentName': 'explorer' },
      status: 'running',
      title: 'find the auth code',
    });
    state = applyUpdate(state, {
      sessionUpdate: 'subagent',
      _meta: { 'com.pegg/agentId': 'a1', 'com.pegg/agentName': 'explorer' },
      status: 'finished',
      rawOutput: 'found it',
    });

    expect(state.items).toHaveLength(2);
    const item = state.items.find((entry) => entry.kind === 'subagent');
    expect(item).toBeDefined();
    if (!item) return;
    expect(item.kind).toBe('subagent');
    if (item.kind === 'subagent') {
      expect(item.agentId).toBe('a1');
      expect(item.agentName).toBe('explorer');
      expect(item.status).toBe('finished');
      expect(item.task).toBe('find the auth code');
      expect(item.output).toBe('found it');
    }
  });

  it('keeps separate items for separate subagents', () => {
    let state = initialState();
    state = applyUpdate(state, { sessionUpdate: 'subagent', _meta: { 'com.pegg/agentId': 'a1', 'com.pegg/agentName': 'explorer' }, status: 'running' });
    state = applyUpdate(state, { sessionUpdate: 'subagent', _meta: { 'com.pegg/agentId': 'a2', 'com.pegg/agentName': 'planner' }, status: 'running' });
    expect(state.items).toHaveLength(2);
  });

  it('records the tool name and the owning agent', () => {
    let state = initialState();
    state = applyUpdate(state, {
      sessionUpdate: 'tool_call',
      toolCallId: 't1',
      name: 'read',
      title: 'read: a.ts',
      kind: 'read',
      status: 'pending',
      _meta: { 'com.pegg/agentId': 'a1', 'com.pegg/agentName': 'explorer' },
    });

    const item = state.items[0];
    expect(item.kind).toBe('tool');
    if (item.kind === 'tool') {
      expect(item.tool.name).toBe('read');
      expect(item.tool.agentId).toBe('a1');
      expect(item.tool.agentName).toBe('explorer');
    }
  });

  it('records the agent id on assistant messages', () => {
    const state = applyUpdate(initialState(), {
      sessionUpdate: 'agent_message_chunk',
      messageId: 'm1',
      _meta: { 'com.pegg/agentId': 'a1', 'com.pegg/agentName': 'explorer' },
      content: { type: 'text', text: 'hello' },
    });
    const item = state.items[0];
    expect(item.kind).toBe('assistant');
    if (item.kind === 'assistant') {
      expect(item.agentId).toBe('a1');
    }
  });
});

describe('filterConversation', () => {
  const make = () => {
    let state = initialState();
    state = applyUpdate(state, { sessionUpdate: 'subagent', _meta: { 'com.pegg/agentId': 'a1', 'com.pegg/agentName': 'explorer' }, status: 'running' });
    state = applyUpdate(state, {
      sessionUpdate: 'agent_message_chunk',
      messageId: 'main-1',
      _meta: { 'com.pegg/agentId': 'main' },
      content: { type: 'text', text: 'main answer' },
    });
    state = applyUpdate(state, {
      sessionUpdate: 'agent_message_chunk',
      messageId: 'sub-1',
      _meta: { 'com.pegg/agentId': 'a1', 'com.pegg/agentName': 'explorer' },
      content: { type: 'text', text: 'sub answer' },
    });
    state = applyUpdate(state, {
      sessionUpdate: 'tool_call',
      toolCallId: 't1',
      name: 'read',
      title: 'read: a.ts',
      kind: 'read',
      status: 'pending',
      _meta: { 'com.pegg/agentId': 'a1', 'com.pegg/agentName': 'explorer' },
    });
    return state;
  };

  it('hides subagent internals from the main view but keeps the card', () => {
    const state = make();
    const subagentIds = new Set(['a1']);
    const items = filterConversation(state.items, null, subagentIds);
    expect(items.map((item) => item.kind)).toEqual(['subagent', 'assistant']);
    const text = items[1].kind === 'assistant' ? items[1].text : '';
    expect(text).toBe('main answer');
  });

  it('shows only the agent messages in the subagent view', () => {
    const state = make();
    const items = filterConversation(state.items, 'a1', new Set(['a1']));
    expect(items).toHaveLength(2);
    expect(items[0].kind).toBe('assistant');
    if (items[0].kind === 'assistant') expect(items[0].text).toBe('sub answer');
    expect(items[1].kind).toBe('tool');
    if (items[1].kind === 'tool') expect(items[1].tool.name).toBe('read');
  });
});

describe('subagent leak safety net', () => {
  it('hides tools that name a subagent even without an agent id', () => {
    let state = initialState();
    state = applyUpdate(state, {
      sessionUpdate: 'tool_call',
      toolCallId: 't1',
      name: 'read',
      title: 'read: a.ts',
      kind: 'read',
      status: 'pending',
      _meta: { 'com.pegg/agentName': 'explorer' },
    });
    state = applyUpdate(state, {
      sessionUpdate: 'tool_call',
      toolCallId: 't2',
      name: 'bash',
      title: 'bash: ls',
      kind: 'execute',
      status: 'pending',
      _meta: { 'com.pegg/agentName': 'orchestrator' },
    });
    const main = filterConversation(state.items, null, new Set());
    expect(main).toHaveLength(1);
    if (main[0].kind === 'tool') expect(main[0].tool.title).toBe('bash: ls');
  });
});

describe('subagent task prompt', () => {
  it('adds the prompt as a user message owned by the subagent', () => {
    let state = initialState();
    state = applyUpdate(state, {
      sessionUpdate: 'subagent',
      _meta: { 'com.pegg/agentId': 'a1', 'com.pegg/agentName': 'explorer' },
      status: 'running',
      title: 'Find the auth code',
    });
    state = applyUpdate(state, {
      sessionUpdate: 'subagent',
      _meta: { 'com.pegg/agentId': 'a1', 'com.pegg/agentName': 'explorer' },
      status: 'running',
      title: 'Find the auth code',
    });

    const userItems = state.items.filter((item) => item.kind === 'user');
    expect(userItems).toHaveLength(1);
    if (userItems[0].kind === 'user') {
      expect(userItems[0].text).toBe('Find the auth code');
      expect(userItems[0].agentId).toBe('a1');
    }
  });

  it('shows the prompt in the subagent chat but not in the main chat', () => {
    let state = initialState();
    state = applyUpdate(state, { sessionUpdate: 'subagent', _meta: { 'com.pegg/agentId': 'a1', 'com.pegg/agentName': 'explorer' }, status: 'running' });
    state = applyUpdate(state, {
      sessionUpdate: 'subagent',
      _meta: { 'com.pegg/agentId': 'a1', 'com.pegg/agentName': 'explorer' },
      status: 'running',
      title: 'Find the auth code',
    });
    state = applyUpdate(state, {
      sessionUpdate: 'agent_message_chunk',
      messageId: 'sub-1',
      _meta: { 'com.pegg/agentId': 'a1', 'com.pegg/agentName': 'explorer' },
      content: { type: 'text', text: 'found it' },
    });

    const sub = filterConversation(state.items, 'a1', new Set(['a1']));
    expect(sub.map((item) => item.kind)).toEqual(['user', 'assistant']);

    const main = filterConversation(state.items, null, new Set(['a1']));
    expect(main.map((item) => item.kind)).toEqual(['subagent']);
  });

  it('does not count subagent prompts as fork/revert targets', () => {
    let state = initialState();
    state = appendUser(state, 'hello');
    state = applyUpdate(state, {
      sessionUpdate: 'subagent',
      _meta: { 'com.pegg/agentId': 'a1', 'com.pegg/agentName': 'explorer' },
      status: 'running',
      title: 'Find the auth code',
    });
    const realUser = state.items.find((item) => item.kind === 'user' && !item.agentId);
    expect(realUser).toBeDefined();
    if (realUser) {
      expect(userMessageOrdinal(state, realUser.id)).toBe(0);
    }
    const prompt = state.items.find((item) => item.kind === 'user' && item.agentId === 'a1');
    expect(prompt).toBeDefined();
    if (prompt) {
      expect(userMessageOrdinal(state, prompt.id)).toBeUndefined();
    }
  });
});


describe('task cards', () => {
  const taskCall = (background = false): Parameters<typeof applyUpdate>[1] => ({
    sessionUpdate: 'tool_call',
    toolCallId: 'task-1',
    name: 'task',
    title: 'task: explore-api',
    kind: 'think',
    status: 'pending',
    rawInput: { name: 'explore-api', subagent_type: 'explorer', prompt: 'Map it', background },
  });

  it('creates a task card from the task call arguments', () => {
    const state = applyUpdate(initialState(), taskCall());
    const item = state.items[0];
    expect(item.kind).toBe('tool');
    if (item.kind !== 'tool') return;
    expect(item.tool.task).toMatchObject({
      name: 'explore-api',
      type: 'explorer',
      prompt: 'Map it',
      status: 'running',
    });
  });

  it('links the task to the subagent and hides the duplicate subagent card', () => {
    let state = applyUpdate(initialState(), taskCall(true));
    state = applyUpdate(state, {
      sessionUpdate: 'subagent',
      _meta: {
        'com.pegg/agentId': 'sub-1',
        'com.pegg/agentName': 'explorer',
        'com.pegg/taskName': 'explore-api',
      },
      status: 'running',
    });
    const tool = state.items.find((item) => item.kind === 'tool');
    const sub = state.items.find((item) => item.kind === 'subagent');
    expect(tool?.kind === 'tool' ? tool.tool.task?.agentId : undefined).toBe('sub-1');
    expect(sub?.kind === 'subagent' ? sub.linkedTaskId : undefined).toBe('task-1');

    const visible = filterConversation(state.items, null, new Set(['sub-1']));
    expect(visible.some((item) => item.kind === 'subagent')).toBe(false);
    expect(visible.some((item) => item.kind === 'tool')).toBe(true);
  });

  it('keeps a background task running until the subagent finishes', () => {
    let state = applyUpdate(initialState(), taskCall(true));
    state = applyUpdate(state, {
      sessionUpdate: 'tool_call_update',
      toolCallId: 'task-1',
      status: 'completed',
      rawOutput: 'Started subagent in background: explore-api\n',
    });
    let tool = state.items.find((item) => item.kind === 'tool');
    expect(tool?.kind === 'tool' ? tool.tool.task?.status : undefined).toBe('running');
    expect(tool?.kind === 'tool' ? tool.tool.task?.output : 'x').toBeUndefined();

    state = applyUpdate(state, {
      sessionUpdate: 'subagent',
      _meta: {
        'com.pegg/agentId': 'sub-1',
        'com.pegg/agentName': 'explorer',
        'com.pegg/taskName': 'explore-api',
      },
      status: 'running',
    });
    tool = state.items.find((item) => item.kind === 'tool');
    expect(tool?.kind === 'tool' ? tool.tool.task?.agentId : undefined).toBe('sub-1');
    expect(tool?.kind === 'tool' ? tool.tool.task?.status : undefined).toBe('running');

    state = applyUpdate(state, {
      sessionUpdate: 'subagent',
      _meta: {
        'com.pegg/agentId': 'sub-1',
        'com.pegg/agentName': 'explorer',
        'com.pegg/usage': { used: 42, cost: 0.01 },
      },
      status: 'finished',
      rawOutput: 'final answer',
    });
    tool = state.items.find((item) => item.kind === 'tool');
    expect(tool?.kind === 'tool' ? tool.tool.task?.status : undefined).toBe('finished');
    expect(tool?.kind === 'tool' ? tool.tool.task?.output : undefined).toBe('final answer');
    expect(tool?.kind === 'tool' ? tool.tool.task?.usage?.used : undefined).toBe(42);
  });

  it('marks a linked task as failed when its subagent errors', () => {
    let state = applyUpdate(initialState(), taskCall(true));
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
    const tool = state.items.find((item) => item.kind === 'tool');
    expect(tool?.kind === 'tool' ? tool.tool.task?.status : undefined).toBe('error');
    expect(tool?.kind === 'tool' ? tool.tool.task?.error : undefined).toBe('boom');
  });

  it('restores a finished task card from history', () => {
    const state = applyHistory(initialState(), [
      taskCall(),
      {
        sessionUpdate: 'tool_call_update',
        toolCallId: 'task-1',
        status: 'completed',
        rawOutput: 'Subagent "explore-api" finished.\nResponse:\nanswer\n',
      },
    ]);
    const item = state.items[0];
    expect(item.kind).toBe('tool');
    if (item.kind !== 'tool') return;
    expect(item.tool.task?.status).toBe('finished');
    expect(item.tool.task?.output).toBe('answer');
    expect(item.tool.task?.agentId).toBeUndefined();
  });

  it('restores a background task card without the start message as output', () => {
    const state = applyHistory(initialState(), [
      taskCall(true),
      {
        sessionUpdate: 'tool_call_update',
        toolCallId: 'task-1',
        status: 'completed',
        rawOutput: 'Started subagent in background: explore-api\n',
      },
    ]);
    const item = state.items[0];
    if (item.kind !== 'tool') throw new Error('task card missing');
    expect(item.tool.task?.status).toBe('background');
    expect(item.tool.task?.output).toBeUndefined();
  });
});
