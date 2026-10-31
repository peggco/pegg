import { describe, expect, it } from 'vitest';
import {
  addAsk,
  addPermission,
  applyUpdate,
  emptyChat,
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
      toolName: 'edit',
      title: 'Edit main.go',
      kind: 'edit',
      status: 'pending',
      rawInput: { path: 'main.go' },
    });
    state = applyUpdate(state, {
      sessionUpdate: 'tool_call_update',
      toolCallId: 't1',
      status: 'completed',
      contents: [{ type: 'diff', diff: { path: 'main.go', oldText: 'a', newText: 'b' } }],
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
    state = addPermission(state, 'p1', { sessionId: 's', options: [], question: 'Allow?' });
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
      agentId: 'sub-1',
      agentName: 'Explorer',
      title: 'Map the API',
      status: 'running',
    });
    state = applyUpdate(state, {
      sessionUpdate: 'agent_message_chunk',
      agentId: 'sub-1',
      agentName: 'Explorer',
      content: { type: 'text', text: 'Looking around' },
    });
    expect(state.items).toHaveLength(1);
    expect(state.items[0].kind).toBe('subagent');
    expect(state.subagents['sub-1'].items).toHaveLength(1);
    expect(state.subagents['sub-1'].items[0]).toMatchObject({ kind: 'assistant', text: 'Looking around' });
  });
});
