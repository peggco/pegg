import { useLayoutEffect, useRef, useState } from 'react';
import type { TaskState, ToolState } from '../state/chat';
import { useT } from '../lib/i18n';

function useClampOverflow(expanded: boolean, text: string) {
  const ref = useRef<HTMLDivElement>(null);
  const [overflow, setOverflow] = useState(false);
  useLayoutEffect(() => {
    if (expanded) return;
    const element = ref.current;
    if (!element) return;
    const measure = () => setOverflow(element.scrollHeight > element.clientHeight + 1);
    measure();
    const observer = new ResizeObserver(measure);
    observer.observe(element);
    return () => observer.disconnect();
  }, [expanded, text]);
  return { ref, overflow };
}

const TASK_PROMPT_LINES = 4;
const TASK_OUTPUT_LINES = 7;
const TASK_EXPANDED_LINES = 60;

function cappedLines(text: string, limit: number, mark: boolean): string {
  const all = text.split('\n');
  if (all.length <= limit) return text;
  const shown = all.slice(0, limit).join('\n');
  return mark ? `${shown}\n…` : shown;
}

function formatDuration(ms: number): string {
  if (ms < 1000) return `${Math.round(ms)}ms`;
  return `${(ms / 1000).toFixed(1)}s`;
}

function statusText(task: TaskState): string {
  switch (task.status) {
    case 'running':
      return 'running';
    case 'error':
      return 'failed';
    case 'background':
      return 'background';
    case 'unknown':
      return 'no result';
    default:
      return task.durationMs ? `done · ${formatDuration(task.durationMs)}` : 'done';
  }
}

export function TaskCard({ tool, onOpen }: { tool: ToolState; onOpen?: () => void }) {
  const t = useT();
  const task = tool.task;
  const [expanded, setExpanded] = useState(false);
  const prompt = task?.prompt ?? '';
  const promptClamp = useClampOverflow(expanded, prompt);
  if (!task) return null;

  const running = task.status === 'running';
  const output = task.output ?? '';
  const error = task.error ?? tool.error ?? '';
  const outputLines = output.split('\n');
  const outputTruncated = outputLines.length > TASK_OUTPUT_LINES;
  const errorLines = error.split('\n');
  const errorTruncated = errorLines.length > TASK_OUTPUT_LINES;
  const canExpand = promptClamp.overflow || outputTruncated || errorTruncated;
  const visible = cappedLines(output, expanded ? TASK_EXPANDED_LINES : TASK_OUTPUT_LINES, expanded);
  const visibleError = cappedLines(error, expanded ? TASK_EXPANDED_LINES : TASK_OUTPUT_LINES, expanded);

  return (
    <article className={`task-card status-${task.status}${expanded ? ' expanded' : ''}`}>
      <div className="task-head">
        <span className="task-badge">TASK</span>
        <span className="task-name">{task.name}</span>
        {task.type ? <span className="task-chip">{task.type}</span> : null}
        {task.background ? <span className="task-chip subtle">{t('task.background')}</span> : null}
        <span className="task-spacer" />
        <span className={`task-dot status-${task.status}`} />
        <span className="task-state">{statusText(task)}</span>
        {expanded ? (
          <button className="task-open" onClick={() => setExpanded(false)}>
            {t('task.collapse')}
          </button>
        ) : running && onOpen ? (
          <button className="task-open" onClick={onOpen}>{t('task.history')}</button>
        ) : canExpand ? (
          <button className="task-open" onClick={() => setExpanded(true)}>
            {t('task.expand')}
          </button>
        ) : null}
      </div>
      {task.prompt ? (
        <div
          ref={promptClamp.ref}
          className="task-prompt"
          style={expanded ? undefined : { WebkitLineClamp: TASK_PROMPT_LINES }}
        >
          {task.prompt}
        </div>
      ) : null}
      {!expanded && promptClamp.overflow ? (
        <button className="task-more" onClick={() => setExpanded(true)} title={t('task.expand')}>
          …
        </button>
      ) : null}
      {running ? (
        <div className="task-activity">
          <span className="task-spinner" />
          <span>{task.activity || t('task.starting')}</span>
        </div>
      ) : task.status === 'error' ? (
        <>
          <pre className="task-error">{visibleError || t('task.failed')}</pre>
          {!expanded && errorTruncated ? (
            <button className="task-more" onClick={() => setExpanded(true)} title={t('task.expand')}>
              …
            </button>
          ) : null}
        </>
      ) : task.status === 'background' ? (
        <div className="task-note">{t('task.backgroundNote')}</div>
      ) : task.status === 'unknown' ? (
        <div className="task-note">{t('task.noResult')}</div>
      ) : output ? (
        <>
          <pre className="task-output">{visible}</pre>
          {!expanded && outputTruncated ? (
            <button className="task-more" onClick={() => setExpanded(true)} title={t('task.expand')}>
              …
            </button>
          ) : null}
        </>
      ) : (
        <div className="task-note">{t('task.noResult')}</div>
      )}
      {task.usage || task.durationMs ? (
        <div className="task-foot">
          <span>{task.usage ? `${task.usage.used.toLocaleString()} tokens` : ''}</span>
          {task.durationMs ? <span>{formatDuration(task.durationMs)}</span> : null}
        </div>
      ) : null}
    </article>
  );
}
