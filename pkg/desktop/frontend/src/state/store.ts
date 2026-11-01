import { useSyncExternalStore } from 'react';
import type {
  AcpStatus,
  BinaryProgress,
  BinaryStatus,
  BootstrapInfo,
  ModelEntry,
  Prefs,
  Recent,
  SessionListItem,
  Theme,
} from '../types';
import { emptyChat, type ChatState } from './chat';

export interface Toast {
  id: string;
  kind: 'info' | 'warn' | 'error' | 'success';
  text: string;
}

export interface UIState {
  ready: boolean;
  boot?: BootstrapInfo;
  themes: Theme[];
  theme?: Theme;
  prefs: Prefs;
  chat: ChatState;
  commandsCatalog: import('../types').SlashCommand[];
  sessions: SessionListItem[];
  sessionsLoading: boolean;
  sessionsNextCursor?: string;
  models: ModelEntry[];
  modelsLoading: boolean;
  workspace: string;
  recents: Recent[];
  status: AcpStatus;
  binary: BinaryStatus;
  installing: boolean;
  installProgress?: BinaryProgress;
  loadingSession?: string;
  toasts: Toast[];
  language: string;
  view: 'welcome' | 'chat';
  subagentView: string | null;
  sidebarCollapsed: boolean;
  settingsOpen: boolean;
  paletteOpen: boolean;
  modelsOpen: boolean;
  themePickerOpen: boolean;
  composerFocus: number;
  error?: string;
}

const initialPrefs: Prefs = {
  theme: 'dark',
  themeMode: 'auto',
  workspace: '',
  recents: [],
  binaryPath: '',
  autoDownload: true,
  sendOnEnter: true,
  fontScale: 1,
  sidebarCollapsed: false,
  lastSession: '',
  window: {},
};

export const initialState: UIState = {
  ready: false,
  themes: [],
  prefs: initialPrefs,
  chat: emptyChat(),
  commandsCatalog: [],
  sessions: [],
  sessionsLoading: false,
  sessionsNextCursor: undefined,
  models: [],
  modelsLoading: false,
  workspace: '',
  recents: [],
  status: { state: 'disconnected' },
  binary: { found: false },
  installing: false,
  toasts: [],
  language: 'en',
  view: 'welcome',
  subagentView: null,
  sidebarCollapsed: false,
  settingsOpen: false,
  paletteOpen: false,
  modelsOpen: false,
  themePickerOpen: false,
  composerFocus: 0,
};

class Store {
  private state: UIState = initialState;
  private listeners = new Set<() => void>();

  snapshot = (): UIState => this.state;

  subscribe = (listener: () => void): (() => void) => {
    this.listeners.add(listener);
    return () => this.listeners.delete(listener);
  };

  set(patch: Partial<UIState> | ((state: UIState) => Partial<UIState>)): void {
    const next = typeof patch === 'function' ? patch(this.state) : patch;
    this.state = { ...this.state, ...next };
    for (const listener of this.listeners) listener();
  }

  get(): UIState {
    return this.state;
  }

  setChat(updater: (chat: ChatState) => ChatState): void {
    const chat = updater(this.state.chat);
    if (chat === this.state.chat) return;
    this.state = { ...this.state, chat };
    for (const listener of this.listeners) listener();
  }

  toast(kind: Toast['kind'], text: string): void {
    const id = `toast-${Date.now()}-${Math.random().toString(36).slice(2, 6)}`;
    this.set((s) => ({ toasts: [...s.toasts, { id, kind, text }] }));
    window.setTimeout(() => {
      this.set((s) => ({ toasts: s.toasts.filter((t) => t.id !== id) }));
    }, 4200);
  }

  dismissToast(id: string): void {
    this.set((s) => ({ toasts: s.toasts.filter((t) => t.id !== id) }));
  }
}

export const store = new Store();

export function useStore<T>(selector: (state: UIState) => T): T {
  return useSyncExternalStore(
    store.subscribe,
    () => selector(store.snapshot()),
    () => selector(initialState),
  );
}

export function useUI(): UIState {
  return useSyncExternalStore(store.subscribe, store.snapshot, () => initialState);
}
