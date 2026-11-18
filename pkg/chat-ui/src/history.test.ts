import { describe, expect, it } from 'vitest';
import {
  MAX_PROMPT_HISTORY,
  createPromptHistory,
  handleHistoryKey,
  historyNext,
  historyPrevious,
  promptHistoryEntries,
  rememberPrompt,
  resetPromptHistory,
} from './history';

function state(history = createPromptHistory(), value = '') {
  return { history, value, selectionCollapsed: true, caretAtEnd: true, popupOpen: false };
}

describe('prompt history', () => {
  it('skips empty prompts and consecutive duplicates', () => {
    let history = createPromptHistory();
    history = rememberPrompt(history, '  first  ');
    history = rememberPrompt(history, 'first');
    history = rememberPrompt(history, '   ');
    history = rememberPrompt(history, 'second');

    expect(history.entries).toEqual(['first', 'second']);
    expect(history.index).toBe(2);
  });

  it('caps the history at the newest entries', () => {
    let history = createPromptHistory();
    for (let i = 0; i < MAX_PROMPT_HISTORY + 5; i += 1) history = rememberPrompt(history, `prompt ${i}`);

    expect(history.entries).toHaveLength(MAX_PROMPT_HISTORY);
    expect(history.entries[0]).toBe('prompt 5');
    expect(history.entries.at(-1)).toBe(`prompt ${MAX_PROMPT_HISTORY + 4}`);
  });

  it('walks backwards and forwards while keeping multiline text intact', () => {
    const history = createPromptHistory(['first', 'line one\nline two']);

    const older = historyPrevious(history, '');
    expect(older?.text).toBe('line one\nline two');
    const oldest = historyPrevious(older!.history, '');
    expect(oldest?.text).toBe('first');

    const newer = historyNext(oldest!.history);
    expect(newer?.text).toBe('line one\nline two');
    const draft = historyNext(newer!.history);
    expect(draft?.text).toBe('');
    expect(historyNext(draft!.history)).toBeNull();
  });

  it('restores the draft that was in the composer before browsing', () => {
    const history = createPromptHistory(['previous']);
    const recalled = historyPrevious(history, 'my draft');
    expect(recalled?.text).toBe('previous');

    const restored = historyNext(recalled!.history);
    expect(restored?.text).toBe('my draft');
    expect(restored?.history.draft).toBe('');
    expect(historyNext(restored!.history)).toBeNull();
  });

  it('treats the oldest entry as the end of navigation', () => {
    const history = createPromptHistory(['only']);
    const recalled = historyPrevious(history, 'draft');
    const again = historyPrevious(recalled!.history, recalled!.text);
    expect(again?.text).toBe('only');
    expect(again?.history.index).toBe(0);
  });

  it('resets navigation without dropping the entries', () => {
    const history = createPromptHistory(['first', 'second']);
    const browsing = historyPrevious(history, 'draft')!;
    const reset = resetPromptHistory(browsing.history);
    expect(reset.entries).toEqual(['first', 'second']);
    expect(reset.index).toBe(2);
    expect(reset.draft).toBe('');
  });

  it('seeds entries from user messages only', () => {
    expect(
      promptHistoryEntries([
        { kind: 'user', text: 'one' },
        { kind: 'assistant', text: 'reply' },
        { kind: 'user', text: '  ' },
        { kind: 'user', text: 'task prompt', agentId: 'explorer' },
        { kind: 'user', text: 'two' },
      ]),
    ).toEqual(['one', 'two']);
  });
});

describe('history key handling', () => {
  it('recalls the newest prompt only from an empty composer', () => {
    const history = createPromptHistory(['one', 'two']);
    expect(handleHistoryKey(state(history, ''), 'ArrowUp')).toMatchObject({
      handled: true,
      text: 'two',
    });
    expect(handleHistoryKey(state(history, 'typing'), 'ArrowUp')).toMatchObject({
      handled: false,
      text: 'typing',
    });
  });

  it('navigates and restores the draft', () => {
    let history = createPromptHistory(['one', 'two']);
    let value = '';
    const press = (key: string) => {
      const outcome = handleHistoryKey(
        { history, value, selectionCollapsed: true, caretAtEnd: true, popupOpen: false },
        key,
      );
      history = outcome.history;
      value = outcome.text;
      return outcome;
    };

    expect(press('ArrowUp').text).toBe('two');
    expect(press('ArrowUp').text).toBe('one');
    expect(press('ArrowDown').text).toBe('two');
    expect(press('ArrowDown').text).toBe('');
    expect(press('ArrowDown').handled).toBe(false);
    expect(value).toBe('');
  });

  it('does not recall while a popup is open', () => {
    const current = { ...state(createPromptHistory(['one']), ''), popupOpen: true };
    expect(handleHistoryKey(current, 'ArrowUp')).toMatchObject({ handled: true, text: '' });
  });

  it('leaves browsing when the caret moved and keeps the text', () => {
    const browsing = state(createPromptHistory(['one']), '');
    const recalled = handleHistoryKey(browsing, 'ArrowUp');
    const moved = {
      ...browsing,
      history: recalled.history,
      value: recalled.text,
      caretAtEnd: false,
    };
    const outcome = handleHistoryKey(moved, 'ArrowUp');
    expect(outcome.handled).toBe(false);
    expect(outcome.text).toBe('one');
    expect(outcome.history.index).toBe(outcome.history.entries.length);
  });

  it('ignores keys other than Up and Down', () => {
    const current = state(createPromptHistory(['one']), '');
    expect(handleHistoryKey(current, 'ArrowLeft').handled).toBe(false);
    expect(handleHistoryKey(current, 'Enter').handled).toBe(false);
  });
});
