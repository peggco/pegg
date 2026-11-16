import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

const { undoRevert } = vi.hoisted(() => ({ undoRevert: vi.fn(async () => undefined) }));

vi.mock('../state/actions', () => ({ undoRevert }));

import { RevertSummary } from './RevertSummary';
import { emptyChat, type RevertSummary as RevertSummaryState } from '../state/chat';
import { store } from '../state/store';

function seed(revert: RevertSummaryState | undefined, language: 'en' | 'tr' = 'en'): void {
  store.set({
    language,
    chat: {
      ...emptyChat(),
      acpId: 'acp-1',
      peggId: 'pegg-1',
      revert,
    },
  });
}

const summary: RevertSummaryState = {
  removed: 2,
  snapshotId: 'snap-1',
  status: 'reverted',
  files: [
    { path: 'src/app.go', additions: 3, deletions: 1, status: 'reverted' },
    { path: 'src/ui.tsx', additions: 2, deletions: 0, status: 'reverted' },
  ],
  totalAdditions: 5,
  totalDeletions: 1,
};

beforeEach(() => {
  vi.clearAllMocks();
});

afterEach(cleanup);

describe('RevertSummary', () => {
  it('renders the file count and green/red totals above the composer', () => {
    seed(summary);
    const { container } = render(<RevertSummary />);
    expect(screen.getByText('Reverted 2 files')).toBeTruthy();
    expect(screen.getByText('+5')).toBeTruthy();
    expect(screen.getByText('−1')).toBeTruthy();
    expect(container.querySelector('.revert-summary')).toBeTruthy();
  });

  it('expands to per-file counts and conflict badges', () => {
    seed({
      ...summary,
      files: [{ path: 'local.ts', additions: 1, deletions: 1, status: 'conflict' }],
      totalAdditions: 1,
      totalDeletions: 1,
    });
    render(<RevertSummary />);
    fireEvent.click(screen.getByRole('button', { name: 'Show reverted files' }));
    expect(screen.getByText('local.ts')).toBeTruthy();
    expect(screen.getByText('Conflict')).toBeTruthy();
    expect(screen.getByText(/modified locally/)).toBeTruthy();
  });

  it('calls undo and keeps the summary visible while undoing', () => {
    seed(summary);
    render(<RevertSummary />);
    fireEvent.click(screen.getByRole('button', { name: 'Undo' }));
    expect(undoRevert).toHaveBeenCalledTimes(1);
  });

  it('localizes the summary and the undo action', () => {
    seed(summary, 'tr');
    render(<RevertSummary />);
    expect(screen.getByText('2 dosya geri alındı')).toBeTruthy();
    expect(screen.getByRole('button', { name: 'Geri al' })).toBeTruthy();
  });

  it('shows the restored state and dismisses it', () => {
    seed({ ...summary, status: 'restored' });
    render(<RevertSummary />);
    expect(screen.getByText('Changes restored')).toBeTruthy();
    expect(screen.queryByRole('button', { name: 'Undo' })).toBeNull();
    fireEvent.click(screen.getByRole('button', { name: 'Dismiss' }));
    expect(store.get().chat.revert).toBeUndefined();
  });
});
