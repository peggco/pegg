import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { SelfUpdateStatus } from '../types';
import { store } from '../state/store';
import { UpdateBanner } from './UpdateBanner';

const { downloadDesktopUpdate, openExternal, restartToUpdate } = vi.hoisted(() => ({
  downloadDesktopUpdate: vi.fn(),
  openExternal: vi.fn(),
  restartToUpdate: vi.fn(),
}));

vi.mock('../state/actions', () => ({ downloadDesktopUpdate, restartToUpdate }));

vi.mock('../lib/bridge', () => ({ api: { OpenExternal: openExternal } }));

function status(overrides: Partial<SelfUpdateStatus> = {}): SelfUpdateStatus {
  return {
    currentVersion: '0.2.4',
    state: 'idle',
    available: false,
    supported: true,
    desktopAvailable: false,
    desktopReady: false,
    ready: false,
    progress: 0,
    agentAvailable: false,
    agentSupported: false,
    agentReady: false,
    ...overrides,
  };
}

describe('UpdateBanner', () => {
  beforeEach(() => {
    downloadDesktopUpdate.mockReset();
    restartToUpdate.mockReset();
    openExternal.mockReset();
    store.set({ selfUpdate: status() });
  });

  afterEach(() => {
    cleanup();
  });

  it('shows the restart action when an update is ready', () => {
    store.set({ selfUpdate: status({ available: true, desktopAvailable: true, desktopReady: true, ready: true, state: 'ready', latestVersion: '0.2.5' }) });
    render(<UpdateBanner />);
    expect(screen.getByText('Pegg Desktop 0.2.5 is ready')).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: 'Restart to update' }));
    expect(restartToUpdate).toHaveBeenCalled();
  });

  it('shows the download page when auto-update is unsupported', () => {
    store.set({
      selfUpdate: status({
        available: true,
        desktopAvailable: true,
        supported: false,
        state: 'unsupported',
        latestVersion: '0.2.5',
        releaseUrl: 'https://example.com',
      }),
    });
    render(<UpdateBanner />);
    fireEvent.click(screen.getByRole('button', { name: 'Open download page' }));
    expect(openExternal).toHaveBeenCalledWith('https://example.com');
  });

  it('shows download progress', () => {
    store.set({ selfUpdate: status({ available: true, desktopAvailable: true, state: 'downloading', latestVersion: '0.2.5', progress: 42 }) });
    render(<UpdateBanner />);
    expect(screen.getByText('Downloading Pegg updates… 42%')).toBeTruthy();
  });

  it('renders nothing without an available update', () => {
    const { container } = render(<UpdateBanner />);
    expect(container.firstChild).toBeNull();
  });
});
