import type { ViewKeyBinding, ViewLine, ViewSpan } from '../types';

const NAMED_KEYS: Record<string, string> = {
  tab: 'tab',
  enter: 'enter',
  return: 'enter',
  escape: 'escape',
  esc: 'escape',
  space: ' ',
  backspace: 'backspace',
  delete: 'delete',
  insert: 'insert',
  home: 'home',
  end: 'end',
  up: 'arrowup',
  arrowup: 'arrowup',
  down: 'arrowdown',
  arrowdown: 'arrowdown',
  left: 'arrowleft',
  arrowleft: 'arrowleft',
  right: 'arrowright',
  arrowright: 'arrowright',
  pgup: 'pageup',
  pageup: 'pageup',
  pgdn: 'pagedown',
  pagedown: 'pagedown',
};

export function viewLineSpans(line: ViewLine): ViewSpan[] {
  if (line.spans?.length) return line.spans;
  if (line.text) return [{ text: line.text }];
  return [];
}

export function viewLineText(line: ViewLine): string {
  return viewLineSpans(line)
    .map((span) => span.text)
    .join('');
}

export function matchViewBinding(
  event: KeyboardEvent,
  bindings?: ViewKeyBinding[],
): ViewKeyBinding | null {
  if (!bindings?.length || event.isComposing) return null;
  for (const binding of bindings) {
    if (bindingMatches(event, binding.key)) return binding;
  }
  return null;
}

export function bindingMatches(event: KeyboardEvent, spec: string): boolean {
  const parts = spec
    .toLowerCase()
    .split('+')
    .map((part) => part.trim())
    .filter(Boolean);
  let name = '';
  let wantCtrl = false;
  let wantMeta = false;
  let wantAlt = false;
  let wantShift = false;
  for (const part of parts) {
    switch (part) {
      case 'ctrl':
      case 'control':
        wantCtrl = true;
        break;
      case 'cmd':
      case 'meta':
      case 'super':
        wantMeta = true;
        break;
      case 'alt':
      case 'option':
        wantAlt = true;
        break;
      case 'shift':
        wantShift = true;
        break;
      default:
        name = part;
    }
  }
  if (!name) return false;
  if (wantCtrl && !(event.ctrlKey || event.metaKey)) return false;
  if (!wantCtrl && wantMeta && !event.metaKey) return false;
  if (wantAlt && !event.altKey) return false;
  if (wantShift && !event.shiftKey) return false;

  const key = event.key.toLowerCase();
  if (/^f([1-9]|1[0-2])$/.test(name)) return key === name;
  const named = NAMED_KEYS[name];
  if (named) return key === named;
  if (name.length !== 1) return false;
  if (!wantCtrl && !wantMeta && !wantAlt && !wantShift) return false;
  return key === name;
}
