import { act, fireEvent, render, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { Composer } from './Composer';
import { api } from '../lib/bridge';
import { editorText, focusRange } from '../lib/editor';
import { installHubSkill, sendPrompt } from '../state/actions';
import { emptyChat, appendUser } from '../state/chat';
import { store } from '../state/store';
import type { HubSkill, SlashCommand } from '../types';

vi.mock('../lib/bridge', () => ({
  api: {
    SearchHubSkills: vi.fn(async () => []),
    SearchFiles: vi.fn(async () => []),
    ReadAttachment: vi.fn(),
  },
  hasWails: false,
}));

vi.mock('../state/actions', () => ({
  attachFiles: vi.fn(async () => []),
  cancelPrompt: vi.fn(),
  changeMode: vi.fn(),
  changeModel: vi.fn(),
  errorText: (err: unknown) => String(err),
  installHubSkill: vi.fn(async () => ({})),
  loadModels: vi.fn(),
  sendPrompt: vi.fn(async () => undefined),
  toggleRecording: vi.fn(),
}));

const submitPrompt = vi.mocked(sendPrompt);
const install = vi.mocked(installHubSkill);
const searchHub = vi.mocked(api.SearchHubSkills);

Element.prototype.scrollIntoView = () => undefined;

const HUB_SKILL: HubSkill = {
  name: 'remotion',
  skillId: 'remotion',
  source: 'pegg/skills',
  url: '',
  official: true,
};

function setup(commands: SlashCommand[] = [{ name: 'remotion', description: 'Make videos' }]) {
  store.set({
    workspace: '/tmp/project',
    language: 'en',
    commandsCatalog: [],
    prefs: { ...store.get().prefs, sendOnEnter: true },
  });
  store.setChat(() => ({ ...emptyChat(), commands }));
}

function mount() {
  const { container } = render(<Composer />);
  const editor = container.querySelector('.composer-input') as HTMLElement;
  return { container, editor };
}

function type(editor: HTMLElement, value: string) {
  editor.textContent = value;
  fireEvent.input(editor);
}

function appendText(editor: HTMLElement, value: string) {
  editor.append(document.createTextNode(value));
  fireEvent.input(editor);
}

describe('Composer slash skills', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    searchHub.mockResolvedValue([]);
    submitPrompt.mockResolvedValue(undefined);
    install.mockResolvedValue({});
    setup();
  });

  it('turns a picked skill into a chip and closes the menu', () => {
    const { container, editor } = mount();

    act(() => type(editor, '/rem'));
    expect(container.querySelector('.slash-menu')).not.toBeNull();
    expect(container.textContent).toContain('/remotion');

    fireEvent.keyDown(editor, { key: 'Enter' });

    expect(editor.querySelector('[data-chip="remotion"]')).not.toBeNull();
    expect(editorText(editor)).toBe('  ');
    expect(container.querySelector('.slash-menu')).toBeNull();
  });

  it('stops searching the hub once the skill is a chip', async () => {
    const { container, editor } = mount();

    act(() => type(editor, '/rem'));
    fireEvent.keyDown(editor, { key: 'Enter' });
    act(() => appendText(editor, 'make a video'));

    await new Promise((resolve) => setTimeout(resolve, 400));

    expect(searchHub).not.toHaveBeenCalled();
    expect(container.querySelector('.slash-menu')).toBeNull();
    expect(editorText(editor)).toBe('  make a video');
  });

  it('sends the chip back as a slash command on Enter', () => {
    const { editor } = mount();

    act(() => type(editor, '/rem'));
    fireEvent.keyDown(editor, { key: 'Enter' });
    act(() => appendText(editor, 'make a video'));
    fireEvent.keyDown(editor, { key: 'Enter' });

    expect(submitPrompt).toHaveBeenCalledWith({
      text: '/remotion make a video',
      attachments: [],
    });
    expect(editor.querySelector('[data-chip]')).toBeNull();
  });

  it('installs a hub skill and chips the installed name', async () => {
    setup([]);
    searchHub.mockResolvedValue([HUB_SKILL]);
    install.mockResolvedValue({ name: 'remotion-video' });
    const { container, editor } = mount();

    act(() => type(editor, '/remo'));
    await waitFor(() => expect(container.querySelector('.slash-item.hub')).not.toBeNull());

    fireEvent.keyDown(editor, { key: 'Enter' });
    await waitFor(() => expect(editor.querySelector('[data-chip="remotion-video"]')).not.toBeNull());

    expect(install).toHaveBeenCalledWith('pegg/skills', 'remotion', 'global');
    expect(editorText(editor)).toBe('  ');
  });

  it('removes a chip from its close button', () => {
    const { editor } = mount();

    act(() => type(editor, '/rem'));
    fireEvent.keyDown(editor, { key: 'Enter' });
    const close = editor.querySelector('[data-chip-remove]') as HTMLElement;

    fireEvent.mouseDown(close);

    expect(editor.querySelector('[data-chip]')).toBeNull();
    expect(editorText(editor)).toBe('');
  });

  it('removes a chip with Backspace', () => {
    const { editor } = mount();

    act(() => type(editor, '/rem'));
    fireEvent.keyDown(editor, { key: 'Enter' });
    const space = editor.querySelector('[data-chip]')?.nextSibling as Text;
    focusRange(
      (() => {
        const range = document.createRange();
        range.setStart(space, 1);
        range.collapse(true);
        return range;
      })(),
    );

    fireEvent.keyDown(editor, { key: 'Backspace' });

    expect(editor.querySelector('[data-chip]')).toBeNull();
  });

  it('keeps Shift+Enter as a newline and Escape as a menu dismissal', () => {
    const { container, editor } = mount();

    act(() => type(editor, 'hello'));
    fireEvent.keyDown(editor, { key: 'Enter', shiftKey: true });
    expect(submitPrompt).not.toHaveBeenCalled();
    expect(editorText(editor)).toBe('hello\n');

    act(() => type(editor, '/rem'));
    fireEvent.keyDown(editor, { key: 'Escape' });
    expect(container.querySelector('.slash-menu')).toBeNull();
  });
});

