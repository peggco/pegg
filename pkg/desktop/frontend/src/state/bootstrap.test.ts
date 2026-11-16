import { beforeEach, describe, expect, it, vi } from 'vitest';

const { bootstrapInfo, applyWindowTheme, selfUpdateStatus } = vi.hoisted(() => ({
  bootstrapInfo: vi.fn(),
  applyWindowTheme: vi.fn(async () => undefined),
  selfUpdateStatus: vi.fn(async () => ({})),
}));

vi.mock('../lib/bridge', () => ({
  api: {
    Bootstrap: bootstrapInfo,
    ApplyWindowTheme: applyWindowTheme,
    SelfUpdateStatus: selfUpdateStatus,
  },
}));

import { bootstrap } from './actions';
import { store } from './store';
import { LANGUAGE_STORAGE } from '../lib/i18n';
import type { BootstrapInfo } from '../types';

function bootInfo(overrides: Partial<BootstrapInfo> = {}): BootstrapInfo {
  return {
    platform: 'linux',
    arch: 'amd64',
    version: 'test',
    home: '',
    prefs: {
      theme: 'dark',
      themeMode: 'auto',
      workspace: '',
      recents: [],
      binaryPath: '',
      autoDownload: true,
      sendOnEnter: true,
      fontScale: 1,
      sidebarCollapsed: false,
      sidebarWidth: 280,
      sidebarMode: 'sidebar',
      layoutMode: 'default',
      direction: 'ltr',
      chatHistoryMode: 'compact',
      lastSession: '',
      window: {},
    },
    themes: [],
    binary: { found: false },
    selfUpdate: {
      currentVersion: '',
      state: 'idle',
      available: false,
      supported: false,
      desktopAvailable: false,
      desktopReady: false,
      ready: false,
      progress: 0,
      agentAvailable: false,
      agentSupported: false,
      agentReady: false,
    },
    defaultWorkspace: '',
    workspace: '',
    logPath: '',
    language: '',
    nativeTitlebar: false,
    ...overrides,
  };
}

beforeEach(() => {
  vi.clearAllMocks();
  window.localStorage.clear();
  document.documentElement.lang = 'en';
  store.set({ language: 'en' });
});

describe('bootstrap language', () => {
  it('uses the cached language when the config has none', async () => {
    window.localStorage.setItem(LANGUAGE_STORAGE, 'tr');
    bootstrapInfo.mockResolvedValue(bootInfo({ language: '' }));

    await bootstrap();

    expect(store.get().language).toBe('tr');
    expect(document.documentElement.lang).toBe('tr');
    expect(window.localStorage.getItem(LANGUAGE_STORAGE)).toBe('tr');
  });

  it('prefers the config language and reconciles the cache', async () => {
    window.localStorage.setItem(LANGUAGE_STORAGE, 'tr');
    bootstrapInfo.mockResolvedValue(bootInfo({ language: 'en' }));

    await bootstrap();

    expect(store.get().language).toBe('en');
    expect(document.documentElement.lang).toBe('en');
    expect(window.localStorage.getItem(LANGUAGE_STORAGE)).toBe('en');
  });

  it('defaults to English when neither source has a language', async () => {
    bootstrapInfo.mockResolvedValue(bootInfo({ language: '' }));

    await bootstrap();

    expect(store.get().language).toBe('en');
    expect(window.localStorage.getItem(LANGUAGE_STORAGE)).toBe('en');
  });
});
