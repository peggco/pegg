import { useMemo, useState } from 'react';
import { computeDiffRows } from './diff';
import { highlightCode, languageForPath } from './markdown';

const DIFF_LIMIT = 160;

export function DiffView({ diff }: { diff: { path: string; oldText?: string; newText: string } }) {
  const [expanded, setExpanded] = useState(false);
  const rows = useMemo(
    () => computeDiffRows(diff.oldText ?? '', diff.newText ?? ''),
    [diff.oldText, diff.newText],
  );
  const language = useMemo(() => languageForPath(diff.path), [diff.path]);
  const highlighted = useMemo(() => {
    const out = rows.map((row) => (row.kind === 'hunk' ? '' : highlightCode(row.text || ' ', language)));
    return out;
  }, [rows, language]);
  const adds = rows.filter((row) => row.kind === 'add').length;
  const dels = rows.filter((row) => row.kind === 'del').length;
  const visible = expanded ? rows : rows.slice(0, DIFF_LIMIT);
  const hasMore = rows.length > DIFF_LIMIT;

  return (
    <div className="diff">
      <div className="diff-head">
        <span className="diff-icon" aria-hidden="true">
          ±
        </span>
        <span className="diff-path" title={diff.path}>
          {shortPath(diff.path, 80)}
        </span>
        <span className="diff-stat">
          <span className="add">+{adds}</span> <span className="del">−{dels}</span>
        </span>
      </div>
      <div className="diff-body">
        <div className="diff-scroll">
          {rows.length === 0 ? <div className="diff-empty">No changes</div> : null}
          {visible.map((row, index) =>
            row.kind === 'hunk' ? (
              <div className="diff-hunk" key={`h-${index}`}>
                {row.text}
              </div>
            ) : (
              <div className={`diff-line ${row.kind}`} key={`l-${index}`}>
                <span className="diff-num">{row.oldLine ?? ''}</span>
                <span className="diff-num">{row.newLine ?? ''}</span>
                <span className="diff-sign">{row.kind === 'add' ? '+' : row.kind === 'del' ? '−' : ' '}</span>
                <span className="diff-text" dangerouslySetInnerHTML={{ __html: highlighted[index] ?? '' }} />
              </div>
            ),
          )}
        </div>
      </div>
      {hasMore ? (
        <button className="diff-more" onClick={() => setExpanded((value) => !value)}>
          {expanded ? 'Collapse' : `Show all ${rows.length} lines`}
        </button>
      ) : null}
    </div>
  );
}

function shortPath(path: string, max: number): string {
  if (!path) return '';
  const clean = path.replace(/\\/g, '/');
  if (clean.length <= max) return clean;
  const parts = clean.split('/');
  if (parts.length <= 2) return clean.slice(-max);
  return `${parts[0]}/…/${parts.slice(-2).join('/')}`;
}
