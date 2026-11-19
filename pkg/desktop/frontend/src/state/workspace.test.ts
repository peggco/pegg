import { beforeEach, describe, expect, it, vi } from 'vitest';

const {
  openWorkspace,
  savePrefs,
  loadSessionsPage,
  slashCommands,
  recentsFn,
  removeRecentFn,
  log,
} = vi.hoisted(() => ({
  openWorkspace: vi.fn(),
  savePrefs: vi.fn(),
  loadSessionsPage: vi.fn(),
  slashCommands: vi.fn(async () => []),
  recentsFn: vi.fn(async () => []),
  removeRecentFn: vi.fn(),
  log: vi.fn(async () => undefined),
}));

vi.mock('../lib/bridge', () => ({
  api: {
    OpenWorkspace: openWorkspace,
    SavePrefs: savePrefs,
    LoadSessionsPage: loadSessionsPage,
    SlashCommands: slashCommands,
    Recents: recentsFn,
    RemoveRecent: removeRecentFn,
    Log: log,
  },
}));

import { removeRecent, setWorkspace } from './actions';
import { store, initialState } from './store';
import type { SessionListItem } from '../types';

function session(id: string, projectDir: string): SessionListItem {
  const at = '2026-01-01T00:00:00Z';
  return { id, title: id, createdAt: at, updatedAt: at, projectDir };
}

beforeEach(() => {
  vi.clearAllMocks();
  store.set({ ...initialState });
  savePrefs.mockImplementation(async (p) => p);
  loadSessionsPage.mockResolvedValue({ sessions: [], nextCursor: '' });
  removeRecentFn.mockResolvedValue([]);
});

describe('setWorkspace', () => {
  it('drops the previous project state before the new one loads', async () => {
    store.set({
      workspace: '/old',
      view: 'chat',
      sessions: [session('a', '/old')],
      sessionsNextCursor: 'cursor',
      commandsCatalog: [{ name: 'old', description: '' }],
      subagentView: 'sub-1',
    });
    openWorkspace.mockResolvedValue('/new');

    await setWorkspace('/new');

    const state = store.get();
    expect(state.workspace).toBe('/new');
    expect(state.view).toBe('welcome');
    expect(state.sessions).toEqual([]);
    expect(state.sessionsNextCursor).toBeFalsy();
    expect(state.commandsCatalog).toEqual([]);
    expect(state.subagentView).toBeNull();
    expect(state.chat.peggId).toBeFalsy();
  });

  it('ignores a session page that arrives after another switch', async () => {
    openWorkspace.mockResolvedValue('/new');
    loadSessionsPage.mockImplementation(async () => {
      store.set({ workspace: '/newer' });
      return { sessions: [session('a', '/new')], nextCursor: '' };
    });

    await setWorkspace('/new');

    expect(store.get().sessions).toEqual([]);
  });
});

describe('removeRecent', () => {
  it('forgets the project and keeps the open one when it is another', async () => {
    store.set({ workspace: '/keep', sessions: [session('a', '/keep')] });
    removeRecentFn.mockResolvedValue([{ path: '/keep', name: 'keep', openedAt: '' }]);

    await removeRecent('/gone');

    expect(removeRecentFn).toHaveBeenCalledWith('/gone');
    expect(store.get().workspace).toBe('/keep');
    expect(store.get().recents).toHaveLength(1);
    expect(store.get().sessions).toHaveLength(1);
  });

  it('clears the open project when it is the one removed', async () => {
    store.set({
      workspace: '/gone',
      view: 'chat',
      sessions: [session('a', '/gone')],
      prefs: { ...initialState.prefs, workspace: '/gone' },
    });

    await removeRecent('/gone');

    const state = store.get();
    expect(state.workspace).toBe('');
    expect(state.prefs.workspace).toBe('');
    expect(state.view).toBe('welcome');
    expect(state.sessions).toEqual([]);
  });
});
