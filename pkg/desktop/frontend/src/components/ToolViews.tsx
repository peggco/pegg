import { useMemo, useState } from 'react';
import {
  Brain,
  Check,
  ChevronRight,
  FileText,
  Globe,
  Pencil,
  Search,
  Terminal,
  Wrench,
} from 'lucide-react';
import { shortPath, toolKindLabel } from '../lib/format';
import { highlightCode, languageForPath, stripAnsi } from '../lib/markdown';
import { useT } from '../lib/i18n';
import type { ToolState } from '../state/chat';

function StatusMark({ status }: { status: string }) {
  if (status === 'completed') {
    return <Check size={12} className="status-check" aria-label="completed" />;
  }
  return <span className={`status-dot ${status}`} title={status} />;
}

function ToolIcon({ kind }: { kind?: string }) {
  switch (kind) {
    case 'execute':
      return <Terminal size={13} />;
    case 'read':
      return <FileText size={13} />;
    case 'edit':
      return <Pencil size={13} />;
    case 'search':
      return <Search size={13} />;
    case 'fetch':
      return <Globe size={13} />;
    case 'think':
      return <Brain size={13} />;
    default:
      return <Wrench size={13} />;
  }
}

function inputRecord(input: unknown): Record<string, unknown> {
  return input && typeof input === 'object' ? (input as Record<string, unknown>) : {};
}

function outputText(output: unknown): string {
  if (output === undefined || output === null) return '';
  if (typeof output === 'string') return stripAnsi(output);
  try {
    return stripAnsi(JSON.stringify(output, null, 2));
  } catch {
    return stripAnsi(String(output));
  }
}

function toolPath(tool: ToolState): string {
  const record = inputRecord(tool.input);
  if (typeof record.path === 'string') return record.path;
  if (tool.diff?.path) return tool.diff.path;
  return tool.locations?.[0]?.path ?? '';
}

function writeContent(tool: ToolState): string {
  const record = inputRecord(tool.input);
  for (const key of ['content', 'text', 'newText', 'new_string', 'newString']) {
    const value = record[key];
    if (typeof value === 'string') return value;
  }
  if (tool.diff?.newText) return tool.diff.newText;
  return outputText(tool.output);
}

function bashCommand(tool: ToolState): string {
  const record = inputRecord(tool.input);
  if (typeof record.command === 'string' && record.command.trim()) return record.command.trim();
  const title = (tool.title ?? '').replace(/^bash:?\s*/i, '');
  return title || tool.name || 'bash';
}

function classify(tool: ToolState): 'bash' | 'write' | 'edit' | 'compact' {
  const haystack = `${tool.name ?? ''} ${tool.title ?? ''}`.toLowerCase();
  if (tool.kind === 'execute' || /\bbash\b/.test(haystack)) return 'bash';
  if (/\bwrite\b/.test(haystack)) return 'write';
  if (tool.diff && (tool.kind === 'edit' || /\bedit\b/.test(haystack) || !haystack.trim())) return 'edit';
  if (tool.kind === 'edit' && tool.diff) return 'edit';
  return 'compact';
}

export function ToolCard({ tool }: { tool: ToolState }) {
  switch (classify(tool)) {
    case 'bash':
      return <BashTool tool={tool} />;
    case 'write':
      return <WriteTool tool={tool} />;
    case 'edit':
      return <EditTool tool={tool} />;
    default:
      return <CompactTool tool={tool} />;
  }
}

function CompactTool({ tool }: { tool: ToolState }) {
  const t = useT();
  const [showError, setShowError] = useState(false);
  const path = toolPath(tool);
  const base = path.split(/[\\/]/).pop() ?? '';
  const showPath = Boolean(path) && !tool.title.includes(path) && !tool.title.includes(base);
  return (
    <div className="tool-mini-wrap">
      <div className={`tool-mini${tool.error ? ' has-error' : ''}`}>
        <span className="tool-mini-icon">
          <ToolIcon kind={tool.kind} />
        </span>
        <span className="tool-mini-title" title={tool.title}>
          {tool.title}
        </span>
        {showPath ? <span className="tool-mini-path">{shortPath(path, 48)}</span> : null}
        <span className="tool-kind">{toolKindLabel(tool.kind)}</span>
        {tool.error ? (
          <button className="tool-error-toggle" onClick={() => setShowError((value) => !value)}>
            {showError ? t('common.hideError') : t('common.showError')}
          </button>
        ) : null}
        <StatusMark status={tool.status} />
      </div>
      {showError && tool.error ? <pre className="tool-error">{tool.error}</pre> : null}
    </div>
  );
}

