import { useMemo } from 'react';
import { Check, CircleDot, ListChecks, ShieldQuestion, Sparkles } from 'lucide-react';
import type { PlanEntry } from '../types';
import type { PendingPermission } from '../state/chat';
import type { ChatItem } from '../state/chat';
import { respondPermission } from '../state/actions';
import { useT } from '../lib/i18n';

export function PlanCard({ entries }: { entries: PlanEntry[] }) {
  const done = entries.filter((e) => e.status === 'completed').length;
  const pct = entries.length ? Math.round((done / entries.length) * 100) : 0;
  return (
    <div className="plan-card">
      <div className="plan-head">
        <span className="plan-title">
          <ListChecks size={14} />
          Plan
        </span>
        <span className="plan-progress">
          {done}/{entries.length} · {pct}%
        </span>
      </div>
      <div className="plan-list">
        {entries.map((entry, index) => {
          const status = entry.status === 'completed' ? 'completed' : entry.status === 'in_progress' ? 'in_progress' : 'pending';
          return (
            <div key={`${entry.content}-${index}`} className={`plan-item ${status}`}>
              <span className="plan-check">
                {status === 'completed' ? <Check size={11} strokeWidth={3} /> : null}
                {status === 'in_progress' ? <CircleDot size={9} /> : null}
              </span>
              <span>{entry.content}</span>
            </div>
          );
        })}
      </div>
    </div>
  );
}

export function SubagentCard({
  item,
  onOpen,
}: {
  item: Extract<ChatItem, { kind: 'subagent' }>;
  onOpen: () => void;
}) {
  const t = useT();
  return (
    <div
      className="subagent-card"
      role="button"
      tabIndex={0}
      onClick={onOpen}
      onKeyDown={(event) => {
        if (event.key === 'Enter' || event.key === ' ') {
          event.preventDefault();
          onOpen();
        }
      }}
    >
      <div className="subagent-head">
        <Sparkles size={14} />
        <span>{item.agentName || 'Subagent'}</span>
        <span className="subagent-status">
          {item.status === 'running' ? t('task.running') : item.status === 'error' ? t('task.failure') : t('task.done')}
        </span>
        <span className="subagent-open">{t('common.openHistory')}</span>
      </div>
      {item.task ? <div className="subagent-task">{item.task}</div> : null}
      {item.error ? <div className="subagent-output" style={{ color: 'var(--pg-danger)' }}>{item.error}</div> : null}
    </div>
  );
}

function permissionLabel(option: { name: string; kind: string }, index: number): string {
  const hint = index < 3 ? ` (${index + 1})` : '';
  return `${option.name}${hint}`;
}

export function PermissionCard({ request }: { request: PendingPermission }) {
  const params = request.params;
  const options = useMemo(
    () =>
      params.options?.length
        ? params.options
        : [
            { optionId: 'allow_once', name: 'Allow', kind: 'allow_once' },
            { optionId: 'allow_always', name: 'Always allow', kind: 'allow_always' },
            { optionId: 'reject_once', name: 'Reject', kind: 'reject_once' },
          ],
    [params.options],
  );
  const question =
    (params._meta?.['com.pegg/question'] as string | undefined) ||
    params.toolCall?.title ||
    'Allow this tool call?';
  const toolPreview = params.toolCall?.rawInput ? JSON.stringify(params.toolCall.rawInput, null, 2) : '';

  return (
    <div className="permission-card" role="alertdialog" aria-label="Permission required">
      <div className="permission-head">
        <ShieldQuestion size={14} />
        Permission required
      </div>
      <div className="permission-question">{question}</div>
      {params.toolCall?.title ? <div className="permission-tool">{params.toolCall.title}</div> : null}
      {toolPreview ? (
        <pre className="tool-pre" style={{ marginTop: 8, maxHeight: 140 }}>
          {toolPreview.slice(0, 1600)}
        </pre>
      ) : null}
      <div className="permission-actions">
        {options.map((option, index) => {
          const reject = option.kind?.startsWith('reject');
          const primary = index === 0 && !reject;
          return (
            <button
              key={option.optionId}
              className={`btn${primary ? ' primary' : reject ? ' danger' : ''}`}
              onClick={() => void respondPermission(request.requestId, option.optionId)}
            >
              {permissionLabel(option, index)}
            </button>
          );
        })}
      </div>
    </div>
  );
}
