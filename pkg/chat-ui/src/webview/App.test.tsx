import { act, fireEvent, render } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { App } from './App';
import { setHost } from './host';
import { appendUser, initialState, type ChatState } from '../state';

const postMessage = vi.fn();

function session(...prompts: string[]): ChatState {
  let state: ChatState = { ...initialState(), acpSessionId: 'session-1', peggSessionId: 'pegg-1' };
  for (const prompt of prompts) state = appendUser(state, prompt);
  return state;
}

function pushState(state: ChatState) {
  act(() => {
    window.dispatchEvent(
      new MessageEvent('message', {
        data: {
          type: 'state',
          state,
          settings: { sendOnEnter: 'enter', defaultMode: 'auto' },
        },
      }),
    );
  });
}

function mount() {
  const { container } = render(<App />);
  return { container, textarea: container.querySelector('textarea') as HTMLTextAreaElement };
}

describe('chat-ui prompt history', () => {
  beforeEach(() => {
    postMessage.mockClear();
    setHost({ postMessage });
    Element.prototype.scrollIntoView = () => undefined;
  });

  it('recalls and walks prompts with the arrow keys', () => {
    const { textarea } = mount();
    pushState(session('first', 'second'));

    fireEvent.keyDown(textarea, { key: 'ArrowUp' });
    expect(textarea.value).toBe('second');

    fireEvent.keyDown(textarea, { key: 'ArrowUp' });
    expect(textarea.value).toBe('first');

    fireEvent.keyDown(textarea, { key: 'ArrowDown' });
    expect(textarea.value).toBe('second');

    fireEvent.keyDown(textarea, { key: 'ArrowDown' });
    expect(textarea.value).toBe('');
  });

  it('adds sent prompts to the history', () => {
    const { textarea } = mount();
    pushState(session('first'));

    fireEvent.change(textarea, { target: { value: 'second' } });
    fireEvent.keyDown(textarea, { key: 'Enter' });

    expect(postMessage).toHaveBeenCalledWith({ type: 'send', text: 'second' });

    fireEvent.keyDown(textarea, { key: 'ArrowUp' });
    expect(textarea.value).toBe('second');
    fireEvent.keyDown(textarea, { key: 'ArrowUp' });
    expect(textarea.value).toBe('first');
  });

  it('leaves suggestion navigation in control of the arrows', () => {
    const { container, textarea } = mount();
    pushState(session('previous'));

    fireEvent.change(textarea, { target: { value: '/' } });
    fireEvent.change(textarea, { target: { value: '/r' } });
    fireEvent.change(textarea, { target: { value: '/re' } });
    fireEvent.change(textarea, { target: { value: '/rev' } });
    expect(container.querySelector('.suggestions-popup')).not.toBeNull();

    fireEvent.keyDown(textarea, { key: 'ArrowUp' });
    expect(textarea.value).toBe('/rev');
  });
});
