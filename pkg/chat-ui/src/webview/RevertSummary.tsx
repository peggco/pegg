import { useState, type MouseEvent } from 'react';
import { translate } from '../i18n';
import type { RevertFileChange, RevertSummary } from '../types';

export type RevertViewStatus = 'reverted' | 'undoing' | 'restored' | 'error';

export interface RevertSummaryView extends RevertSummary {
  removed: number;
  snapshotId?: string;
  status: RevertViewStatus;
  error?: string;
}

interface Props {
  summary: RevertSummaryView;
  language?: string;
  defaultExpanded?: boolean;
  onUndo: () => void;
  onDismiss: () => void;
}

function statusLabel(language: string | undefined, status: RevertFileChange['status']): string {
  if (status === 'conflict') return translate(language, 'revert.status.conflict');
  if (status === 'skipped') return translate(language, 'revert.status.skipped');
  return translate(language, 'revert.status.reverted');
}

export function RevertSummaryBar({ summary, language, defaultExpanded = false, onUndo, onDismiss }: Props) {
  const [expanded, setExpanded] = useState(defaultExpanded);
  const t = (key: string, vars?: Record<string, string | number>) => translate(language, key, vars);
  const files = summary.files ?? [];
  const status = summary.status;
  const restored = status === 'restored';
  const restoring = status === 'undoing';
  const failed = status === 'error';

  const title = restored
    ? t('revert.restored')
    : files.length === 0
      ? t('revert.title.none')
      : t(files.length === 1 ? 'revert.title.one' : 'revert.title.other', { count: files.length });

  const keepFocus = (event: MouseEvent) => event.preventDefault();

  return (
    <div className={`revert-summary ${status}`} role="status" aria-live="polite">
      <div className="revert-summary-head">
        <span className="revert-summary-icon" aria-hidden="true">
          {restored ? '✓' : '↺'}
        </span>
        <span className="revert-summary-title">{title}</span>
        {!restored && files.length > 0 ? (
          <span className="revert-summary-stat">
            <span className="add">+{summary.totalAdditions}</span>{' '}
            <span className="del">−{summary.totalDeletions}</span>
          </span>
        ) : null}
        <span className="revert-summary-spacer" />
        {!restored && files.length > 0 ? (
          <button
            type="button"
            className="revert-summary-iconbtn"
            aria-expanded={expanded}
            aria-label={t(expanded ? 'revert.hideFiles' : 'revert.showFiles')}
            title={t(expanded ? 'revert.hideFiles' : 'revert.showFiles')}
            onMouseDown={keepFocus}
            onClick={() => setExpanded((value) => !value)}
          >
            <svg width="12" height="12" viewBox="0 0 16 16" aria-hidden="true">
              <path
                d={expanded ? 'M4 10l4-4 4 4' : 'M4 6l4 4 4-4'}
                fill="none"
                stroke="currentColor"
                strokeWidth="1.6"
                strokeLinecap="round"
                strokeLinejoin="round"
              />
            </svg>
          </button>
        ) : null}
        {restored ? (
          <button
            type="button"
            className="revert-summary-iconbtn"
            aria-label={t('revert.dismiss')}
            title={t('revert.dismiss')}
            onMouseDown={keepFocus}
            onClick={onDismiss}
          >
            ×
          </button>
        ) : (
          <button
            type="button"
            className="revert-summary-undo"
            disabled={restoring || !summary.snapshotId}
            onMouseDown={keepFocus}
            onClick={onUndo}
          >
            {restoring ? t('revert.undoing') : t('revert.undo')}
          </button>
        )}
      </div>

      {restored ? (
        <div className="revert-summary-restored">{t('revert.restoredDetail')}</div>
      ) : null}

      {failed ? (
        <div className="revert-summary-error">{summary.error || t('revert.undoFailed')}</div>
      ) : null}

      {!restored && expanded && files.length > 0 ? (
        <div className="revert-summary-files">
          {files.map((file) => {
            const fileStatus = file.status ?? 'reverted';
            return (
              <div key={file.path} className={`revert-summary-file ${fileStatus}`}>
                <span className="revert-summary-path" title={file.path}>
                  {file.path}
                </span>
                <span className={`revert-summary-badge ${fileStatus}`}>{statusLabel(language, fileStatus)}</span>
                <span className="revert-summary-stat">
                  <span className="add">+{file.additions}</span>{' '}
                  <span className="del">−{file.deletions}</span>
                </span>
              </div>
            );
          })}
          {files.some((file) => file.status === 'conflict') ? (
            <div className="revert-summary-note">{t('revert.conflictHint')}</div>
          ) : null}
          {files.some((file) => file.status === 'skipped') ? (
            <div className="revert-summary-note">{t('revert.skippedHint')}</div>
          ) : null}
        </div>
      ) : null}
    </div>
  );
}
