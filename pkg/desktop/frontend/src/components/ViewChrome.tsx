import type { DesktopViewState, ViewFragment, ViewLine, ViewSpan } from '../types';
import { viewLineSpans } from '../lib/view';

const SPAN_STYLES = new Set(['default', 'muted', 'hint', 'accent', 'error', 'warning', 'success', 'bold']);

function spanClass(style?: string): string {
  const name = style && SPAN_STYLES.has(style) ? style : 'default';
  return `view-span view-${name}`;
}

function ViewLineRow({ line }: { line: ViewLine }) {
  const spans = viewLineSpans(line);
  if (!spans.length) return null;
  return (
    <div className="view-line">
      {spans.map((span, index) => (
        <span key={`${index}-${span.text}`} className={spanClass(span.style)}>
          {span.text}
        </span>
      ))}
    </div>
  );
}

export function ViewLines({ lines, className = 'view-lines' }: { lines?: ViewLine[]; className?: string }) {
  if (!lines?.length) return null;
  return (
    <div className={className}>
      {lines.map((line, index) => (
        <ViewLineRow key={index} line={line} />
      ))}
    </div>
  );
}

function SpanList({ spans }: { spans: ViewSpan[] }) {
  return (
    <>
      {spans.map((span, index) => (
        <span key={`${index}-${span.text}`} className={spanClass(span.style)}>
          {span.text}
        </span>
      ))}
    </>
  );
}

export function ViewStatusBar({ view }: { view?: DesktopViewState }) {
  const left = view?.statusLeft ?? [];
  const right = view?.statusRight ?? [];
  if (!left.length && !right.length) return null;
  return (
    <div className="view-status" role="status">
      <div className="view-status-side">
        <SpanList spans={left} />
      </div>
      <div className="view-status-side view-status-right">
        <SpanList spans={right} />
      </div>
    </div>
  );
}

export function PluginCard({ fragment }: { fragment: ViewFragment }) {
  return (
    <div className="plugin-card">
      {fragment.title || fragment.badge ? (
        <div className="plugin-card-head">
          {fragment.badge ? <span className="plugin-badge">{fragment.badge}</span> : null}
          {fragment.title ? <span className="plugin-card-title">{fragment.title}</span> : null}
        </div>
      ) : null}
      <ViewLines lines={fragment.lines} className="view-lines plugin-card-lines" />
    </div>
  );
}

export function ViewEmpty({ items }: { items?: ViewFragment[] }) {
  if (!items?.length) return null;
  return (
    <div className="view-empty">
      {items.map((fragment, index) => (
        <PluginCard key={fragment.id ?? index} fragment={fragment} />
      ))}
    </div>
  );
}
