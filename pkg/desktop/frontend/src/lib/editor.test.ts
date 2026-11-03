import { beforeEach, describe, expect, it } from 'vitest';
import {
  adjacentChip,
  caretOffset,
  createChip,
  editorText,
  endRange,
  flatten,
  focusRange,
  insertChip,
  insertPlainText,
  promptText,
  rangeForOffsets,
  removeChip,
  setEditorText,
  syncChipName,
} from './editor';

function editor(html = ''): HTMLElement {
  const el = document.createElement('div');
  el.setAttribute('contenteditable', 'true');
  el.innerHTML = html;
  document.body.appendChild(el);
  return el;
}

function caretAt(node: Node, offset: number): void {
  const range = document.createRange();
  range.setStart(node, offset);
  range.collapse(true);
  focusRange(range);
}

describe('editor serialization', () => {
  beforeEach(() => {
    document.body.innerHTML = '';
  });

  it('reads text nodes, line breaks and block wrappers', () => {
    const el = editor('hello<br>world<div>second block</div>');
    expect(editorText(el)).toBe('hello\nworld\nsecond block');
  });

  it('serializes chips as a space for the editor and as /name for the prompt', () => {
    const el = editor('make ');
    insertChip(el, endRange(el), 'remotion');
    el.append(document.createTextNode('a video'));

    expect(editorText(el)).toBe('make   a video');
    expect(promptText(el)).toBe('make /remotion a video');
  });

  it('normalizes non-breaking spaces', () => {
    const el = editor('a\u00a0b');
    expect(editorText(el)).toBe('a b');
  });

  it('updates a chip label in place', () => {
    const el = editor('');
    const chip = insertChip(el, endRange(el), 'remotion');
    syncChipName(chip, 'remotion-video');
    expect(chip.getAttribute('data-chip')).toBe('remotion-video');
    expect(chip.querySelector('.composer-chip-label')?.textContent).toBe('/remotion-video');
    expect(promptText(el).trim()).toBe('/remotion-video');
  });
});

describe('editor caret mapping', () => {
  beforeEach(() => {
    document.body.innerHTML = '';
  });

  it('reports the caret offset in editor coordinates', () => {
    const el = editor('hello');
    const text = el.firstChild as Text;
    caretAt(text, 5);
    expect(caretOffset(el)).toBe(5);
  });

  it('treats a chip as a single character boundary', () => {
    const el = editor('');
    const chip = insertChip(el, endRange(el), 'remotion');
    el.append(document.createTextNode('go'));
    caretAt(el.lastChild as Text, 2);
    expect(editorText(el)).toBe('  go');
    expect(caretOffset(el)).toBe(4);
    expect(editorText(el).slice(0, 2)).toBe('  ');
    expect(chip.getAttribute('data-chip')).toBe('remotion');
  });

  it('maps offsets back to a range that can be replaced', () => {
    const el = editor('build /rem');
    const range = rangeForOffsets(el, 6, 10);
    expect(range).not.toBeNull();
    expect(range?.toString()).toBe('/rem');
  });

  it('returns null when the offsets fall inside a chip', () => {
    const el = editor('');
    insertChip(el, endRange(el), 'remotion');
    expect(rangeForOffsets(el, 0, 1)).toBeNull();
  });
});

describe('editor chip insertion and removal', () => {
  beforeEach(() => {
    document.body.innerHTML = '';
  });

  it('replaces the slash token with a chip and leaves a separator space', () => {
    const el = editor('/rem');
    const range = rangeForOffsets(el, 0, 4);
    const chip = insertChip(el, range, 'remotion');
    expect(chip.parentNode).toBe(el);
    expect(editorText(el)).toBe('  ');
    expect(promptText(el)).toBe('/remotion ');
    expect(el.querySelector('[data-chip="remotion"]')).toBe(chip);
  });

  it('removes the chip and its separator', () => {
    const el = editor('make ');
    const chip = insertChip(el, endRange(el), 'remotion');
    removeChip(chip);
    expect(el.querySelector('[data-chip]')).toBeNull();
    expect(editorText(el)).toBe('make ');
  });

  it('finds a chip adjacent to the caret for Backspace', () => {
    const el = editor('');
    const chip = insertChip(el, endRange(el), 'remotion');
    const space = chip.nextSibling as Text;
    caretAt(space, 1);
    expect(adjacentChip(el, 'before')).toBe(chip);
    expect(adjacentChip(el, 'after')).toBeNull();
  });

  it('creates removable chips with an accessible label', () => {
    const chip = createChip('review', 'Remove skill');
    expect(chip.querySelector('[data-chip-remove]')?.getAttribute('aria-label')).toBe('Remove skill');
  });
});

describe('editor imperative helpers', () => {
  beforeEach(() => {
    document.body.innerHTML = '';
  });

  it('writes text with newlines', () => {
    const el = editor('');
    setEditorText(el, 'one\ntwo');
    expect(editorText(el)).toBe('one\ntwo');
  });

  it('inserts plain text at the caret', () => {
    const el = editor('hello ');
    caretAt(el.firstChild as Text, 6);
    insertPlainText(el, 'world\nagain');
    expect(editorText(el)).toBe('hello world\nagain');
  });

  it('unwraps injected inline markup but keeps chips', () => {
    const el = editor('<b>bold</b> text');
    const chip = createChip('remotion');
    el.append(chip);
    flatten(el);
    expect(el.querySelector('b')).toBeNull();
    expect(el.querySelector('[data-chip="remotion"]')).toBe(chip);
    expect(editorText(el)).toBe('bold text ');
  });
});