function seedHistory(...prompts: string[]) {
  setup();
  store.setChat((chat) => prompts.reduce((next, prompt) => appendUser(next, prompt), chat));
}

describe('Composer prompt history', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    searchHub.mockResolvedValue([]);
    submitPrompt.mockResolvedValue(undefined);
    install.mockResolvedValue({});
  });

  it('recalls history from an empty composer and walks Up/Down', () => {
    seedHistory('first', 'second');
    const { editor } = mount();

    fireEvent.keyDown(editor, { key: 'ArrowUp' });
    expect(editorText(editor)).toBe('second');

    fireEvent.keyDown(editor, { key: 'ArrowUp' });
    expect(editorText(editor)).toBe('first');

    fireEvent.keyDown(editor, { key: 'ArrowUp' });
    expect(editorText(editor)).toBe('first');

    fireEvent.keyDown(editor, { key: 'ArrowDown' });
    expect(editorText(editor)).toBe('second');

    fireEvent.keyDown(editor, { key: 'ArrowDown' });
    expect(editorText(editor)).toBe('');
  });

  it('restores the draft with Down past the newest entry', () => {
    seedHistory('previous');
    const { editor } = mount();

    act(() => type(editor, 'my draft'));
    fireEvent.keyDown(editor, { key: 'ArrowUp', altKey: true });
    expect(editorText(editor)).toBe('previous');

    fireEvent.keyDown(editor, { key: 'ArrowDown', altKey: true });
    expect(editorText(editor)).toBe('my draft');
  });

  it('seeds prompts that finish loading after mount', () => {
    setup();
    const { editor } = mount();

    act(() => {
      store.setChat((chat) => appendUser(chat, 'loaded later'));
    });

    fireEvent.keyDown(editor, { key: 'ArrowUp' });
    expect(editorText(editor)).toBe('loaded later');
  });

  it('keeps multiline prompts intact', () => {
    seedHistory('line one\nline two\nline three');
    const { editor } = mount();

    fireEvent.keyDown(editor, { key: 'ArrowUp' });
    expect(editorText(editor)).toBe('line one\nline two\nline three');
  });

  it('stops browsing after an edit and keeps the text', () => {
    seedHistory('previous');
    const { editor } = mount();

    fireEvent.keyDown(editor, { key: 'ArrowUp' });
    expect(editorText(editor)).toBe('previous');

    act(() => type(editor, 'edited'));
    fireEvent.keyDown(editor, { key: 'ArrowDown' });
    expect(editorText(editor)).toBe('edited');
  });

  it('adds submitted prompts to the session history', () => {
    seedHistory('first');
    const { editor } = mount();

    act(() => type(editor, 'second'));
    fireEvent.keyDown(editor, { key: 'Enter' });
    expect(submitPrompt).toHaveBeenCalledWith({ text: 'second', attachments: [] });
    expect(editorText(editor)).toBe('');

    fireEvent.keyDown(editor, { key: 'ArrowUp' });
    expect(editorText(editor)).toBe('second');
    fireEvent.keyDown(editor, { key: 'ArrowUp' });
    expect(editorText(editor)).toBe('first');
  });

  it('leaves slash-menu navigation in control of the arrows', () => {
    seedHistory('previous');
    const { container, editor } = mount();

    act(() => type(editor, '/rem'));
    expect(container.querySelector('.slash-menu')).not.toBeNull();

    fireEvent.keyDown(editor, { key: 'ArrowUp' });
    expect(editorText(editor)).toBe('/rem');
    expect(container.querySelector('.slash-menu')).not.toBeNull();
  });
});
