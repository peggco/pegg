import { useMemo, useState } from 'react';
import { highlightCode, languageForPath } from './markdown';

const WRITE_LINE_LIMIT = 30;

export function WriteView({ path, content }: { path: string; content: string }) {
  const [expanded, setExpanded] = useState(false);
  const lines = useMemo(() => content.split('\n'), [content]);
  const truncate = content.length > 16000 || lines.length > WRITE_LINE_LIMIT;
  const truncated = truncate && !expanded;
  const shown = truncated ? lines.slice(0, WRITE_LINE_LIMIT).join('\n') : content;
  const html = useMemo(() => highlightCode(shown, languageForPath(path)), [shown, path]);

  return (
    <div className="write-body">
      <pre className="write-code">
        <code className="hljs" dangerouslySetInnerHTML={{ __html: html }} />
      </pre>
      {truncated ? (
        <button className="diff-more" onClick={() => setExpanded(true)}>
          Show all {lines.length} lines
        </button>
      ) : null}
      {expanded && truncate ? (
        <button className="diff-more" onClick={() => setExpanded(false)}>
          Collapse
        </button>
      ) : null}
    </div>
  );
}
