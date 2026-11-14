import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

const {
  peggLogin,
  peggCancel,
  addProvider,
  dismissProviderSetup,
  providerSetup,
  listModels,
  openExternal,
  onPeggStatus,
} = vi.hoisted(() => ({
  peggLogin: vi.fn(),
  peggCancel: vi.fn(),
  addProvider: vi.fn(),
  dismissProviderSetup: vi.fn(),
  providerSetup: vi.fn(),
  listModels: vi.fn(),
  openExternal: vi.fn(),
  onPeggStatus: vi.fn(),
}));

vi.mock('../lib/bridge', () => ({
  api: {
    PeggLogin: peggLogin,
    PeggCancel: peggCancel,
    AddProvider: addProvider,
    DismissProviderSetup: dismissProviderSetup,
    ProviderSetup: providerSetup,
    ListModels: listModels,
    OpenExternal: openExternal,
  },
  onPeggStatus,
  hasWails: false,
}));

import { ProviderOnboarding } from './ProviderOnboarding';
import { store } from '../state/store';
import type { ForeverStatus } from '../types';

let statusListener: ((status: ForeverStatus) => void) | undefined;

function forever(overrides: Partial<ForeverStatus> = {}): ForeverStatus {
  return {
    state: 'starting',
    logs: [],
    configured: false,
    provider: 'pegg-forever',
    ...overrides,
  };
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
  peggLogin.mockResolvedValue(forever());
  peggCancel.mockResolvedValue(forever({ state: 'idle' }));
  addProvider.mockResolvedValue(undefined);
  dismissProviderSetup.mockResolvedValue(undefined);
  providerSetup.mockResolvedValue({ configured: true, dismissed: false });
  listModels.mockResolvedValue({ models: [] });
  openExternal.mockResolvedValue(undefined);
  store.set({
    language: 'en',
    settingsOpen: false,
    settingsTab: 'general',
    providerSetup: { configured: false, dismissed: false },
    models: [],
    modelsLoading: false,
  });
});

afterEach(() => {
  cleanup();
});

describe('ProviderOnboarding', () => {
  it('offers exactly the three setup choices', () => {
    render(<ProviderOnboarding />);

    expect(screen.getByRole('button', { name: /Login to Pegg/ })).toBeTruthy();
    expect(screen.getByRole('button', { name: /Use API key/ })).toBeTruthy();
    expect(screen.getByRole('button', { name: /configure myself/ })).toBeTruthy();
  });

  it('shows login progress, the approval URL, and completes on done', async () => {
    render(<ProviderOnboarding />);

    fireEvent.click(screen.getByRole('button', { name: /Login to Pegg/ }));
    await waitFor(() => expect(peggLogin).toHaveBeenCalled());
    await waitFor(() => expect(statusListener).toBeDefined());

    act(() => statusListener?.(forever({ state: 'waiting', url: 'https://pegg.dev/approve' })));
    expect(screen.getAllByText('Waiting for browser approval…').length).toBeGreaterThan(0);
    expect(screen.getByText('https://pegg.dev/approve')).toBeTruthy();

    act(() => statusListener?.(forever({ state: 'done', configured: true })));

    await waitFor(() => expect(providerSetup).toHaveBeenCalled());
    await waitFor(() => expect(listModels).toHaveBeenCalled());
    expect(store.get().providerSetup).toEqual({ configured: true, dismissed: false });
  });

  it('cancels a browser login and returns to the choices', async () => {
    render(<ProviderOnboarding />);

    fireEvent.click(screen.getByRole('button', { name: /Login to Pegg/ }));
    await waitFor(() => expect(peggLogin).toHaveBeenCalled());
    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }));

    await waitFor(() => expect(peggCancel).toHaveBeenCalled());
    expect(screen.getByRole('button', { name: /Use API key/ })).toBeTruthy();
  });

  it('saves a Pegg Forever API key and clears onboarding', async () => {
    render(<ProviderOnboarding />);

    fireEvent.click(screen.getByRole('button', { name: /Use API key/ }));
    fireEvent.change(screen.getByLabelText('API key'), { target: { value: 'vsk_live_test' } });
    fireEvent.click(screen.getByRole('button', { name: /Save key/ }));

    await waitFor(() =>
      expect(addProvider).toHaveBeenCalledWith(
        'pegg-forever',
        'vsk_live_test',
        'https://forever.pegg.dev/v1',
      ),
    );
    await waitFor(() => expect(providerSetup).toHaveBeenCalled());
    await waitFor(() => expect(listModels).toHaveBeenCalled());
    expect(store.get().providerSetup?.configured).toBe(true);
  });

  it('dismisses onboarding and opens provider settings', async () => {
    render(<ProviderOnboarding />);

    fireEvent.click(screen.getByRole('button', { name: /configure myself/ }));

    await waitFor(() => expect(dismissProviderSetup).toHaveBeenCalled());
    expect(store.get().providerSetup).toEqual({ configured: false, dismissed: true });
    expect(store.get().settingsOpen).toBe(true);
    expect(store.get().settingsTab).toBe('general');
  });

  it('requires a key before saving', async () => {
    render(<ProviderOnboarding />);

    fireEvent.click(screen.getByRole('button', { name: /Use API key/ }));
    fireEvent.click(screen.getByRole('button', { name: /Save key/ }));

    expect(addProvider).not.toHaveBeenCalled();
    expect(screen.getByText('Something went wrong: Enter an API key.')).toBeTruthy();
  });
});
