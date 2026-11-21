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

import { loadMoreSessions, loadSessions, removeRecent, setWorkspace } from './actions';
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

describe('session pages', () => {
  it('does not list anything without an open project', async () => {
    store.set({ workspace: '', sessions: [session('a', '/old')] });

    await loadSessions();

    expect(loadSessionsPage).not.toHaveBeenCalled();
    expect(store.get().sessions).toEqual([]);
    expect(store.get().sessionsLoading).toBe(false);
  });

  it('loads the first page for the open project only', async () => {
    store.set({ workspace: '/p' });
    loadSessionsPage.mockResolvedValue({
      sessions: [session('a', '/p')],
      nextCursor: 'c1',
    });

    await loadSessions();

    expect(loadSessionsPage).toHaveBeenCalledWith('/p', false, '');
    expect(store.get().sessions.map((item) => item.id)).toEqual(['a']);
    expect(store.get().sessionsNextCursor).toBe('c1');
  });

  it('appends the next page for the same project', async () => {
    store.set({
      workspace: '/p',
      sessions: [session('a', '/p')],
      sessionsNextCursor: 'c1',
    });
    loadSessionsPage.mockResolvedValue({
      sessions: [session('a', '/p'), session('b', '/p')],
      nextCursor: 'c2',
    });

    await loadMoreSessions();

    expect(loadSessionsPage).toHaveBeenCalledWith('/p', false, 'c1');
    expect(store.get().sessions.map((item) => item.id)).toEqual(['a', 'b']);
    expect(store.get().sessionsNextCursor).toBe('c2');
  });

  it('drops a page that arrives after another switch', async () => {
    store.set({
      workspace: '/p',
      sessions: [session('a', '/p')],
      sessionsNextCursor: 'c1',
    });
    loadSessionsPage.mockImplementation(async () => {
      store.set({ workspace: '/other' });
      return { sessions: [session('b', '/p')], nextCursor: 'c2' };
    });

    await loadMoreSessions();

    expect(store.get().sessions.map((item) => item.id)).toEqual(['a']);
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
