import { describe, expect, it } from 'vitest';
import type { ChatItem } from '../../src/chat/state';
import { filterTodoTools, isTodoTool, planEntriesFromInput } from '../../src/chat/plan';

function user(id: string): ChatItem {
  return { kind: 'user', id, text: 'hi', createdAt: 1 };
}

function todo(id: string, todos: unknown, createdAt = 2): ChatItem {
  return {
    kind: 'tool',
    id,
    createdAt,
    tool: {
      id,
      name: 'todowrite',
      title: 'todowrite',
      status: 'completed',
      input: { todos },
    },
  };
}

function bash(id: string, createdAt = 2): ChatItem {
  return {
    kind: 'tool',
    id,
    createdAt,
    tool: { id, name: 'bash', title: 'bash', status: 'completed', input: { command: 'ls' } },
  };
}

function ids(items: ChatItem[]): string[] {
  return items.map((item) => item.id);
}

describe('plan helpers', () => {
  it('parses todo entries and detects todowrite tools', () => {
    expect(isTodoTool({ id: 't', name: 'todowrite', title: 'todowrite', status: 'completed' })).toBe(true);
    expect(isTodoTool({ id: 't', name: 'bash', title: 'bash', status: 'completed' })).toBe(false);
    expect(planEntriesFromInput({ todos: [{ title: 'One', status: 'pending', priority: 'high' }] })).toEqual([
      { content: 'One', status: 'pending', priority: 'high' },
    ]);
    expect(planEntriesFromInput(JSON.stringify({ todos: [{ title: 'Two' }] }))).toEqual([
      { content: 'Two', status: undefined, priority: undefined },
    ]);
  });

  it('keeps only the last todo card per user turn', () => {
    const items = [
      user('u1'),
      todo('t1', [{ title: 'One' }]),
      bash('b1'),
      todo('t2', [{ title: 'One' }, { title: 'Two' }]),
      user('u2'),
      todo('t3', [{ title: 'Next turn' }]),
    ];
    expect(ids(filterTodoTools(items))).toEqual(['u1', 'b1', 't2', 'u2', 't3']);
  });

  it('drops a turn whose latest todo list is empty', () => {
    const items = [user('u1'), todo('t1', [{ title: 'One' }]), todo('t2', [])];
    expect(ids(filterTodoTools(items))).toEqual(['u1']);
  });
});
