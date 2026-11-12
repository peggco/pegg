import { cleanup, fireEvent, render, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { CatalogModal } from './CatalogModal';
import * as actions from '../state/actions';
import { store } from '../state/store';
import type { BootstrapInfo, CatalogEntry, CatalogItem } from '../types';

vi.mock('../lib/bridge', () => ({
  api: { OpenExternal: vi.fn(async () => undefined) },
  hasWails: false,
}));

vi.mock('../state/actions', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../state/actions')>();
  return {
    ...actual,
    searchCatalog: vi.fn(async () => ({ items: [], nextCursor: null })),
    fetchCatalogEntry: vi.fn(async () => {
      throw new Error('missing entry');
    }),
    fetchCatalogReadme: vi.fn(async () => ''),
    fetchCatalogInstalled: vi.fn(async () => ({ skills: [], mcp: [], plugins: [] })),
    installHubSkill: vi.fn(async () => ({})),
    installCatalogMCP: vi.fn(async () => ({})),
    installCatalogPlugin: vi.fn(async () => ({})),
  };
});

const search = vi.mocked(actions.searchCatalog);
const fetchEntry = vi.mocked(actions.fetchCatalogEntry);
const fetchReadme = vi.mocked(actions.fetchCatalogReadme);
const installMCP = vi.mocked(actions.installCatalogMCP);
const installSkill = vi.mocked(actions.installHubSkill);

const SKILL: CatalogItem = {
  kind: 'skills',
  id: 'remotion',
  name: 'Remotion',
  description: 'Make videos with React.',
  source: 'pegg/skills',
  url: '',
  official: true,
  skillId: 'remotion',
};

const MCP: CatalogItem = {
  kind: 'mcp',
  id: 'github',
  name: 'GitHub',
  description: 'Issues and pull requests.',
  source: 'github',
  url: '',
  official: true,
  transport: 'stdio',
};

beforeEach(() => {
  vi.clearAllMocks();
  search.mockResolvedValue({ items: [], nextCursor: null });
  fetchEntry.mockRejectedValue(new Error('missing entry'));
  fetchReadme.mockResolvedValue('');
  installMCP.mockResolvedValue({});
  installSkill.mockResolvedValue({});
  store.set({ language: 'en', catalogOpen: true, catalogTab: 'skills' });
});

afterEach(() => {
  cleanup();
});

describe('CatalogModal', () => {
  it('searches the active tab and switches kinds', async () => {
    const { getByRole } = render(<CatalogModal />);

    await waitFor(() => expect(search).toHaveBeenCalledWith('skills', '', '', 100, 0));

    fireEvent.click(getByRole('tab', { name: 'MCP' }));

    await waitFor(() => expect(store.get().catalogTab).toBe('mcp'));
    await waitFor(() => expect(search).toHaveBeenLastCalledWith('mcp', '', '', 100, 0));
  });

  it('debounces the search query and sends the official filter', async () => {
    const { container, getByText } = render(<CatalogModal />);
    const input = container.querySelector('.hub-search input') as HTMLInputElement;

    fireEvent.change(input, { target: { value: 'git' } });
    await waitFor(
      () => expect(search).toHaveBeenLastCalledWith('skills', 'git', '', 100, 0),
      { timeout: 1500 },
    );

    fireEvent.click(getByText('Official'));
    await waitFor(() => expect(search).toHaveBeenLastCalledWith('skills', 'git', 'official', 100, 0));
  });

  it('auto-filters plugins by the current platform and keeps the official split', async () => {
    store.set({
      catalogTab: 'plugins',
      boot: { platform: 'darwin' } as BootstrapInfo,
    });
    const { getByText, queryByText } = render(<CatalogModal />);

    await waitFor(() => expect(search).toHaveBeenLastCalledWith('plugins', '', 'darwin', 100, 0));
    expect(queryByText('linux')).toBeNull();
    expect(queryByText('darwin')).toBeNull();

    fireEvent.click(getByText('Official'));
    await waitFor(() => expect(search).toHaveBeenLastCalledWith('plugins', '', 'official,darwin', 100, 0));
  });

  it('opens a detail view and shows the install command', async () => {
    search.mockResolvedValue({ items: [SKILL], nextCursor: null });
    fetchEntry.mockResolvedValue({ ...SKILL, readmeSource: 'pegg/skills' } as CatalogEntry);
    fetchReadme.mockResolvedValue('# Remotion');
    const { container, getByText } = render(<CatalogModal />);

    await waitFor(() => expect(getByText('Remotion')).not.toBeNull());
    fireEvent.click(getByText('Remotion'));

    await waitFor(() => expect(fetchEntry).toHaveBeenCalledWith('skills', 'remotion'));
    expect(container.textContent).toContain('pegg skills add pegg/skills/remotion');
    await waitFor(() => expect(fetchReadme).toHaveBeenCalledWith('skills', 'remotion'));
  });

  it('prompts for MCP environment variables before installing', async () => {
    store.set({ catalogTab: 'mcp' });
    search.mockResolvedValue({ items: [MCP], nextCursor: null });
    fetchEntry.mockResolvedValue({ ...MCP, requiredEnv: ['GITHUB_TOKEN'] });
    const { container, getByText } = render(<CatalogModal />);

    await waitFor(() => expect(getByText('GitHub')).not.toBeNull());
    fireEvent.click(getByText('GitHub'));

    await waitFor(() => expect(fetchEntry).toHaveBeenCalledWith('mcp', 'github'));
    await waitFor(() => expect(container.textContent).toContain('GITHUB_TOKEN'));

    fireEvent.click(container.querySelector('.hub-install-bar .btn.primary') as HTMLElement);
    await waitFor(() => expect(container.querySelector('input[name="GITHUB_TOKEN"]')).not.toBeNull());

    fireEvent.change(container.querySelector('input[name="GITHUB_TOKEN"]') as HTMLElement, {
      target: { value: 'ghp_secret' },
    });
    fireEvent.click(container.querySelector('.modal.provider-modal .btn.primary') as HTMLElement);

    await waitFor(() =>
      expect(installMCP).toHaveBeenCalledWith('github', { GITHUB_TOKEN: 'ghp_secret' }, {}, 'global'),
    );
  });
});