function BashTool({ tool }: { tool: ToolState }) {
  const t = useT();
  const [expanded, setExpanded] = useState(false);
  const command = bashCommand(tool);
  const output = outputText(tool.output);
  const lines = output ? output.split('\n') : [];
  const limit = 24;
  const truncated = !expanded && lines.length > limit;
  const shown = truncated ? lines.slice(0, limit) : lines;

  return (
    <div className="terminal-card">
      <div className="terminal-head">
        <span className="terminal-dots">
          <i />
          <i />
          <i />
        </span>
        <Terminal size={12} />
        <span>bash</span>
        {tool.status === 'in_progress' ? <span className="terminal-status">running</span> : null}
        {tool.status === 'failed' ? <span className="terminal-status terminal-error">failed</span> : null}
        <span className="terminal-spacer" />
        <StatusMark status={tool.status} />
      </div>
      <div className="terminal-body">
        <div className="terminal-cmd">
          <span className="terminal-prompt">$</span>
          <span>{command}</span>
        </div>
        {shown.map((line, index) => (
          <div className="terminal-line" key={index}>
            {line || '\u00a0'}
          </div>
        ))}
        {!lines.length && !tool.error ? <div className="terminal-line terminal-muted">{t('common.noOutput')}</div> : null}
        {tool.error ? <div className="terminal-line terminal-error">{tool.error}</div> : null}
        {truncated ? (
          <button className="terminal-more" onClick={() => setExpanded(true)}>
            {t('common.showAllLines', { count: lines.length })}
          </button>
        ) : null}
        {expanded && lines.length > limit ? (
          <button className="terminal-more" onClick={() => setExpanded(false)}>
            {t('common.collapse')}
          </button>
        ) : null}
      </div>
    </div>
  );
}

function WriteTool({ tool }: { tool: ToolState }) {
  const t = useT();
  const content = writeContent(tool);
  const path = toolPath(tool);
  const lines = content.split('\n');
  const lineLimit = 30;
  const truncate = content.length > 16000 || lines.length > lineLimit;
  const [expanded, setExpanded] = useState(false);
  const truncated = truncate && !expanded;
  const shown = truncated ? lines.slice(0, lineLimit).join('\n') : content;
  const lang = languageForPath(path);
  const canExpand = truncate;

  return (
    <div className={`tool-card write-card${expanded ? ' open' : ''}`}>
      <button
        className="tool-head"
        onClick={() => canExpand && setExpanded((value) => !value)}
        aria-expanded={canExpand ? expanded : undefined}
        style={{ cursor: canExpand ? 'pointer' : 'default' }}
      >
        <span className="tool-icon">
          <Pencil size={13} />
        </span>
        <span className="tool-title" title={tool.title}>
          {tool.title}
        </span>
        <span className="tool-kind">write</span>
        <StatusMark status={tool.status} />
        {canExpand ? <ChevronRight size={13} className="tool-chevron" /> : null}
      </button>
      {content ? (
        <div className="write-body">
          <pre className="write-code">
            <code dangerouslySetInnerHTML={{ __html: highlightCode(shown, lang) }} />
          </pre>
          {truncated ? (
            <button className="code-more" onClick={() => setExpanded(true)}>
              {t('common.showAllLines', { count: lines.length })}
            </button>
          ) : null}
          {expanded && truncate ? (
            <button className="code-more" onClick={() => setExpanded(false)}>
              {t('common.collapse')}
            </button>
          ) : null}
        </div>
      ) : null}
      {tool.error ? <pre className="tool-error">{tool.error}</pre> : null}
    </div>
  );
}

function EditTool({ tool }: { tool: ToolState }) {
  const [expanded, setExpanded] = useState(false);
  const diff = tool.diff;
  return (
    <div className={`tool-card edit-card${expanded ? ' open' : ''}`}>
      <div className="tool-head static">
        <span className="tool-icon">
          <Pencil size={13} />
        </span>
        <span className="tool-title" title={tool.title}>
          {tool.title}
        </span>
        <span className="tool-kind">edit</span>
        <StatusMark status={tool.status} />
      </div>
      {diff ? <DiffView diff={diff} expanded={expanded} onToggle={() => setExpanded((value) => !value)} /> : null}
      {tool.error ? <pre className="tool-error">{tool.error}</pre> : null}
    </div>
  );
}

type DiffRow =
  | { kind: 'ctx' | 'add' | 'del'; text: string; oldLine?: number; newLine?: number }
  | { kind: 'hunk'; text: string };

