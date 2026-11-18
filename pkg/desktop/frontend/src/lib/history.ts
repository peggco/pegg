export const MAX_PROMPT_HISTORY = 100;

export interface PromptHistory {
  entries: string[];
  index: number;
  draft: string;
}

export interface HistoryKeyState {
  history: PromptHistory;
  value: string;
  selectionCollapsed: boolean;
  caretAtEnd: boolean;
  popupOpen: boolean;
}

export interface HistoryKeyOutcome {
  handled: boolean;
  history: PromptHistory;
  text: string;
}

export function createPromptHistory(entries: readonly string[] = []): PromptHistory {
  let history: PromptHistory = { entries: [], index: 0, draft: '' };
  for (const entry of entries) history = rememberPrompt(history, entry);
  return history;
}

export function resetPromptHistory(history: PromptHistory): PromptHistory {
  if (history.index === history.entries.length && history.draft === '') return history;
  return { ...history, index: history.entries.length, draft: '' };
}

export function rememberPrompt(history: PromptHistory, text: string): PromptHistory {
  const value = text.trim();
  if (!value) return resetPromptHistory(history);
  const entries = history.entries.slice();
  if (entries[entries.length - 1] !== value) {
    entries.push(value);
    if (entries.length > MAX_PROMPT_HISTORY) entries.splice(0, entries.length - MAX_PROMPT_HISTORY);
  }
  return { entries, index: entries.length, draft: '' };
}

export function historyPrevious(
  history: PromptHistory,
  currentText: string,
): { history: PromptHistory; text: string } | null {
  const { entries } = history;
  if (!entries.length) return null;
  const index = Math.min(Math.max(history.index, 0), entries.length);
  const draft = index === entries.length ? currentText : history.draft;
  if (index === 0) return { history: { entries, index: 0, draft }, text: entries[0] };
  const next = index - 1;
  return { history: { entries, index: next, draft }, text: entries[next] };
}

export function historyNext(
  history: PromptHistory,
): { history: PromptHistory; text: string } | null {
  const { entries } = history;
  if (history.index >= entries.length) return null;
  const next = history.index + 1;
  if (next >= entries.length) {
    return { history: { entries, index: entries.length, draft: '' }, text: history.draft };
  }
  return { history: { ...history, index: next }, text: entries[next] };
}

export function promptHistoryEntries(
  items: readonly { kind: string; text?: string; agentId?: string }[],
): string[] {
  const entries: string[] = [];
  for (const item of items) {
    if (item.kind !== 'user' || item.agentId) continue;
    const text = (item.text ?? '').trim();
    if (text) entries.push(text);
  }
  return entries;
}

export function handleHistoryKey(state: HistoryKeyState, key: string): HistoryKeyOutcome {
  const { history } = state;
  if (key !== 'ArrowUp' && key !== 'ArrowDown') {
    return { handled: false, history, text: state.value };
  }
  if (state.popupOpen) return { handled: true, history, text: state.value };

  const browsing = history.index < history.entries.length;
  if (browsing && (!state.selectionCollapsed || !state.caretAtEnd)) {
    return { handled: false, history: resetPromptHistory(history), text: state.value };
  }

  if (key === 'ArrowUp') {
    if (!browsing && (!state.selectionCollapsed || state.value.trim() !== '')) {
      return { handled: false, history, text: state.value };
    }
    const previous = historyPrevious(history, state.value);
    if (!previous) return { handled: false, history, text: state.value };
    return { handled: true, history: previous.history, text: previous.text };
  }

  const next = historyNext(history);
  if (!next) return { handled: false, history, text: state.value };
  return { handled: true, history: next.history, text: next.text };
}
