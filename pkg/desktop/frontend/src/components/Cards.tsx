import { useMemo, useState } from 'react';
import { Check, CircleDot, ListChecks, ShieldQuestion, Sparkles, UserRound } from 'lucide-react';
import type { PlanEntry } from '../types';
import type { PendingAsk, PendingPermission } from '../state/chat';
import type { ChatItem } from '../state/chat';
import { respondAsk, respondPermission } from '../state/actions';
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
          {item.status === 'running' ? 'working…' : item.status === 'error' ? 'failed' : 'finished'}
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
  const question = params.question || params.toolCall?.title || 'Allow this tool call?';
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

export function AskCard({ request }: { request: PendingAsk }) {
  const [answers, setAnswers] = useState<Record<string, string>>(request.answers ?? {});
  const questions = request.params.questions ?? [];
  const missing = questions.some((q) => q.required && !answers[q.id]);

  const set = (id: string, value: string) => setAnswers((prev) => ({ ...prev, [id]: value }));

  const toggleMulti = (id: string, option: string) => {
    const current = (answers[id] ?? '').split(', ').filter(Boolean);
    const next = current.includes(option) ? current.filter((v) => v !== option) : [...current, option];
    set(id, next.join(', '));
  };

  return (
    <div className="ask-card">
      <div className="ask-head">
        <UserRound size={14} />
        {request.params.agentName ? `${request.params.agentName} asks` : 'Pegg asks'}
      </div>
      {questions.map((question) => {
        const value = answers[question.id] ?? '';
        return (
          <div className="ask-question" key={question.id}>
            <div className="ask-label">{question.question}</div>
            {question.type === 'text' ? (
              <input
                className="ask-input"
                value={value}
                onChange={(event) => set(question.id, event.target.value)}
                placeholder="Type your answer…"
              />
            ) : null}
            {question.type === 'boolean' ? (
              <div className="ask-options">
                {['Yes', 'No'].map((option) => (
                  <button
                    key={option}
                    className={`ask-option${value === option ? ' selected' : ''}`}
                    onClick={() => set(question.id, option)}
                  >
                    {option}
                  </button>
                ))}
              </div>
            ) : null}
            {question.type === 'select' ? (
              <div className="ask-options">
                {(question.options ?? []).map((option) => (
                  <button
                    key={option}
                    className={`ask-option${value === option ? ' selected' : ''}`}
                    onClick={() => set(question.id, option)}
                  >
                    {option}
                  </button>
                ))}
              </div>
            ) : null}
            {question.type === 'multiselect' ? (
              <div className="ask-options">
                {(question.options ?? []).map((option) => {
                  const selected = value.split(', ').filter(Boolean).includes(option);
                  return (
                    <button
                      key={option}
                      className={`ask-option${selected ? ' selected' : ''}`}
                      onClick={() => toggleMulti(question.id, option)}
                    >
                      {option}
                    </button>
                  );
                })}
              </div>
            ) : null}
          </div>
        );
      })}
      <div className="permission-actions">
        <button className="btn primary" disabled={missing} onClick={() => void respondAsk(request.requestId, answers)}>
          Send answer
        </button>
      </div>
    </div>
  );
}
