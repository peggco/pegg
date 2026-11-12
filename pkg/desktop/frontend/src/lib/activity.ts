import type { ChatItem, ToolState } from '../state/chat';
import type { ChatHistoryMode } from '../types';

export type HistoryRow =
  | { kind: 'item'; key: string; item: ChatItem }
  | { kind: 'activity'; key: string; id: string; items: ChatItem[]; createdAt: number };

export interface ActivityCounts {
  commands: number;
  reads: number;
  edits: number;
  searches: number;
  fetches: number;
  thoughts: number;
}

export function isActivityItem(item: ChatItem): boolean {
  if (item.kind === 'reasoning') return true;
  return item.kind === 'tool' && !item.tool.task;
}

export function buildHistoryRows(items: ChatItem[], mode: ChatHistoryMode): HistoryRow[] {
  if (mode !== 'compact') {
    return items.map((item) => ({ kind: 'item', key: item.id, item }));
  }

  const rows: HistoryRow[] = [];
  let group: ChatItem[] = [];

  const flush = (nextId?: string) => {
    if (!group.length) return;
    const id = `activity:${group[0].id}`;
    const anchor = nextId ?? group[group.length - 1].id;
    rows.push({
      kind: 'activity',
      key: `activity:${anchor}`,
      id,
      items: group,
      createdAt: group[group.length - 1].createdAt,
    });
    group = [];
  };

  for (const item of items) {
    if (isActivityItem(item)) {
      group.push(item);
      continue;
    }
    flush(item.id);
    rows.push({ kind: 'item', key: item.id, item });
  }
  flush();
  return rows;
}

export function activityCounts(items: ChatItem[]): ActivityCounts {
  const counts: ActivityCounts = { commands: 0, reads: 0, edits: 0, searches: 0, fetches: 0, thoughts: 0 };
  for (const item of items) {
    if (item.kind === 'reasoning') {
      counts.thoughts += 1;
      continue;
    }
    if (item.kind !== 'tool') continue;
    const category = toolCategory(item.tool);
    if (category) counts[category] += 1;
  }
  return counts;
}

export function formatActivityTime(createdAt: number, locale: string): string {
  try {
    return new Intl.DateTimeFormat(locale, {
      month: 'numeric',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
    }).format(new Date(createdAt));
  } catch {
    return '';
  }
}

function toolCategory(tool: ToolState): keyof ActivityCounts | null {
  const kind = (tool.kind ?? '').toLowerCase();
  const title = (tool.title ?? '').toLowerCase();
  if (kind === 'execute' || kind === 'bash' || /^bash\b/.test(title)) return 'commands';
  if (kind === 'read' || /^read\b/.test(title)) return 'reads';
  if (kind === 'edit' || /^(write|edit|create|update)\b/.test(title)) return 'edits';
  if (kind === 'search' || /^(search|grep|glob|find)\b/.test(title)) return 'searches';
  if (kind === 'fetch' || /^(fetch|web)\b/.test(title)) return 'fetches';
  return null;
}