function computeRows(oldText: string, newText: string): DiffRow[] {
  const a = oldText === '' ? [] : oldText.replace(/\r\n/g, '\n').split('\n');
  const b = newText === '' ? [] : newText.replace(/\r\n/g, '\n').split('\n');

  let start = 0;
  while (start < a.length && start < b.length && a[start] === b[start]) start++;
  let endA = a.length - 1;
  let endB = b.length - 1;
  while (endA >= start && endB >= start && a[endA] === b[endB]) {
    endA--;
    endB--;
  }

  const middleA = a.slice(start, endA + 1);
  const middleB = b.slice(start, endB + 1);
  const ops: Array<{ kind: ' ' | '-' | '+'; text: string }> = [];
  for (let i = 0; i < start; i++) ops.push({ kind: ' ', text: a[i] });

  const n = middleA.length;
  const m = middleB.length;
  if (n > 0 || m > 0) {
    if (n * m <= 250000) {
      const dp: number[][] = Array.from({ length: n + 1 }, () => new Array(m + 1).fill(0));
      for (let i = n - 1; i >= 0; i--) {
        for (let j = m - 1; j >= 0; j--) {
          dp[i][j] = middleA[i] === middleB[j] ? dp[i + 1][j + 1] + 1 : Math.max(dp[i + 1][j], dp[i][j + 1]);
        }
      }
      let i = 0;
      let j = 0;
      while (i < n && j < m) {
        if (middleA[i] === middleB[j]) {
          ops.push({ kind: ' ', text: middleA[i] });
          i++;
          j++;
        } else if (dp[i + 1][j] >= dp[i][j + 1]) {
          ops.push({ kind: '-', text: middleA[i++] });
        } else {
          ops.push({ kind: '+', text: middleB[j++] });
        }
      }
      while (i < n) ops.push({ kind: '-', text: middleA[i++] });
      while (j < m) ops.push({ kind: '+', text: middleB[j++] });
    } else {
      for (const line of middleA) ops.push({ kind: '-', text: line });
      for (const line of middleB) ops.push({ kind: '+', text: line });
    }
  }
  for (let i = endA + 1; i < a.length; i++) ops.push({ kind: ' ', text: a[i] });

  const full: DiffRow[] = [];
  let oldLine = 1;
  let newLine = 1;
  for (const op of ops) {
    if (op.kind === ' ') {
      full.push({ kind: 'ctx', text: op.text, oldLine: oldLine++, newLine: newLine++ });
    } else if (op.kind === '-') {
      full.push({ kind: 'del', text: op.text, oldLine: oldLine++ });
    } else {
      full.push({ kind: 'add', text: op.text, newLine: newLine++ });
    }
  }

  const changed = full
    .map((row, index) => (row.kind === 'ctx' ? -1 : index))
    .filter((index) => index >= 0);
  if (!changed.length) return [];

  const keep = new Set<number>();
  for (const index of changed) {
    for (let i = Math.max(0, index - 3); i <= Math.min(full.length - 1, index + 3); i++) keep.add(i);
  }

  const out: DiffRow[] = [];
  let index = 0;
  while (index < full.length) {
    if (!keep.has(index)) {
      index++;
      continue;
    }
    let end = index;
    while (end < full.length && keep.has(end)) end++;
    const group = full.slice(index, end);
    const olds = group.filter((row) => row.kind !== 'hunk' && row.oldLine !== undefined) as Array<{ oldLine: number }>;
    const news = group.filter((row) => row.kind !== 'hunk' && row.newLine !== undefined) as Array<{ newLine: number }>;
    const oldStart = olds[0]?.oldLine ?? 0;
    const newStart = news[0]?.newLine ?? 0;
    const oldCount = olds.length;
    const newCount = news.length;
    out.push({ kind: 'hunk', text: `@@ -${oldStart},${oldCount} +${newStart},${newCount} @@` });
    out.push(...group);
    index = end;
  }
  return out;
}

export function DiffView({
  diff,
  expanded,
  onToggle,
}: {
  diff: { path: string; oldText?: string; newText: string };
  expanded: boolean;
  onToggle: () => void;
}) {
  const t = useT();
  const rows = useMemo(() => computeRows(diff.oldText ?? '', diff.newText ?? ''), [diff.oldText, diff.newText]);
  const adds = rows.filter((row) => row.kind === 'add').length;
  const dels = rows.filter((row) => row.kind === 'del').length;
  const limit = 160;
  const visible = expanded ? rows : rows.slice(0, limit);
  const hasMore = rows.length > limit;

  return (
    <div className="diff gh-diff">
      <div className="diff-head">
        <Pencil size={12} />
        <span className="diff-path">{shortPath(diff.path, 80)}</span>
        <span className="diff-stat">
          <span className="add">+{adds}</span> <span className="del">−{dels}</span>
        </span>
      </div>
      <div className="diff-body">
        {rows.length === 0 ? <div className="diff-empty">{t('common.noOutput')}</div> : null}
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
              <span className="diff-text">{row.text || ' '}</span>
            </div>
          ),
        )}
      </div>
      {hasMore ? (
        <button className="diff-more" onClick={onToggle}>
          {expanded ? t('common.collapse') : t('common.showAllLines', { count: rows.length })}
        </button>
      ) : null}
    </div>
  );
}
