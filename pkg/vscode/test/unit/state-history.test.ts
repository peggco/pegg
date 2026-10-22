import { describe, expect, it } from 'vitest';
import {
  addNotice,
  appendUser,
  applyUpdate,
  initialState,
  truncateFrom,
  userMessageOrdinal,
} from '../../src/chat/state';

describe('message history helpers', () => {
  it('counts user messages only', () => {
    let state = initialState();
    state = appendUser(state, 'one');
    state = addNotice(state, 'a notice');
    state = appendUser(state, 'two');
    state = addNotice(state, 'another notice');
    state = appendUser(state, 'three');

    const ids = state.items.filter((item) => item.kind === 'user').map((item) => item.id);
    expect(userMessageOrdinal(state, ids[0])).toBe(0);
    expect(userMessageOrdinal(state, ids[1])).toBe(1);
    expect(userMessageOrdinal(state, ids[2])).toBe(2);
  });

  it('returns undefined for unknown items', () => {
    const state = appendUser(initialState(), 'one');
    expect(userMessageOrdinal(state, 'missing')).toBeUndefined();
  });

  it('truncates the target and everything after it', () => {
    let state = initialState();
    state = appendUser(state, 'one');
    state = appendUser(state, 'two');
    state = addNotice(state, 'after');
    const second = state.items[1];

    const next = truncateFrom(state, second.id);
    expect(next.items).toHaveLength(1);
    expect(next.items[0].kind).toBe('user');
    expect(state.items).toHaveLength(3);
  });

  it('ignores an unknown item id', () => {
    const state = appendUser(initialState(), 'one');
    expect(truncateFrom(state, 'missing')).toBe(state);
  });

  it('keeps the stored message id of a replayed user message', () => {
    const state = applyUpdate(initialState(), {
      sessionUpdate: 'user_message_chunk',
      messageId: 'stored-1',
      content: { type: 'text', text: 'replayed' },
    });
    const item = state.items[0];
    expect(item.kind).toBe('user');
    if (item.kind === 'user') {
      expect(item.messageId).toBe('stored-1');
    }
  });
});
