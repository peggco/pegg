import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

const { pluginSettingsAction } = vi.hoisted(() => ({
  pluginSettingsAction: vi.fn(),
}));

vi.mock('../lib/bridge', () => ({
  api: {
    PluginSettingsAction: pluginSettingsAction,
  },
  hasWails: false,
}));

import { PluginPagePanel } from './PluginPagePanel';
import type { PluginSettingPage } from '../types';

const page: PluginSettingPage = {
  id: 'kanban',
  title: 'Kanban',
  description: 'Board and runner settings.',
  owner: 'kanban',
  settings: [{ id: 'github_repo', label: 'GitHub repo', value: 'acme/demo' }],
  saveAction: 'settings.save',
  sections: [
    {
      title: 'Cards',
      items: [
        {
          id: 'card-1',
          title: 'Fix login',
          detail: 'in_progress · high',
          badge: 'running',
          action: 'card.update',
          fields: [
            { id: 'title', label: 'Title', value: 'Fix login' },
            { id: 'column', label: 'Column', value: 'in_progress' },
          ],
        },
      ],
    },
  ],
  actions: [
    {
      id: 'card.add',
      label: 'Add card',
      description: 'Create a card.',
      fields: [{ id: 'title', label: 'Title', required: true }],
    },
  ],
};

beforeEach(() => {
  vi.clearAllMocks();
  pluginSettingsAction.mockResolvedValue({ output: 'Saved.' });
});

afterEach(() => {
  cleanup();
});

describe('PluginPagePanel', () => {
  it('saves page settings', async () => {
    render(<PluginPagePanel page={page} onRefresh={vi.fn(async () => undefined)} onPage={vi.fn()} />);

    fireEvent.click(screen.getByRole('button', { name: /Settings/ }));
    const dialog = screen.getByRole('dialog');
    const input = within(dialog).getByDisplayValue('acme/demo');
    fireEvent.change(input, { target: { value: 'acme/other' } });
    fireEvent.click(within(dialog).getByRole('button', { name: /Save/ }));

    await waitFor(() =>
      expect(pluginSettingsAction).toHaveBeenCalledWith('kanban', 'kanban', 'settings.save', {
        github_repo: 'acme/other',
      }),
    );
  });

  it('edits a card item with its id and values', async () => {
    render(<PluginPagePanel page={page} onRefresh={vi.fn(async () => undefined)} onPage={vi.fn()} />);

    fireEvent.click(screen.getByRole('button', { name: /Edit/ }));
    const dialog = screen.getByRole('dialog');
    const title = within(dialog).getByDisplayValue('Fix login');
    fireEvent.change(title, { target: { value: 'Fix signup' } });
    fireEvent.click(within(dialog).getByRole('button', { name: /Save/ }));

    await waitFor(() =>
      expect(pluginSettingsAction).toHaveBeenCalledWith('kanban', 'kanban', 'card.update', {
        id: 'card-1',
        title: 'Fix signup',
        column: 'in_progress',
      }),
    );
  });

  it('runs a field action directly without confirmation', async () => {
    render(<PluginPagePanel page={page} onRefresh={vi.fn(async () => undefined)} onPage={vi.fn()} />);

    fireEvent.click(screen.getByRole('button', { name: /Add card/ }));
    const dialog = await screen.findByRole('dialog');
    const input = within(dialog).getByRole('textbox');
    fireEvent.change(input, { target: { value: 'New card' } });
    fireEvent.click(within(dialog).getByRole('button', { name: /Save/ }));

    await waitFor(() =>
      expect(pluginSettingsAction).toHaveBeenCalledWith('kanban', 'kanban', 'card.add', { title: 'New card' }),
    );
  });
});
