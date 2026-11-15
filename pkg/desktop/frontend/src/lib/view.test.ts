import { describe, expect, it } from 'vitest';
import { bindingMatches, matchViewBinding, viewLineSpans, viewLineText } from './view';

function keyEvent(init: KeyboardEventInit & { key: string }): KeyboardEvent {
  return new KeyboardEvent('keydown', init);
}

describe('view lines', () => {
  it('flattens spans and plain text', () => {
    expect(viewLineText({ spans: [{ text: 'a' }, { text: 'b' }] })).toBe('ab');
    expect(viewLineSpans({ text: 'x' })).toEqual([{ text: 'x' }]);
    expect(viewLineSpans({})).toEqual([]);
  });
});

describe('view key bindings', () => {
  it('matches ctrl on both platforms', () => {
    expect(bindingMatches(keyEvent({ key: 'k', ctrlKey: true }), 'ctrl+k')).toBe(true);
    expect(bindingMatches(keyEvent({ key: 'k', metaKey: true }), 'ctrl+k')).toBe(true);
    expect(bindingMatches(keyEvent({ key: 'k' }), 'ctrl+k')).toBe(false);
  });

  it('matches named keys and function keys', () => {
    expect(bindingMatches(keyEvent({ key: 'Enter', altKey: true }), 'alt+enter')).toBe(true);
    expect(bindingMatches(keyEvent({ key: 'F2' }), 'f2')).toBe(true);
    expect(bindingMatches(keyEvent({ key: 'Tab', shiftKey: true }), 'shift+tab')).toBe(true);
    expect(bindingMatches(keyEvent({ key: 'ArrowUp', ctrlKey: true }), 'ctrl+up')).toBe(true);
  });

  it('ignores unmodified runes and resolves bindings', () => {
    expect(bindingMatches(keyEvent({ key: 'k' }), 'k')).toBe(false);
    const match = matchViewBinding(keyEvent({ key: 'k', ctrlKey: true }), [
      { id: 'a', key: 'ctrl+k', label: 'A' },
    ]);
    expect(match?.id).toBe('a');
    expect(matchViewBinding(keyEvent({ key: 'k' }), [])).toBeNull();
  });
});
