export type SerializeMode = 'editor' | 'prompt';

export interface EditorPiece {
  node: Node;
  start: number;
  end: number;
  chip?: string;
}

interface Point {
  node: Node;
  offset: number;
}

const BLOCK_TAGS = new Set([
  'ADDRESS',
  'ARTICLE',
  'ASIDE',
  'BLOCKQUOTE',
  'DD',
  'DIV',
  'DL',
  'DT',
  'FIELDSET',
  'FIGCAPTION',
  'FIGURE',
  'FOOTER',
  'FORM',
  'H1',
  'H2',
  'H3',
  'H4',
  'H5',
  'H6',
  'HEADER',
  'LI',
  'MAIN',
  'NAV',
  'OL',
  'P',
  'PRE',
  'SECTION',
  'TABLE',
  'TR',
  'UL',
]);

const INLINE_TAGS = new Set([
  'A',
  'B',
  'CODE',
  'DEL',
  'EM',
  'FONT',
  'I',
  'INS',
  'MARK',
  'S',
  'SMALL',
  'SPAN',
  'STRONG',
  'SUB',
  'SUP',
  'U',
]);

function isElement(node: Node | null | undefined): node is HTMLElement {
  return !!node && node.nodeType === 1;
}

export function isChipNode(node: Node | null | undefined): node is HTMLElement {
  return isElement(node) && node.hasAttribute('data-chip');
}

export function chipName(chip: HTMLElement): string {
  return chip.getAttribute('data-chip') ?? '';
}

class Builder {
  text = '';
  pieces: EditorPiece[] = [];
  private pendingBreak = false;

  break(): void {
    if (this.text && !this.text.endsWith('\n')) this.pendingBreak = true;
  }

  newline(): void {
    this.pendingBreak = false;
    this.text += '\n';
  }

  write(node: Node, value: string, chip?: string): void {
    if (!value) return;
    if (this.pendingBreak) {
      this.pendingBreak = false;
      this.text += '\n';
    }
    const start = this.text.length;
    this.text += value;
    this.pieces.push({ node, start, end: this.text.length, chip });
  }
}

function walkNode(node: Node, mode: SerializeMode, out: Builder): void {
  if (node.nodeType === 3) {
    out.write(node, (node as Text).data.replace(/\u00a0/g, ' '));
    return;
  }
  if (!isElement(node)) return;
  if (node.hasAttribute('data-chip')) {
    out.write(node, mode === 'prompt' ? `/${chipName(node)}` : ' ', chipName(node));
    return;
  }
  if (node.tagName === 'BR') {
    out.newline();
    return;
  }
  const block = BLOCK_TAGS.has(node.tagName);
  if (block) out.break();
  node.childNodes.forEach((child) => walkNode(child, mode, out));
  if (block) out.break();
}

function serialize(root: Node, mode: SerializeMode): { text: string; pieces: EditorPiece[] } {
  const out = new Builder();
  if (root.nodeType === 3) {
    out.write(root, (root as Text).data.replace(/\u00a0/g, ' '));
  } else {
    root.childNodes.forEach((child) => walkNode(child, mode, out));
  }
  return { text: out.text, pieces: out.pieces };
}

export function editorText(root: Node): string {
  return serialize(root, 'editor').text;
}

export function promptText(root: Node): string {
  return serialize(root, 'prompt').text;
}

function fragmentHost(fragment: DocumentFragment): HTMLElement {
  const host = document.createElement('div');
  host.appendChild(fragment);
  return host;
}

export function currentRange(root: HTMLElement): Range | null {
  const selection = window.getSelection();
  if (!selection || selection.rangeCount === 0) return null;
  const range = selection.getRangeAt(0);
  if (!root.contains(range.startContainer)) return null;
  return range;
}

export function endRange(root: HTMLElement): Range {
  const range = document.createRange();
  range.selectNodeContents(root);
  range.collapse(false);
  return range;
}

export function focusRange(range: Range): void {
  const selection = window.getSelection();
  if (!selection) return;
  selection.removeAllRanges();
  selection.addRange(range);
}

export function caretOffset(root: HTMLElement): number | null {
  const range = currentRange(root);
  if (!range) return null;
  const before = document.createRange();
  before.selectNodeContents(root);
  try {
    before.setEnd(range.startContainer, range.startOffset);
  } catch {
    return null;
  }
  const text = editorText(fragmentHost(before.cloneContents()));
  return Math.min(text.length, editorText(root).length);
}

function locate(pieces: EditorPiece[], offset: number): Point | null {
  for (const piece of pieces) {
    if (offset < piece.start || offset > piece.end) continue;
    if (piece.chip !== undefined) continue;
    const max = piece.end - piece.start;
    return { node: piece.node, offset: Math.max(0, Math.min(offset - piece.start, max)) };
  }
  return null;
}

export function rangeForOffsets(root: HTMLElement, start: number, end: number): Range | null {
  const { pieces } = serialize(root, 'editor');
  const from = locate(pieces, start);
  const to = locate(pieces, end);
  if (!from || !to) return null;
  const range = document.createRange();
  try {
    range.setStart(from.node, from.offset);
    range.setEnd(to.node, to.offset);
  } catch {
    return null;
  }
  return range;
}

