import { cleanup, fireEvent, render, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { MCPEnvDialog } from './MCPEnvDialog';
import { store } from '../state/store';

vi.mock('../lib/bridge', () => ({
  api: { OpenExternal: vi.fn(async () => undefined) },
  hasWails: false,
}));

beforeEach(() => {
  vi.clearAllMocks();
  store.set({ language: 'en' });
});

afterEach(() => {
  cleanup();
});

describe('MCPEnvDialog', () => {
  it('renders required variables as password inputs', () => {
    const { container } = render(
      <MCPEnvDialog name="GitHub" requiredEnv={['GITHUB_TOKEN', 'ORG']} onClose={() => undefined} onSubmit={() => undefined} />,
    );

    const token = container.querySelector('input[name="GITHUB_TOKEN"]') as HTMLInputElement;
    const org = container.querySelector('input[name="ORG"]') as HTMLInputElement;
    expect(token?.type).toBe('password');
    expect(org?.type).toBe('password');
    expect(container.textContent).toContain('GITHUB_TOKEN');
  });

  it('blocks submission until every value is filled', async () => {
    const onSubmit = vi.fn(async () => undefined);
    const { getByText, container } = render(
      <MCPEnvDialog name="GitHub" requiredEnv={['GITHUB_TOKEN']} onClose={() => undefined} onSubmit={onSubmit} />,
    );

    fireEvent.click(getByText('Install'));

    expect(container.textContent).toContain('GITHUB_TOKEN');
    expect(onSubmit).not.toHaveBeenCalled();

    fireEvent.change(container.querySelector('input[name="GITHUB_TOKEN"]') as HTMLElement, {
      target: { value: 'ghp_secret' },
    });
    fireEvent.click(getByText('Install'));

    await waitFor(() => expect(onSubmit).toHaveBeenCalledWith({ GITHUB_TOKEN: 'ghp_secret' }));
  });

  it('shows submission errors without leaking values', async () => {
    const onSubmit = vi.fn(async () => {
      throw new Error('config write failed');
    });
    const { getByText, container } = render(
      <MCPEnvDialog name="GitHub" requiredEnv={['GITHUB_TOKEN']} onClose={() => undefined} onSubmit={onSubmit} />,
    );

    fireEvent.change(container.querySelector('input[name="GITHUB_TOKEN"]') as HTMLElement, {
      target: { value: 'ghp_secret' },
    });
    fireEvent.click(getByText('Install'));

    await waitFor(() => expect(container.textContent).toContain('config write failed'));
    expect(container.textContent).not.toContain('ghp_secret');
  });
});
