import { act, cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

const {
  settingsSnapshot,
  providerCatalog,
  providerDrivers,
  peggLogin,
  peggCancel,
  addProvider,
  addCustomProvider,
  listModels,
  providerSetup,
  openExternal,
  onPeggStatus,
  pluginSettingsPages,
  pluginSettingsAction,
  saveGeneralSettings,
  saveLanguage,
} = vi.hoisted(() => ({
  settingsSnapshot: vi.fn(),
  providerCatalog: vi.fn(),
  providerDrivers: vi.fn(),
  peggLogin: vi.fn(),
  peggCancel: vi.fn(),
  addProvider: vi.fn(),
  addCustomProvider: vi.fn(),
  listModels: vi.fn(),
  providerSetup: vi.fn(),
  openExternal: vi.fn(),
  onPeggStatus: vi.fn(),
  pluginSettingsPages: vi.fn(),
  pluginSettingsAction: vi.fn(),
  saveGeneralSettings: vi.fn(),
  saveLanguage: vi.fn(),
}));

vi.mock('../lib/bridge', () => ({
  api: {
    SettingsSnapshot: settingsSnapshot,
    ProviderCatalog: providerCatalog,
    ProviderDrivers: providerDrivers,
    PeggLogin: peggLogin,
    PeggCancel: peggCancel,
    AddProvider: addProvider,
    AddCustomProvider: addCustomProvider,
    ListModels: listModels,
    ProviderSetup: providerSetup,
    OpenExternal: openExternal,
    SaveGeneralSettings: saveGeneralSettings,
    SaveLanguage: saveLanguage,
    RemoveProvider: vi.fn(async () => undefined),
    PluginSettingsPages: pluginSettingsPages,
    PluginSettingsRefresh: pluginSettingsPages,
    PluginSettingsAction: pluginSettingsAction,
  },
  onPeggStatus,
  hasWails: false,
}));

import { Settings } from './Settings';
import { LANGUAGE_STORAGE } from '../lib/i18n';
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
      languages: [
        { code: 'en', label: 'English' },
        { code: 'tr', label: 'Türkçe' },
      ],
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
  addCustomProvider.mockResolvedValue(undefined);
  providerDrivers.mockResolvedValue(['claude', 'gemini', 'openai']);
  providerSetup.mockResolvedValue({ configured: true, dismissed: false });
  listModels.mockResolvedValue({ models: [] });
  openExternal.mockResolvedValue(undefined);
  pluginSettingsPages.mockResolvedValue([]);
  pluginSettingsAction.mockResolvedValue({ output: 'ok' });
  saveGeneralSettings.mockResolvedValue(undefined);
  saveLanguage.mockResolvedValue(undefined);
  window.localStorage.clear();
  document.documentElement.lang = 'en';
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
    expect(within(dialog).queryByText('Base URL')).toBeNull();
    const key = screen.getByPlaceholderText('sk-…') as HTMLInputElement;
    fireEvent.change(key, { target: { value: 'vsk_live_test' } });
    fireEvent.click(within(dialog).getByRole('button', { name: 'Add provider' }));

    await waitFor(() => expect(addProvider).toHaveBeenCalledWith('pegg-forever', 'vsk_live_test', ''));
  });

  it('adds a custom endpoint with name, driver, base URL and API key', async () => {
    render(<Settings />);

    await waitFor(() => expect(screen.getByRole('button', { name: /Add provider/ })).toBeTruthy());
    fireEvent.click(screen.getByRole('button', { name: /Add provider/ }));
    await waitFor(() => expect(providerCatalog).toHaveBeenCalled());

    const dialog = screen.getByRole('dialog', { name: 'Add provider' });
    const provider = within(dialog).getByRole('combobox') as HTMLSelectElement;
    fireEvent.change(provider, { target: { value: '__custom__' } });

    await waitFor(() => {
      const driver = within(dialog).getAllByRole('combobox')[1] as HTMLSelectElement;
      expect(driver.value).toBe('openai');
    });

    fireEvent.change(within(dialog).getByPlaceholderText('my-gateway'), {
      target: { value: 'my-gateway' },
    });
    fireEvent.change(within(dialog).getByPlaceholderText('https://api.example.com/v1'), {
      target: { value: 'https://gateway.example.com/v1' },
    });
    fireEvent.change(within(dialog).getByPlaceholderText('sk-… (optional)'), {
      target: { value: 'sk-custom' },
    });
    fireEvent.click(within(dialog).getByRole('button', { name: 'Add provider' }));

    await waitFor(() =>
      expect(addCustomProvider).toHaveBeenCalledWith(
        'my-gateway',
        'openai',
        'sk-custom',
        'https://gateway.example.com/v1',
      ),
    );
  });
});

describe('Settings language', () => {
  it('applies and persists a selected language immediately', async () => {
    render(<Settings />);

    const select = await screen.findByRole('combobox', { name: 'Language' });
    fireEvent.change(select, { target: { value: 'tr' } });

    expect(store.get().language).toBe('tr');
    expect(document.documentElement.lang).toBe('tr');
    expect(window.localStorage.getItem(LANGUAGE_STORAGE)).toBe('tr');
    await waitFor(() => expect(saveLanguage).toHaveBeenCalledWith('tr'));

    const save = await screen.findByRole('button', { name: 'Değişiklikleri kaydet' });
    expect(screen.getByText('Dil')).toBeTruthy();

    fireEvent.click(save);
    await waitFor(() =>
      expect(saveGeneralSettings).toHaveBeenCalledWith(
        expect.objectContaining({ language: 'tr' }),
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
