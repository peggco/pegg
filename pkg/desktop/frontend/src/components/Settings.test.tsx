import { act, cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

const {
  settingsSnapshot,
  providerCatalog,
  peggLogin,
  peggCancel,
  addProvider,
  listModels,
  providerSetup,
  openExternal,
  onPeggStatus,
  pluginSettingsPages,
  pluginSettingsAction,
} = vi.hoisted(() => ({
  settingsSnapshot: vi.fn(),
  providerCatalog: vi.fn(),
  peggLogin: vi.fn(),
  peggCancel: vi.fn(),
  addProvider: vi.fn(),
  listModels: vi.fn(),
  providerSetup: vi.fn(),
  openExternal: vi.fn(),
  onPeggStatus: vi.fn(),
  pluginSettingsPages: vi.fn(),
  pluginSettingsAction: vi.fn(),
}));

vi.mock('../lib/bridge', () => ({
  api: {
    SettingsSnapshot: settingsSnapshot,
    ProviderCatalog: providerCatalog,
    PeggLogin: peggLogin,
    PeggCancel: peggCancel,
    AddProvider: addProvider,
    ListModels: listModels,
    ProviderSetup: providerSetup,
    OpenExternal: openExternal,
    SaveGeneralSettings: vi.fn(async () => undefined),
    RemoveProvider: vi.fn(async () => undefined),
    PluginSettingsPages: pluginSettingsPages,
    PluginSettingsRefresh: pluginSettingsPages,
    PluginSettingsAction: pluginSettingsAction,
  },
  onPeggStatus,
  hasWails: false,
}));

import { Settings } from './Settings';
import { store } from '../state/store';
import type { ForeverStatus, SettingsSnapshot } from '../types';

let statusListener: ((status: ForeverStatus) => void) | undefined;

function snapshot(): SettingsSnapshot {
  return {
    general: {
      provider: '',
      model: '',
      reasoningEffort: '',
      language: 'en',
      languages: [{ code: 'en', label: 'English' }],
      providers: [],
      stt: {
        providers: [],
        configured: [],
        provider: '',
        model: '',
        models: [],
        modelsByProvider: {},
        entries: [],
        recordCommand: '',
        maxDuration: 60,
      },
    },
  } as unknown as SettingsSnapshot;
}

beforeEach(() => {
  vi.clearAllMocks();
  statusListener = undefined;
  onPeggStatus.mockImplementation((cb: (status: ForeverStatus) => void) => {
    statusListener = cb;
    return () => {
      statusListener = undefined;
    };
  });
  settingsSnapshot.mockResolvedValue(snapshot());
  providerCatalog.mockResolvedValue([
    { name: 'pegg-forever', configured: false, kind: 'api_key', hint: 'API key' },
  ]);
  peggLogin.mockResolvedValue({ state: 'starting', logs: [], configured: false, provider: 'pegg-forever' });
  peggCancel.mockResolvedValue({ state: 'idle', logs: [], configured: false, provider: 'pegg-forever' });
  addProvider.mockResolvedValue(undefined);
  providerSetup.mockResolvedValue({ configured: true, dismissed: false });
  listModels.mockResolvedValue({ models: [] });
  openExternal.mockResolvedValue(undefined);
  pluginSettingsPages.mockResolvedValue([]);
  pluginSettingsAction.mockResolvedValue({ output: 'ok' });
  store.set({
    language: 'en',
    settingsOpen: true,
    settingsTab: 'general',
    providerSetup: { configured: false, dismissed: false },
    models: [],
    modelsLoading: false,
  });
});

afterEach(() => {
  cleanup();
});

describe('Settings provider onboarding', () => {
  it('shows Login to Pegg for pegg-forever and refreshes after login', async () => {
    render(<Settings />);

    await waitFor(() => expect(screen.getByRole('button', { name: /Add provider/ })).toBeTruthy());
    fireEvent.click(screen.getByRole('button', { name: /Add provider/ }));

    await waitFor(() => expect(providerCatalog).toHaveBeenCalled());

    const login = await screen.findByRole('button', { name: /Login to Pegg/ });
    fireEvent.click(login);

    await waitFor(() => expect(peggLogin).toHaveBeenCalled());
    await waitFor(() => expect(statusListener).toBeDefined());

    act(() =>
      statusListener?.({
        state: 'done',
        logs: [],
        configured: true,
        provider: 'pegg-forever',
      }),
    );

    await waitFor(() => expect(listModels).toHaveBeenCalled());
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Add provider' })).toBeNull());
  });

  it('saves a Pegg Forever API key through AddProvider', async () => {
    render(<Settings />);

    await waitFor(() => expect(screen.getByRole('button', { name: /Add provider/ })).toBeTruthy());
    fireEvent.click(screen.getByRole('button', { name: /Add provider/ }));
    await waitFor(() => expect(providerCatalog).toHaveBeenCalled());

    const dialog = screen.getByRole('dialog', { name: 'Add provider' });
    const key = screen.getByPlaceholderText('sk-…') as HTMLInputElement;
    fireEvent.change(key, { target: { value: 'vsk_live_test' } });
    fireEvent.click(within(dialog).getByRole('button', { name: 'Add provider' }));

    await waitFor(() =>
      expect(addProvider).toHaveBeenCalledWith(
        'pegg-forever',
        'vsk_live_test',
        'https://forever.pegg.dev/v1',
      ),
    );
  });
});

describe('Settings plugin pages', () => {
  it('lists plugin pages in the sidebar and renders the selected page', async () => {
    pluginSettingsPages.mockResolvedValue([
      {
        id: 'notify',
        title: 'Notify',
        description: 'Configure channels.',
        owner: 'notify',
        sections: [
          {
            title: 'Channels',
            items: [{ title: 'slack', detail: 'slack · hooks.slack.com', badge: 'ready' }],
          },
        ],
        actions: [{ id: 'add', label: 'Add channel' }],
      },
    ]);

    render(<Settings />);

    const nav = await screen.findByRole('button', { name: /Notify/ });
    fireEvent.click(nav);

    await waitFor(() => expect(screen.getByText('Configure channels.')).toBeTruthy());
    expect(screen.getByText('slack')).toBeTruthy();
    expect(screen.getByRole('button', { name: /Add channel/ })).toBeTruthy();
  });
});
