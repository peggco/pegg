import { describe, expect, it } from 'vitest';
import type { ChatItem } from '../state/chat';
import { activeActivityKey, activityCounts, buildHistoryRows, formatActivityTime } from './activity';

function user(id: string, createdAt = 1): ChatItem {
  return { kind: 'user', id, text: 'hi', createdAt };
}

function assistant(id: string, createdAt = 2): ChatItem {
  return { kind: 'assistant', id, text: 'done', createdAt };
}

function reasoning(id: string, createdAt = 3): ChatItem {
  return { kind: 'reasoning', id, text: 'thinking', createdAt };
}

function tool(id: string, kind: string, title: string, createdAt = 4, task = false): ChatItem {
  return {
    kind: 'tool',
    id,
    createdAt,
    tool: {
      id,
      title,
      kind,
      status: 'completed',
      task: task
        ? { name: 'task', prompt: '', background: false, status: 'finished' }
        : undefined,
    },
  };
}

describe('buildHistoryRows', () => {
  it('keeps one row per item in detailed mode', () => {
    const items = [user('u1'), reasoning('r1'), tool('t1', 'execute', 'bash'), assistant('a1')];
    const rows = buildHistoryRows(items, 'detailed');
    expect(rows.map((row) => row.kind)).toEqual(['item', 'item', 'item', 'item']);
    expect(rows.every((row) => row.kind === 'item' && row.key === row.item.id)).toBe(true);
  });

  it('groups consecutive reasoning and tools in compact mode', () => {
    const items = [
      user('u1'),
      reasoning('r1'),
      tool('t1', 'execute', 'bash'),
      tool('t2', 'read', 'read file'),
      assistant('a1'),
    ];
    const rows = buildHistoryRows(items, 'compact');
    expect(rows.map((row) => row.kind)).toEqual(['item', 'activity', 'item']);
    const group = rows[1];
    expect(group.kind).toBe('activity');
    if (group.kind === 'activity') {
      expect(group.items).toHaveLength(3);
      expect(group.id).toBe('activity:r1');
      expect(group.key).toBe('activity:a1');
      expect(group.createdAt).toBe(4);
    }
  });

  it('keeps the activity row key stable when older items merge into the group', () => {
    const base = [reasoning('r2'), tool('t2', 'read', 'read file'), assistant('a1')];
    const before = buildHistoryRows(base, 'compact');
    const after = buildHistoryRows([reasoning('r1'), tool('t1', 'execute', 'bash'), ...base], 'compact');
    expect(before[0].kind === 'activity' && after[0].kind === 'activity').toBe(true);
    if (before[0].kind === 'activity' && after[0].kind === 'activity') {
      expect(after[0].items).toHaveLength(4);
      expect(after[0].key).toBe(before[0].key);
      expect(after[0].id).not.toBe(before[0].id);
    }
  });

  it('keeps the trailing activity row key stable when new items append', () => {
    const before = buildHistoryRows([reasoning('r1')], 'compact');
    const after = buildHistoryRows([reasoning('r1'), tool('t1', 'execute', 'bash')], 'compact');
    expect(before[0].kind === 'activity' && after[0].kind === 'activity').toBe(true);
    if (before[0].kind === 'activity' && after[0].kind === 'activity') {
      expect(after[0].items).toHaveLength(2);
      expect(after[0].key).toBe(before[0].key);
    }
  });

  it('breaks groups around assistant, user, notice and task tools', () => {
    const items = [
      reasoning('r1'),
      tool('t1', 'execute', 'bash'),
      assistant('a1'),
      reasoning('r2'),
      tool('task1', 'edit', 'task', 5, true),
      reasoning('r3'),
    ];
    const rows = buildHistoryRows(items, 'compact');
    expect(rows.map((row) => row.kind)).toEqual(['activity', 'item', 'activity', 'item', 'activity']);
    expect(rows[1].kind === 'item' && rows[1].item.id).toBe('a1');
    expect(rows[3].kind === 'item' && rows[3].item.id).toBe('task1');
  });

  it('is stable and empty-safe', () => {
    expect(buildHistoryRows([], 'compact')).toEqual([]);
    const items = [tool('t1', 'execute', 'bash')];
    expect(buildHistoryRows(items, 'compact')[0].key).toBe('activity:t1');
  });
});

describe('activeActivityKey', () => {
  it('returns null when the agent is not running', () => {
    const rows = buildHistoryRows([reasoning('r1'), tool('t1', 'execute', 'bash')], 'compact');
    expect(activeActivityKey(rows, false)).toBeNull();
  });

  it('returns null when there are no rows', () => {
    expect(activeActivityKey([], true)).toBeNull();
  });

  it('returns the trailing activity key while running', () => {
    const rows = buildHistoryRows([user('u1'), reasoning('r1'), tool('t1', 'execute', 'bash')], 'compact');
    expect(rows[rows.length - 1].kind).toBe('activity');
    expect(activeActivityKey(rows, true)).toBe(rows[rows.length - 1].key);
  });

  it('returns null when the trailing row is an assistant item', () => {
    const rows = buildHistoryRows([reasoning('r1'), tool('t1', 'execute', 'bash'), assistant('a1')], 'compact');
    expect(rows[rows.length - 1].kind).toBe('item');
    expect(activeActivityKey(rows, true)).toBeNull();
  });
});

describe('activityCounts', () => {
  it('counts tools by category and reasoning', () => {
    const items = [
      reasoning('r1'),
      reasoning('r2'),
      tool('t1', 'execute', 'bash'),
      tool('t2', 'read', 'read file'),
      tool('t3', 'edit', 'edit file'),
      tool('t4', 'search', 'grep'),
      tool('t5', 'fetch', 'web fetch'),
      tool('t6', 'unknown', 'other'),
    ];
    expect(activityCounts(items)).toEqual({
      commands: 1,
      reads: 1,
      edits: 1,
      searches: 1,
      fetches: 1,
      thoughts: 2,
    });
  });
});

describe('formatActivityTime', () => {
  it('formats a timestamp for a locale', () => {
    expect(formatActivityTime(Date.UTC(2026, 6, 11, 10, 45), 'en-US')).toMatch(/\d/);
  });
});