export function createChip(name: string, removeLabel = 'Remove'): HTMLElement {
  const chip = document.createElement('span');
  chip.className = 'composer-chip';
  chip.setAttribute('contenteditable', 'false');
  chip.setAttribute('data-chip', name);
  chip.setAttribute('aria-label', `/${name}`);

  const label = document.createElement('span');
  label.className = 'composer-chip-label';
  label.textContent = `/${name}`;

  const remove = document.createElement('span');
  remove.className = 'composer-chip-x';
  remove.setAttribute('data-chip-remove', '');
  remove.setAttribute('role', 'button');
  remove.setAttribute('aria-label', removeLabel);
  remove.textContent = '×';

  chip.append(label, remove);
  return chip;
}

export function syncChipName(chip: HTMLElement, name: string, removeLabel = 'Remove'): void {
  chip.setAttribute('data-chip', name);
  chip.setAttribute('aria-label', `/${name}`);
  const label = chip.querySelector('.composer-chip-label');
  if (label) label.textContent = `/${name}`;
  const remove = chip.querySelector('[data-chip-remove]');
  if (remove) remove.setAttribute('aria-label', removeLabel);
}

function caretAfter(chip: HTMLElement): void {
  const range = document.createRange();
  const next = chip.nextSibling;
  if (next && next.nodeType === 3 && (next as Text).data.startsWith(' ')) {
    range.setStart(next, 1);
  } else {
    range.setStartAfter(chip);
  }
  range.collapse(true);
  focusRange(range);
}

export function insertChip(
  root: HTMLElement,
  range: Range | null,
  name: string,
  removeLabel = 'Remove',
): HTMLElement {
  const target = range && root.contains(range.startContainer) ? range : endRange(root);
  target.deleteContents();
  const chip = createChip(name, removeLabel);
  target.insertNode(chip);
  const parent = chip.parentNode;
  if (!parent) return chip;
  const space = document.createTextNode(' ');
  parent.insertBefore(space, chip.nextSibling);
  caretAfter(chip);
  return chip;
}

export function removeChip(chip: HTMLElement): void {
  const parent = chip.parentNode;
  if (!parent) return;
  const range = document.createRange();
  range.setStartBefore(chip);
  range.collapse(true);
  const next = chip.nextSibling;
  let separator: Node | null = null;
  if (next && next.nodeType === 3) {
    const text = next as Text;
    if (text.data.startsWith(' ')) {
      if (text.data.length === 1) separator = text;
      else text.data = text.data.slice(1);
    }
  }
  parent.removeChild(chip);
  if (separator) parent.removeChild(separator);
  focusRange(range);
}

export function adjacentChip(root: HTMLElement, direction: 'before' | 'after'): HTMLElement | null {
  const range = currentRange(root);
  if (!range || !range.collapsed) return null;
  const node = range.startContainer;
  const offset = range.startOffset;
  let candidate: Node | null = null;
  if (node.nodeType === 3) {
    const text = node as Text;
    const onlySpace = text.data.trim() === '';
    if (direction === 'before' && (offset === 0 || (onlySpace && offset === text.data.length))) {
      candidate = text.previousSibling;
    }
    if (direction === 'after' && (offset === text.data.length || (onlySpace && offset === 0))) {
      candidate = text.nextSibling;
    }
  } else if (direction === 'before') {
    candidate = node.childNodes[offset - 1] ?? null;
  } else {
    candidate = node.childNodes[offset] ?? null;
  }
  for (let hops = 0; candidate && hops < 4; hops += 1) {
    if (isChipNode(candidate)) return candidate;
    if (candidate.nodeType === 3 && (candidate as Text).data.length === 0) {
      candidate = direction === 'before' ? candidate.previousSibling : candidate.nextSibling;
      continue;
    }
    return null;
  }
  return isChipNode(candidate) ? candidate : null;
}

export function setEditorText(root: HTMLElement, value: string): void {
  root.textContent = '';
  const lines = value.replace(/\r\n?/g, '\n').split('\n');
  lines.forEach((line, index) => {
    if (index > 0) root.appendChild(document.createElement('br'));
    if (line) root.appendChild(document.createTextNode(line));
  });
}


export function insertPlainText(root: HTMLElement, value: string, range?: Range | null): void {
  const target = range ?? currentRange(root) ?? endRange(root);
  const startContainer = target.startContainer;
  const startOffset = target.startOffset;
  target.deleteContents();
  const caret = document.createRange();
  if (value) {
    const fragment = document.createDocumentFragment();
    const lines = value.replace(/\r\n?/g, '\n').split('\n');
    lines.forEach((line, index) => {
      if (index > 0) fragment.appendChild(document.createElement('br'));
      if (line) fragment.appendChild(document.createTextNode(line));
    });
    const last = fragment.lastChild;
    target.insertNode(fragment);
    if (last && last.parentNode) caret.setStartAfter(last);
    else caret.setStart(startContainer, startOffset);
  } else {
    caret.setStart(startContainer, startOffset);
  }
  caret.collapse(true);
  focusRange(caret);
}

function stripAttributes(element: HTMLElement): void {
  for (const name of element.getAttributeNames()) element.removeAttribute(name);
}

export function flatten(root: HTMLElement): void {
  const elements = Array.from(root.querySelectorAll<HTMLElement>('*'));
  for (const element of elements) {
    if (!root.contains(element)) continue;
    if (element.hasAttribute('data-chip') || element.closest('[data-chip]')) continue;
    if (element.tagName === 'BR') continue;
    if (!INLINE_TAGS.has(element.tagName)) {
      stripAttributes(element);
      continue;
    }
    const parent = element.parentNode;
    if (!parent) continue;
    while (element.firstChild) parent.insertBefore(element.firstChild, element);
    parent.removeChild(element);
  }
}

