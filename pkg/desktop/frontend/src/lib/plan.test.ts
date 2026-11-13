import { describe, expect, it } from 'vitest';
import type { ChatItem } from '../state/chat';
import { filterTodoTools, isTodoTool, planEntriesFromInput } from './plan';

function user(id: string): ChatItem {
  return { kind: 'user', id, text: 'hi', createdAt: 1 };
}

function tool(id: string, name: string, input: unknown, createdAt = 2): ChatItem {
  return { kind: 'tool', id, createdAt, tool: { id, name, title: name, status: 'completed', input } };
}

function todo(id: string, todos: unknown, createdAt = 2): ChatItem {
  return tool(id, 'todowrite', { todos }, createdAt);
}

function ids(items: ChatItem[]): string[] {
  return items.map((item) => item.id);
}

describe('isTodoTool', () => {
  it('matches todowrite by name or title', () => {
    expect(isTodoTool({ id: 't', title: 'x', status: 'completed', name: 'todowrite' })).toBe(true);
    expect(isTodoTool({ id: 't', title: 'todowrite: plan', status: 'completed' })).toBe(true);
    expect(isTodoTool({ id: 't', title: 'bash', status: 'completed', name: 'bash' })).toBe(false);
  });
});

describe('planEntriesFromInput', () => {
  it('parses object and JSON string inputs', () => {
    const input = {
      todos: [
        { title: 'Inspect', status: 'completed', priority: 'high' },
        { title: 'Implement', status: 'in_progress', priority: 'medium' },
      ],
    };
    expect(planEntriesFromInput(input)).toEqual([
      { content: 'Inspect', status: 'completed', priority: 'high' },
      { content: 'Implement', status: 'in_progress', priority: 'medium' },
    ]);
    expect(planEntriesFromInput(JSON.stringify(input))).toHaveLength(2);
  });

  it('falls back to content and skips empty titles', () => {
    expect(
      planEntriesFromInput({ todos: [{ content: 'From content' }, { title: '   ' }, { status: 'pending' }] }),
    ).toEqual([{ content: 'From content', status: undefined, priority: undefined }]);
  });

  it('returns an empty list for malformed input', () => {
    expect(planEntriesFromInput(undefined)).toEqual([]);
    expect(planEntriesFromInput('not json')).toEqual([]);
    expect(planEntriesFromInput({ todos: 'nope' })).toEqual([]);
  });
});

describe('filterTodoTools', () => {
  it('keeps only the last todo write in a turn', () => {
    const items = [
      user('u1'),
      todo('t1', [{ title: 'One' }]),
      tool('b1', 'bash', { command: 'ls' }),
      todo('t2', [{ title: 'One' }, { title: 'Two' }]),
      tool('b2', 'bash', { command: 'pwd' }),
    ];
    expect(ids(filterTodoTools(items))).toEqual(['u1', 'b1', 't2', 'b2']);
  });

  it('keeps one todo card per user turn', () => {
    const items = [
      user('u1'),
      todo('t1', [{ title: 'First turn' }]),
      user('u2'),
      todo('t2', [{ title: 'Second turn' }]),
    ];
    expect(ids(filterTodoTools(items))).toEqual(['u1', 't1', 'u2', 't2']);
  });

  it('drops a turn whose latest todo list is empty', () => {
    const items = [
      user('u1'),
      todo('t1', [{ title: 'One' }]),
      todo('t2', []),
    ];
    expect(ids(filterTodoTools(items))).toEqual(['u1']);
  });

  it('treats items before the first user message as one turn', () => {
    const items = [todo('t1', [{ title: 'One' }]), todo('t2', [{ title: 'Two' }])];
    expect(ids(filterTodoTools(items))).toEqual(['t2']);
  });

  it('keeps non-todo tools untouched', () => {
    const items = [tool('b1', 'bash', { command: 'ls' }), tool('w1', 'write', { path: 'a' })];
    expect(ids(filterTodoTools(items))).toEqual(['b1', 'w1']);
  });
});
