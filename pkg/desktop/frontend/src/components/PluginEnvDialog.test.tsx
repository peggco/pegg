import { cleanup, fireEvent, render, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { PluginEnvDialog } from './PluginEnvDialog';
import { store } from '../state/store';

const { pluginEnv, setPluginEnv } = vi.hoisted(() => ({
  pluginEnv: vi.fn(
    async (_name: string): Promise<Record<string, string>> => ({ GITHUB_TOKEN: 'ghp_secret' }),
  ),
  setPluginEnv: vi.fn(async (_name: string, _env: Record<string, string>): Promise<void> => undefined),
}));

vi.mock('../lib/bridge', () => ({
  api: {
    PluginEnv: pluginEnv,
    SetPluginEnv: setPluginEnv,
  },
  hasWails: false,
}));

beforeEach(() => {
  vi.clearAllMocks();
  store.set({ language: 'en' });
});

afterEach(() => {
  cleanup();
});

describe('PluginEnvDialog', () => {
  it('loads configured values as masked inputs', async () => {
    const { container } = render(<PluginEnvDialog name="kanban" onClose={() => undefined} />);

    await waitFor(() => expect(pluginEnv).toHaveBeenCalledWith('kanban'));
    const key = container.querySelector('input[aria-label="Key"]') as HTMLInputElement;
    const value = container.querySelector('input[aria-label="Value"]') as HTMLInputElement;

    expect(key?.value).toBe('GITHUB_TOKEN');
    expect(value?.value).toBe('ghp_secret');
    expect(value?.type).toBe('password');
  });

  it('toggles value visibility', async () => {
    const { container, getByLabelText } = render(<PluginEnvDialog name="kanban" onClose={() => undefined} />);

    await waitFor(() => expect(container.querySelector('input[aria-label="Value"]')).toBeTruthy());
    fireEvent.click(getByLabelText('Reveal value'));

    const value = container.querySelector('input[aria-label="Value"]') as HTMLInputElement;
    expect(value.type).toBe('text');
  });

  it('adds, edits and saves a variable', async () => {
    pluginEnv.mockResolvedValueOnce({});
    const onClose = vi.fn();
    const { container, getByText } = render(
      <PluginEnvDialog name="kanban" onClose={onClose} />,
    );

    await waitFor(() => expect(container.querySelector('input[aria-label="Key"]')).toBeNull());
    fireEvent.click(getByText('Add variable'));

    fireEvent.change(container.querySelector('input[aria-label="Key"]') as HTMLElement, {
      target: { value: 'JIRA_TOKEN' },
    });
    fireEvent.change(container.querySelector('input[aria-label="Value"]') as HTMLElement, {
      target: { value: 'secret' },
    });
    fireEvent.click(getByText('Save changes'));

    await waitFor(() => expect(setPluginEnv).toHaveBeenCalledWith('kanban', { JIRA_TOKEN: 'secret' }));
    expect(onClose).toHaveBeenCalled();
  });

  it('rejects an invalid variable name without saving', async () => {
    pluginEnv.mockResolvedValueOnce({});
    const { container, getByText } = render(<PluginEnvDialog name="kanban" onClose={() => undefined} />);

    await waitFor(() => expect(container.querySelector('input[aria-label="Key"]')).toBeNull());
    fireEvent.click(getByText('Add variable'));
    fireEvent.change(container.querySelector('input[aria-label="Key"]') as HTMLElement, {
      target: { value: 'bad key' },
    });
    fireEvent.click(getByText('Save changes'));

    await waitFor(() => expect(container.textContent).toContain('Invalid variable name'));
    expect(setPluginEnv).not.toHaveBeenCalled();
  });

  it('removes a variable before saving', async () => {
    pluginEnv.mockResolvedValueOnce({ GITHUB_TOKEN: 'ghp_secret', ORG: 'pegg' });
    const { container, getByText } = render(<PluginEnvDialog name="kanban" onClose={() => undefined} />);

    await waitFor(() => expect(container.querySelectorAll('input[aria-label="Key"]')).toHaveLength(2));
    const removeButtons = container.querySelectorAll('button[aria-label="Remove variable"]');
    fireEvent.click(removeButtons[0]);
    fireEvent.click(getByText('Save changes'));

    await waitFor(() => expect(setPluginEnv).toHaveBeenCalledWith('kanban', { ORG: 'pegg' }));
  });
});
