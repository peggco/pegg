import { beforeEach, describe, expect, it, vi } from 'vitest';

const { loadHistory, searchCatalog, installCatalogMCP, installCatalogPlugin } = vi.hoisted(() => ({
  loadHistory: vi.fn(),
  searchCatalog: vi.fn(),
  installCatalogMCP: vi.fn(),
  installCatalogPlugin: vi.fn(),
}));

vi.mock('../lib/bridge', () => ({
  api: {
    LoadHistory: loadHistory,
    SearchCatalog: searchCatalog,
    InstallCatalogMCP: installCatalogMCP,
    InstallCatalogPlugin: installCatalogPlugin,
  },
}));

import {
  handleUpdates,
  installCatalogMCP as installMCP,
  installCatalogPlugin as installPlugin,
  loadOlderMessages,
  searchCatalog as runSearch,
} from './actions';
import { MAX_CHAT_ITEMS, emptyChat, type ChatItem, type ChatState } from './chat';
import { store } from './store';
import type { SessionUpdate } from '../types';

function seed(overrides: Partial<ChatState> = {}): void {
  store.set({
    chat: {
      ...emptyChat(),
      acpId: 'acp-1',
      peggId: 'pegg-1',
      historyHasMore: true,
      historyOldestSeq: 10,
      ...overrides,
    },
  });
}

function deferred<T>(): { promise: Promise<T>; resolve: (value: T) => void } {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((done) => {
    resolve = done;
  });
  return { promise, resolve };
}

const historyPage = (text: string, seq: number): SessionUpdate => ({
  sessionUpdate: 'user_message_chunk',
  content: { type: 'text', text },
  _meta: { 'com.pegg/historyPage': true, 'com.pegg/seq': seq },
});

beforeEach(() => {
  vi.clearAllMocks();
  seed();
});

describe('loadOlderMessages', () => {
  it('keeps loading until the history page notifications arrive', async () => {
    loadHistory.mockResolvedValue({ oldestSeq: 5, hasMore: true });

    await loadOlderMessages();

    expect(loadHistory).toHaveBeenCalledWith(
      expect.objectContaining({ sessionId: 'acp-1', beforeSeq: 10, limit: 200 }),
    );
    expect(store.get().chat.historyLoading).toBe(true);

    handleUpdates('acp-1', [historyPage('older', 4)]);

    const chat = store.get().chat;
    expect(chat.historyLoading).toBe(false);
    expect(chat.historyOldestSeq).toBe(5);
    expect(chat.items).toHaveLength(1);
    expect(chat.items[0]).toMatchObject({ kind: 'user', text: 'older', seq: 4 });
  });

  it('clears loading immediately when there are no more pages', async () => {
    loadHistory.mockResolvedValue({ oldestSeq: 0, hasMore: false });

    await loadOlderMessages();

    expect(store.get().chat.historyLoading).toBe(false);
    expect(store.get().chat.historyHasMore).toBe(false);
  });

  it('ignores a response from a superseded request', async () => {
    const pending = deferred<{ oldestSeq: number; hasMore: boolean }>();
    loadHistory.mockReturnValue(pending.promise);

    const request = loadOlderMessages();
    expect(store.get().chat.historyRequestId).toBe(1);

    store.set({ chat: { ...store.get().chat, historyRequestId: 99 } });
    pending.resolve({ oldestSeq: 1, hasMore: true });
    await request;

    expect(store.get().chat.historyOldestSeq).toBe(10);
    expect(store.get().chat.historyRequestId).toBe(99);
  });

  it('does nothing when no older pages are available', async () => {
    seed({ historyHasMore: false });
    await loadOlderMessages();
    expect(loadHistory).not.toHaveBeenCalled();
  });
});

describe('handleUpdates', () => {
  it('prunes live-only updates while pinned to the bottom', () => {
    const items: ChatItem[] = Array.from({ length: MAX_CHAT_ITEMS + 5 }, (_, index) => ({
      kind: 'notice' as const,
      id: `notice-${index}`,
      text: 'note',
      level: 'info' as const,
      createdAt: index,
      seq: index + 1,
    }));
    seed({ items, historyOldestSeq: 1, stickToBottom: true });

    handleUpdates('acp-1', [
      { sessionUpdate: 'agent_message_chunk', content: { type: 'text', text: 'new' } },
    ]);

    const chat = store.get().chat;
    expect(chat.items.length).toBeLessThanOrEqual(MAX_CHAT_ITEMS);
    expect(chat.historyHasMore).toBe(true);
  });

  it('applies command updates before the session id is known', () => {
    seed({ acpId: '', peggId: '', commands: [] });

    handleUpdates('unknown', [
      {
        sessionUpdate: 'available_commands_update',
        availableCommands: [{ name: 'plugin-skill', description: 'from plugin' }],
      },
    ]);

    expect(store.get().chat.commands).toEqual([{ name: 'plugin-skill', description: 'from plugin' }]);
  });

  it('ignores other updates before the session id is known', () => {
    seed({ acpId: '', peggId: '', items: [] });

    handleUpdates('unknown', [
      { sessionUpdate: 'agent_message_chunk', content: { type: 'text', text: 'hello' } },
    ]);

    expect(store.get().chat.items).toEqual([]);
  });
});

describe('catalog actions', () => {
  it('delegates search parameters to the bridge', async () => {
    searchCatalog.mockResolvedValue({ items: [], nextCursor: 2 });

    const result = await runSearch('mcp', 'git', 'official,stdio', 10, 0);

    expect(searchCatalog).toHaveBeenCalledWith('mcp', 'git', 'official,stdio', 10, 0);
    expect(result.nextCursor).toBe(2);
  });

  it('toasts a restart notice when MCP install requests one', async () => {
    installCatalogMCP.mockResolvedValue({ name: 'github', restartRequired: true });

    const result = await installMCP('github', { GITHUB_TOKEN: 'x' }, {}, 'global');

    expect(installCatalogMCP).toHaveBeenCalledWith('github', { GITHUB_TOKEN: 'x' }, {}, 'global');
    expect(result.restartRequired).toBe(true);
    expect(store.get().toasts.some((toast) => toast.kind === 'warn')).toBe(true);
  });

  it('installs plugins and applies returned commands', async () => {
    installCatalogPlugin.mockResolvedValue({
      name: 'browser-use',
      commands: [{ name: 'browse', description: 'Open a browser' }],
    });

    await installPlugin('peggco/pegg-browser-use', 'browser-use');

    expect(installCatalogPlugin).toHaveBeenCalledWith('peggco/pegg-browser-use', 'browser-use');
    expect(store.get().chat.commands).toContainEqual({ name: 'browse', description: 'Open a browser' });
  });
});
