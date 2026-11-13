import type { ChatItem, ToolState } from './state';
import type { PlanEntry } from './types';

export function isTodoTool(tool: ToolState): boolean {
  const name = (tool.name ?? '').toLowerCase();
  if (name === 'todowrite') return true;
  const title = tool.title ?? '';
  return /todowrite/i.test(name) || /todowrite/i.test(title);
}

export function planEntriesFromInput(input: unknown): PlanEntry[] {
  let value = input;
  if (typeof value === 'string') {
    try {
      value = JSON.parse(value);
    } catch {
      return [];
    }
  }
  if (!value || typeof value !== 'object') return [];
  const todos = (value as Record<string, unknown>).todos;
  if (!Array.isArray(todos)) return [];
  const entries: PlanEntry[] = [];
  for (const item of todos) {
    if (!item || typeof item !== 'object') continue;
    const record = item as Record<string, unknown>;
    const title =
      typeof record.title === 'string'
        ? record.title
        : typeof record.content === 'string'
          ? record.content
          : '';
    if (!title.trim()) continue;
    entries.push({
      content: title,
      status: typeof record.status === 'string' ? record.status : undefined,
      priority: typeof record.priority === 'string' ? record.priority : undefined,
    });
  }
  return entries;
}

export function filterTodoTools(items: ChatItem[]): ChatItem[] {
  const keep = new Set<string>();
  let latest: { id: string; entries: PlanEntry[] } | null = null;
  const flush = () => {
    if (latest && latest.entries.length > 0) keep.add(latest.id);
    latest = null;
  };
  for (const item of items) {
    if (item.kind === 'user') {
      flush();
      continue;
    }
    if (item.kind === 'tool' && isTodoTool(item.tool)) {
      latest = { id: item.id, entries: planEntriesFromInput(item.tool.input) };
    }
  }
  flush();
  return items.filter((item) => !(item.kind === 'tool' && isTodoTool(item.tool) && !keep.has(item.id)));
}
