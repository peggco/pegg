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

function keyState(history = createPromptHistory(), value = '') {
  return { history, value, selectionCollapsed: true, caretAtEnd: true, popupOpen: false };
}

describe('desktop prompt history', () => {
  it('remembers prompts, skipping empties and consecutive duplicates', () => {
    let history = createPromptHistory();
    history = rememberPrompt(history, '  first  ');
    history = rememberPrompt(history, 'first');
    history = rememberPrompt(history, '   ');
    history = rememberPrompt(history, 'second\nline two');

    expect(history.entries).toEqual(['first', 'second\nline two']);
  });

  it('caps history and drops the oldest entries', () => {
    let history = createPromptHistory();
    for (let i = 0; i < MAX_PROMPT_HISTORY + 3; i += 1) history = rememberPrompt(history, `prompt ${i}`);
    expect(history.entries).toHaveLength(MAX_PROMPT_HISTORY);
    expect(history.entries[0]).toBe('prompt 3');
  });

  it('seeds from user messages only', () => {
    expect(
      promptHistoryEntries([
        { kind: 'user', text: 'one' },
        { kind: 'assistant', text: 'reply' },
        { kind: 'user', text: 'task', agentId: 'developer' },
        { kind: 'user', text: 'two' },
      ]),
    ).toEqual(['one', 'two']);
  });

  it('navigates, restores the draft, and keeps multiline text', () => {
    let state = keyState(createPromptHistory(['first', 'line one\nline two']), '');
    let outcome = handleHistoryKey(state, 'ArrowUp');
    expect(outcome.text).toBe('line one\nline two');
    state = { ...state, ...outcome };
    outcome = handleHistoryKey(state, 'ArrowUp');
    expect(outcome.text).toBe('first');
    state = { ...state, ...outcome };
    outcome = handleHistoryKey(state, 'ArrowUp');
    expect(outcome.text).toBe('first');
    expect(outcome.handled).toBe(true);
    state = { ...state, ...outcome };
    outcome = handleHistoryKey(state, 'ArrowDown');
    expect(outcome.text).toBe('line one\nline two');
    state = { ...state, ...outcome };
    outcome = handleHistoryKey(state, 'ArrowDown');
    expect(outcome.text).toBe('');
    expect(handleHistoryKey({ ...state, ...outcome }, 'ArrowDown').handled).toBe(false);
  });

  it('restores a saved draft when leaving history', () => {
    const history = createPromptHistory(['previous']);
    const recalled = historyPrevious(history, 'my draft')!;
    expect(recalled.text).toBe('previous');
    expect(historyNext(recalled.history)?.text).toBe('my draft');
  });

  it('recalls only from an empty composer when not browsing', () => {
    const history = createPromptHistory(['previous']);
    expect(handleHistoryKey(keyState(history, ''), 'ArrowUp').text).toBe('previous');
    expect(handleHistoryKey(keyState(history, 'typing'), 'ArrowUp').handled).toBe(false);
  });

  it('keeps popups in control of the arrows', () => {
    const state = { ...keyState(createPromptHistory(['previous']), ''), popupOpen: true };
    expect(handleHistoryKey(state, 'ArrowUp')).toMatchObject({ handled: true, text: '' });
  });

  it('exits browsing when the caret moved', () => {
    const state = keyState(createPromptHistory(['previous']), '');
    const recalled = { ...state, ...handleHistoryKey(state, 'ArrowUp') };
    const moved = { ...recalled, value: recalled.text, caretAtEnd: false };
    const outcome = handleHistoryKey(moved, 'ArrowDown');
    expect(outcome.handled).toBe(false);
    expect(outcome.text).toBe('previous');
    expect(outcome.history.index).toBe(outcome.history.entries.length);
  });

  it('resets navigation without dropping entries', () => {
    const history = createPromptHistory(['first', 'second']);
    const browsing = historyPrevious(history, 'draft')!;
    const reset = resetPromptHistory(browsing.history);
    expect(reset.entries).toEqual(['first', 'second']);
    expect(reset.index).toBe(2);
    expect(historyNext(reset)).toBeNull();
  });
});
