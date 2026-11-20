import { renderToStaticMarkup } from 'react-dom/server';
import { describe, expect, it, vi } from 'vitest';
import { RevertSummaryBar, type RevertSummaryView } from './RevertSummary';

function summary(overrides: Partial<RevertSummaryView> = {}): RevertSummaryView {
  return {
    removed: 2,
    snapshotId: 'snap-1',
    status: 'reverted',
    files: [
      { path: 'src/app.go', additions: 3, deletions: 1, status: 'reverted' },
      { path: 'src/ui.tsx', additions: 2, deletions: 0, status: 'reverted' },
    ],
    totalAdditions: 5,
    totalDeletions: 1,
    ...overrides,
  };
}

const noop = () => undefined;

describe('RevertSummaryBar', () => {
  it('renders the collapsed file count and totals', () => {
    const html = renderToStaticMarkup(
      <RevertSummaryBar summary={summary()} onUndo={noop} onDismiss={noop} />,
    );
    expect(html).toContain('Reverted 2 files');
    expect(html).toContain('+5');
    expect(html).toContain('−1');
    expect(html).toContain('>Undo<');
    expect(html).toContain('aria-expanded="false"');
  });

  it('localizes the summary into Turkish and keeps the undo action', () => {
    const html = renderToStaticMarkup(
      <RevertSummaryBar summary={summary()} language="tr" onUndo={noop} onDismiss={noop} />,
    );
    expect(html).toContain('2 dosya geri alındı');
    expect(html).toContain('>Geri al<');
  });

  it('renders the restored state', () => {
    const html = renderToStaticMarkup(
      <RevertSummaryBar summary={summary({ status: 'restored' })} onUndo={noop} onDismiss={noop} />,
    );
    expect(html).toContain('Changes restored');
    expect(html).toContain('The reverted messages are back.');
    expect(html).not.toContain('>Undo<');
  });

  it('renders an error message after a failed undo', () => {
    const html = renderToStaticMarkup(
      <RevertSummaryBar
        summary={summary({ status: 'error', error: 'snapshot missing' })}
        onUndo={vi.fn()}
        onDismiss={noop}
      />,
    );
    expect(html).toContain('snapshot missing');
  });

  it('shows conflict badges and the local-modification note', () => {
    const html = renderToStaticMarkup(
      <RevertSummaryBar
        summary={summary({
          files: [{ path: 'local.ts', additions: 1, deletions: 1, status: 'conflict' }],
          totalAdditions: 1,
          totalDeletions: 1,
        })}
        defaultExpanded
        onUndo={noop}
        onDismiss={noop}
      />,
    );
    expect(html).toContain('Reverted 1 file');
    expect(html).toContain('local.ts');
    expect(html).toContain('Conflict');
    expect(html).toContain('modified locally');
  });
});
