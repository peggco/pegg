import { useEffect, useMemo, useRef, useState, type KeyboardEvent } from 'react';
import { Check, ChevronLeft, ChevronRight, Plus, UserRound, X } from 'lucide-react';
import { respondAsk } from '../state/actions';
import { useT } from '../lib/i18n';
import type { PendingAsk } from '../state/chat';
import type { AskQuestion } from '../types';
import {
  MULTI_SEPARATOR,
  firstMissingRequired,
  hasCustomOption,
  isAnswered,
  multiselectValues,
  questionType,
  serializeAnswers,
  tabLabel,
  toggleMultiselect,
  type QuestionType,
} from '../lib/ask';

function hintKey(type: QuestionType, review: boolean): string {
  if (review) return 'ask.hintReview';
  return type === 'text' ? 'ask.hintText' : 'ask.hintOptions';
}

export function AskPanel({ ask }: { ask: PendingAsk }) {
  const t = useT();
  const questions = ask.params.questions ?? [];
  const total = questions.length;
  const [answers, setAnswers] = useState<Record<string, string>>(() => ({ ...(ask.answers ?? {}) }));
  const [custom, setCustom] = useState<Record<string, string>>({});
  const [tab, setTab] = useState(0);
  const [review, setReview] = useState(false);
  const [editing, setEditing] = useState<string | null>(null);
  const [customDraft, setCustomDraft] = useState('');
  const [error, setError] = useState<string | null>(null);
  const panelRef = useRef<HTMLDivElement>(null);

  const question = questions[Math.min(tab, Math.max(0, total - 1))];
  const type = question ? questionType(question) : 'text';
  const options = question?.options ?? [];
  const answer = question ? answers[question.id] ?? '' : '';
  const customValue = question ? custom[question.id] ?? '' : '';
  const isTextInput = type === 'text' || ((type === 'select' || type === 'multiselect') && options.length === 0);
  const customSelected =
    isAnswered(customValue) &&
    (type === 'multiselect' ? multiselectValues(answer).includes(customValue) : answer === customValue);
  const answeredCount = useMemo(
    () => questions.filter((item) => isAnswered(answers[item.id])).length,
    [questions, answers],
  );

  useEffect(() => {
    const root = panelRef.current;
    if (!root) return;
    const input = root.querySelector<HTMLElement>('textarea, input');
    const target = input ?? root.querySelector<HTMLElement>('.ask-option, .ask-footer .btn.primary');
    target?.focus();
  }, [tab, editing, review]);

  if (!question) return null;

  const setAnswer = (id: string, value: string) => {
    setAnswers((previous) => ({ ...previous, [id]: value }));
    setError(null);
  };

  const submit = (values: Record<string, string> = answers) => {
    const missing = firstMissingRequired(questions, values);
    if (missing >= 0) {
      setTab(missing);
      setReview(false);
      setError(t('ask.required'));
      return;
    }
    void respondAsk(ask.requestId, serializeAnswers(questions, values));
  };

  const advance = (values: Record<string, string> = answers) => {
    if (question.required && !isAnswered(values[question.id])) {
      setError(t('ask.required'));
      return;
    }
    if (total <= 1) {
      submit(values);
      return;
    }
    setEditing(null);
    setCustomDraft('');
    if (tab >= total - 1) {
      setReview(true);
      setError(null);
      return;
    }
    setTab(tab + 1);
    setError(null);
  };

  const back = () => {
    if (editing) {
      setEditing(null);
      setError(null);
      return;
    }
    if (review) {
      setReview(false);
      return;
    }
    if (tab > 0) {
      setTab(tab - 1);
      setError(null);
    }
  };

  const cancel = () => {
    void respondAsk(ask.requestId, {});
  };

  const moveTab = (delta: number) => {
    if (total <= 1) return;
    setTab((tab + delta + total) % total);
    setReview(false);
    setEditing(null);
    setError(null);
  };

  const openCustom = (current: AskQuestion) => {
    setEditing(current.id);
    setCustomDraft(custom[current.id] ?? '');
    setError(null);
  };

  const commitCustom = () => {
    const value = customDraft.trim();
    if (!value) {
      setError(t('ask.customEmpty'));
      return;
    }
    const previous = custom[question.id] ?? '';
    setCustom((state) => ({ ...state, [question.id]: value }));
    setEditing(null);
    setCustomDraft('');
    setError(null);
    if (type === 'multiselect') {
      let values = multiselectValues(answer);
      if (previous) values = values.filter((item) => item !== previous);
      if (!values.includes(value)) values = [...values, value];
      setAnswer(question.id, values.join(MULTI_SEPARATOR));
      return;
    }
    const values = { ...answers, [question.id]: value };
    setAnswers(values);
    advance(values);
  };

  const removeCustom = () => {
    const previous = custom[question.id] ?? '';
    if (previous) {
      setAnswer(
        question.id,
        multiselectValues(answer)
          .filter((item) => item !== previous)
          .join(MULTI_SEPARATOR),
      );
    }
    setEditing(null);
    setCustomDraft('');
    setError(null);
  };

  const onKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
    const target = event.target as HTMLElement;
    const typing = target.tagName === 'INPUT' || target.tagName === 'TEXTAREA';
    if (event.key === 'Escape') {
      event.preventDefault();
      if (editing) {
        setEditing(null);
        setError(null);
      } else if (review) {
        setReview(false);
      } else {
        cancel();
      }
      return;
    }
    if (event.altKey && event.key === 'ArrowLeft') {
      event.preventDefault();
      moveTab(-1);
      return;
    }
    if (event.altKey && event.key === 'ArrowRight') {
      event.preventDefault();
      moveTab(1);
      return;
    }
    // 1-9: n. seçeneği seç
    if (!typing && !review && !isTextInput && /^[1-9]$/.test(event.key)) {
      const buttons = panelRef.current?.querySelectorAll<HTMLButtonElement>('.ask-option:not(.custom)');
      const hit = buttons?.[Number(event.key) - 1];
      if (hit) {
        event.preventDefault();
        hit.click();
        hit.focus();
      }
      return;
    }
    if (!typing && !review && (event.key === 'ArrowDown' || event.key === 'ArrowUp')) {
      const buttons = Array.from(panelRef.current?.querySelectorAll<HTMLButtonElement>('.ask-option') ?? []);
      if (!buttons.length) return;
      const current = buttons.indexOf(document.activeElement as HTMLButtonElement);
      const delta = event.key === 'ArrowDown' ? 1 : -1;
      const next = current < 0 ? 0 : (current + delta + buttons.length) % buttons.length;
      event.preventDefault();
      buttons[next].focus();
    }
  };

  const title = ask.params.agentName ? t('ask.agentAsks', { name: ask.params.agentName }) : t('ask.title');
  const primaryLabel = review || total === 1 ? t('ask.submit') : tab >= total - 1 ? t('ask.review') : t('ask.next');
  const showArrow = !review && total > 1;
  const onPrimary = () => (review ? submit() : advance());
  const markKind = type === 'multiselect' ? 'check' : 'radio';

  return (
    <div className="ask-panel" role="dialog" aria-label={title} onKeyDown={onKeyDown} ref={panelRef}>
      <div className="ask-panel-head">
        <span className="ask-panel-title">
          <UserRound size={14} />
          {title}
        </span>
        {total > 1 ? (
          <span className="ask-progress">
            {answeredCount}/{total}
          </span>
        ) : null}
        <span className="spacer" />
        <button className="icon-btn" aria-label={t('ask.cancel')} title={t('ask.cancel')} onClick={cancel}>
          <X size={15} />
        </button>
      </div>

      {total > 1 ? (
        <div className="ask-steps" role="tablist" aria-label={t('ask.title')}>
          {questions.map((item, index) => {
            const done = isAnswered(answers[item.id]);
            const active = index === tab && !review;
            return (
              <button
                key={item.id}
                role="tab"
                aria-selected={active}
                title={item.question}
                className={`ask-step${active ? ' active' : ''}${done ? ' done' : ''}`}
                onClick={() => {
                  setTab(index);
                  setReview(false);
                  setEditing(null);
                  setError(null);
                }}
              >
                <span className="ask-step-bar" />
                <span className="ask-step-label">
                  <span className="ask-step-num">{done ? <Check size={11} /> : index + 1}</span>
                  <span className="ask-step-text">{tabLabel(item)}</span>
                </span>
              </button>
            );
          })}
        </div>
      ) : null}

      {review ? (
        <div className="ask-review">
          <div className="ask-review-title">{t('ask.reviewTitle')}</div>
          {questions.map((item, index) => (
            <div className="ask-review-row" key={item.id}>
              <span className="ask-review-q">{item.question}</span>
              <span className={`ask-review-a${isAnswered(answers[item.id]) ? '' : ' empty'}`}>
                {isAnswered(answers[item.id]) ? answers[item.id] : t('ask.skipped')}
              </span>
              <button
                className="ask-review-edit"
                onClick={() => {
                  setTab(index);
                  setReview(false);
                  setError(null);
                }}
              >
                {t('ask.edit')}
              </button>
            </div>
          ))}
        </div>
      ) : (
        <div className="ask-body">
          <div className="ask-question-text">{question.question}</div>

          {isTextInput ? (
            <textarea
              className="ask-input ask-textarea"
              rows={3}
              value={answer}
              placeholder={t('ask.placeholder')}
              onChange={(event) => setAnswer(question.id, event.target.value)}
              onKeyDown={(event) => {
                if (event.key === 'Enter' && !event.shiftKey) {
                  event.preventDefault();
                  advance();
                }
              }}
            />
          ) : null}

          {!isTextInput && type === 'boolean' ? (
            <div className="ask-options">
              {[
                { label: t('ask.yes'), value: 'Yes' },
                { label: t('ask.no'), value: 'No' },
              ].map((option, index) => {
                const selected = answer === option.value;
                return (
                  <button
                    key={option.value}
                    className={`ask-option${selected ? ' selected' : ''}`}
                    onClick={() => setAnswer(question.id, option.value)}
                  >
                    <span className="ask-mark radio" />
                    <span className="ask-option-label">{option.label}</span>
                    <kbd className="ask-key">{index + 1}</kbd>
                  </button>
                );
              })}
            </div>
          ) : null}

          {!isTextInput && (type === 'select' || type === 'multiselect') ? (
            <div className="ask-options">
              {options.map((option, index) => {
                const selected =
                  type === 'multiselect' ? multiselectValues(answer).includes(option) : answer === option;
                return (
                  <button
                    key={option}
                    className={`ask-option${selected ? ' selected' : ''}`}
                    onClick={() =>
                      setAnswer(question.id, type === 'multiselect' ? toggleMultiselect(answer, option) : option)
                    }
                  >
                    <span className={`ask-mark ${markKind}`}>
                      {selected && markKind === 'check' ? <Check size={10} strokeWidth={3} /> : null}
                    </span>
                    <span className="ask-option-label">{option}</span>
                    {index < 9 ? <kbd className="ask-key">{index + 1}</kbd> : null}
                  </button>
                );
              })}

              {hasCustomOption(type) ? (
                editing === question.id ? (
                  <div className="ask-custom-editor">
                    <input
                      className="ask-input ask-custom-input"
                      value={customDraft}
                      placeholder={t('ask.customPlaceholder')}
                      onChange={(event) => setCustomDraft(event.target.value)}
                      onKeyDown={(event) => {
                        if (event.key === 'Enter') {
                          event.preventDefault();
                          commitCustom();
                        } else if (event.key === 'Escape') {
                          event.preventDefault();
                          event.stopPropagation();
                          setEditing(null);
                          setError(null);
                        }
                      }}
                    />
                    {type === 'multiselect' && isAnswered(customValue) ? (
                      <button className="btn" onClick={removeCustom}>
                        {t('ask.remove')}
                      </button>
                    ) : null}
                    <button
                      className="btn"
                      onClick={() => {
                        setEditing(null);
                        setError(null);
                      }}
                    >
                      {t('ask.cancel')}
                    </button>
                    <button className="btn primary" onClick={commitCustom}>
                      {t('ask.apply')}
                    </button>
                  </div>
                ) : (
                  <button
                    className={`ask-option custom${customSelected ? ' selected' : ''}`}
                    onClick={() => openCustom(question)}
                  >
                    <span className="ask-plus">
                      <Plus size={13} />
                    </span>
                    <span className="ask-option-label">
                      {isAnswered(customValue) ? `${t('ask.custom')}: ${customValue}` : t('ask.custom')}
                    </span>
                  </button>
                )
              ) : null}
            </div>
          ) : null}

          {error ? <div className="ask-error">{error}</div> : null}
        </div>
      )}

      <div className="ask-footer">
        <span className="ask-hint">{t(hintKey(type, review))}</span>
        <span className="spacer" />
        {review ? (
          <button className="btn" onClick={() => setReview(false)}>
            <ChevronLeft size={13} />
            {t('ask.back')}
          </button>
        ) : total > 1 ? (
          <button className="btn" onClick={back} disabled={tab === 0}>
            <ChevronLeft size={13} />
            {t('ask.back')}
          </button>
        ) : null}
        <button className="btn primary" onClick={onPrimary}>
          {primaryLabel}
          {showArrow ? <ChevronRight size={13} /> : null}
        </button>
      </div>
    </div>
  );
}