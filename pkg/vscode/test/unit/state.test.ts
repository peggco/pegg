import { describe, expect, it } from 'vitest';
import {
  addAttachment,
  addNotice,
  appendUser,
  applyUpdate,
  clearConversation,
  initialState,
  removeAttachment,
} from '../../src/chat/state';

describe('chat state', () => {
  it('appends streamed assistant chunks to one message', () => {
    let state = initialState();
    state = applyUpdate(state, {
      sessionUpdate: 'agent_message_chunk',
      messageId: 'm1',
      content: { type: 'text', text: 'Hello' },
    });
    state = applyUpdate(state, {
      sessionUpdate: 'agent_message_chunk',
      messageId: 'm1',
      content: { type: 'text', text: ' world' },
    });
    expect(state.items).toHaveLength(1);
    expect(state.items[0]).toMatchObject({ kind: 'assistant', text: 'Hello world' });
  });

  it('upserts tool calls and captures diffs', () => {
    let state = initialState();
    state = applyUpdate(state, {
      sessionUpdate: 'tool_call',
      toolCallId: 't1',
      title: 'edit: main.go',
      kind: 'edit',
      status: 'pending',
      rawInput: { filePath: 'main.go' },
    });
    state = applyUpdate(state, {
      sessionUpdate: 'tool_call_update',
      toolCallId: 't1',
      status: 'completed',
      rawOutput: 'ok',
      contents: [{ type: 'diff', diff: { path: 'main.go', oldText: 'a', newText: 'b' } }],
    });
    expect(state.items).toHaveLength(1);
    const item = state.items[0];
    expect(item.kind).toBe('tool');
    if (item.kind === 'tool') {
      expect(item.tool.status).toBe('completed');
      expect(item.tool.diff?.newText).toBe('b');
    }
  });

  it('keeps text before and after a tool call in separate segments', () => {
    let state = initialState();
    state = applyUpdate(state, {
      sessionUpdate: 'agent_message_chunk',
      messageId: 'before',
      content: { type: 'text', text: 'before' },
    });
    state = applyUpdate(state, {
      sessionUpdate: 'tool_call',
      toolCallId: 't1',
      title: 'bash: ls',
      kind: 'other',
      status: 'pending',
    });
    state = applyUpdate(state, {
      sessionUpdate: 'agent_message_chunk',
      messageId: 'after',
      content: { type: 'text', text: 'after' },
    });
    expect(state.items.map((item) => item.kind)).toEqual(['assistant', 'tool', 'assistant']);
    const tool = state.items[1];
    if (tool.kind === 'tool') expect(tool.tool.kind).toBe('execute');
  });

  it('merges consecutive assistant chunks with different message ids', () => {
    let state = initialState();
    state = applyUpdate(state, {
      sessionUpdate: 'agent_message_chunk',
      messageId: 'a1',
      agentName: 'orchestrator',
      content: { type: 'text', text: 'One ' },
    });
    state = applyUpdate(state, {
      sessionUpdate: 'agent_message_chunk',
      messageId: 'a2',
      agentName: 'orchestrator',
      content: { type: 'text', text: 'response' },
    });
    expect(state.items).toHaveLength(1);
    expect(state.items[0]).toMatchObject({ kind: 'assistant', text: 'One response' });
  });

  it('merges consecutive reasoning segments into one block', () => {
    let state = initialState();
    state = applyUpdate(state, {
      sessionUpdate: 'agent_thought_chunk',
      messageId: 'r1',
      agentName: 'orchestrator',
      content: { type: 'text', text: 'I should' },
    });
    state = applyUpdate(state, {
      sessionUpdate: 'agent_thought_chunk',
      messageId: 'r2',
      agentName: 'orchestrator',
      content: { type: 'text', text: ' inspect the repo' },
    });
    expect(state.items).toHaveLength(1);
    expect(state.items[0]).toMatchObject({ kind: 'reasoning', text: 'I should inspect the repo' });
  });

  it('tracks plan, usage, mode, model and commands', () => {
    let state = initialState();
    state = applyUpdate(state, {
      sessionUpdate: 'plan',
      entries: [{ content: 'one', status: 'pending' }],
    });
    state = applyUpdate(state, { sessionUpdate: 'usage_update', used: 10, size: 100, cost: { amount: 0.1, currency: 'USD' } });
    state = applyUpdate(state, { sessionUpdate: 'current_mode_update', mode: 'plan' });
    state = applyUpdate(state, { sessionUpdate: 'current_model_update', provider: 'openai', model: 'gpt' });
    state = applyUpdate(state, { sessionUpdate: 'available_commands_update', commands: [{ name: 'init' }] });
    expect(state.plan[0].content).toBe('one');
    expect(state.usage.cost).toBe(0.1);
    expect(state.mode).toBe('plan');
    expect(state.provider).toBe('openai');
    expect(state.commands[0].name).toBe('init');
  });

  it('handles reasoning, errors and notices', () => {
    let state = initialState();
    state = applyUpdate(state, {
      sessionUpdate: 'agent_thought_chunk',
      messageId: 'r1',
      content: { type: 'text', text: 'thinking' },
    });
    state = applyUpdate(state, { sessionUpdate: 'error', error: 'boom' });
    state = addNotice(state, 'careful', 'warn');
    expect(state.items.map((item) => item.kind)).toEqual(['reasoning', 'error', 'notice']);
  });

  it('manages user messages, attachments and clearing', () => {
    let state = appendUser(initialState(), 'hi');
    state = addAttachment(state, { id: 'a1', name: 'a.txt', mediaType: 'text/plain', text: 'x' });
    expect(state.composerAttachments).toHaveLength(1);
    state = removeAttachment(state, 'a1');
    expect(state.composerAttachments).toHaveLength(0);
    state = applyUpdate(state, { sessionUpdate: 'usage_update', used: 1, size: 2 });
    state = clearConversation(state);
    expect(state.items).toHaveLength(0);
    expect(state.usage.used).toBe(0);
  });
});
