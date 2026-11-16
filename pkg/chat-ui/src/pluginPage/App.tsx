import { useEffect, useMemo, useRef, useState } from 'react';
import { pluginPageHost } from './host';
import {
  pageKey,
  type PluginPage,
  type PluginPageAction,
  type PluginPageField,
  type PluginPageIncoming,
  type PluginPageItem,
  type PluginPageSection,
} from './types';

interface FormState {
  title: string;
  description?: string;
  actionId: string;
  fields: PluginPageField[];
  extra?: Record<string, string>;
}

interface ConfirmState {
  message: string;
  actionId: string;
  values: Record<string, string>;
  label: string;
}

interface ToastState {
  kind: 'success' | 'error';
  text: string;
}

const TOOLBAR_ACTIONS = new Set(['start', 'open', 'stop', 'sync', 'card.add', 'add', 'enable', 'disable', 'reload']);

let sequence = 0;
function requestId(): string {
  sequence += 1;
  return `page-${sequence}`;
}

export function App() {
  const [pages, setPages] = useState<PluginPage[]>([]);
  const [active, setActive] = useState<PluginPage | null>(null);
  const [busy, setBusy] = useState<string | null>(null);
  const [error, setError] = useState('');
  const [toast, setToast] = useState<ToastState | null>(null);
  const [form, setForm] = useState<FormState | null>(null);
  const [confirm, setConfirm] = useState<ConfirmState | null>(null);
  const [values, setValues] = useState<Record<string, string>>({});
  const [settings, setSettings] = useState<Record<string, string>>({});
  const [settingsOpen, setSettingsOpen] = useState(false);
  const activeRef = useRef<PluginPage | null>(null);

  useEffect(() => {
    activeRef.current = active;
    setSettings(initialValues(active?.settings ?? []));
  }, [active]);

  useEffect(() => {
    if (!toast) return;
    const timer = window.setTimeout(() => setToast(null), 3600);
    return () => window.clearTimeout(timer);
  }, [toast]);

  useEffect(() => {
    const refresh = () => {
      const page = activeRef.current;
      if (!page) return;
      pluginPageHost.postMessage({ type: 'get', requestId: requestId(), owner: page.owner, pageId: page.id });
    };

    const onMessage = (event: MessageEvent) => {
      const message = event.data as PluginPageIncoming | undefined;
      if (!message || typeof message.type !== 'string') return;
      switch (message.type) {
        case 'init': {
          setBusy(null);
          setPages(message.pages);
          if (message.page) {
            setActive(message.page);
            setPages((current) => upsert(current, message.page!));
          } else {
            setActive((current) => current ?? message.pages[0] ?? null);
          }
          break;
        }
        case 'pages':
          setBusy(null);
          setPages(message.pages);
          break;
        case 'page':
          setBusy(null);
          setActive(message.page);
          setPages((current) => upsert(current, message.page));
          break;
        case 'actionResult':
          setBusy(null);
          if (message.error) {
            setError(message.error);
            setToast({ kind: 'error', text: message.error });
            break;
          }
          setError('');
          if (message.page) {
            setActive(message.page);
            setPages((current) => upsert(current, message.page!));
          } else {
            refresh();
          }
          if (message.output) setToast({ kind: 'success', text: message.output });
          break;
        case 'error':
          setBusy(null);
          setError(message.error);
          setToast({ kind: 'error', text: message.error });
          break;
      }
    };

    window.addEventListener('message', onMessage);
    pluginPageHost.postMessage({ type: 'ready' });
    return () => window.removeEventListener('message', onMessage);
  }, []);

  const run = (actionId: string, input: Record<string, string>) => {
    const page = activeRef.current;
    if (!page) return;
    setBusy(actionId);
    setError('');
    pluginPageHost.postMessage({
      type: 'action',
      requestId: requestId(),
      owner: page.owner,
      pageId: page.id,
      actionId,
      values: input,
    });
    setToast(null);
  };

  const beginAction = (action: PluginPageAction) => {
    setError('');
    const input = action.fields?.length ? initialValues(action.fields) : {};
    if (action.confirm) {
      setConfirm({ message: action.description || `Run ${action.label}?`, actionId: action.id, values: input, label: action.label });
      return;
    }
    if (action.fields?.length) {
      setValues(input);
      setForm({ title: action.label, description: action.description, actionId: action.id, fields: action.fields });
      return;
    }
    run(action.id, {});
  };

  const beginItem = (item: PluginPageItem, actionId?: string, extra?: Record<string, string>) => {
    const id = actionId || item.action;
    if (!id) return;
    setError('');
    if (item.fields?.length) {
      const input = initialValues(item.fields);
      setValues(input);
      setForm({
        title: item.title,
        description: item.detail,
        actionId: id,
        fields: item.fields,
        extra: { ...(item.id ? { id: item.id } : {}), ...(extra ?? {}) },
      });
      return;
    }
    run(id, { ...(item.id ? { id: item.id } : {}), ...(extra ?? {}) });
  };

  const moveItem = (item: PluginPageItem, column: string) => {
    if (!item.action) return;
    const input = initialValues(item.fields ?? []);
    input.column = column;
    run(item.action, { ...(item.id ? { id: item.id } : {}), ...input });
  };

  const saveSettings = () => {
    const page = activeRef.current;
    if (!page?.saveAction) return;
    run(page.saveAction, settings);
  };

  const refresh = () => {
    const page = activeRef.current;
    setError('');
    if (!page) {
      pluginPageHost.postMessage({ type: 'list', requestId: requestId() });
      return;
    }
    setBusy('__refresh');
    pluginPageHost.postMessage({ type: 'refresh', requestId: requestId(), owner: page.owner, pageId: page.id });
  };

  const selectPage = (key: string) => {
    const next = pages.find((page) => pageKey(page) === key);
    if (!next) return;
    setActive(next);
    pluginPageHost.postMessage({ type: 'get', requestId: requestId(), owner: next.owner, pageId: next.id });
  };

  const toolbar = active?.actions.filter((action) => TOOLBAR_ACTIONS.has(action.id)) ?? [];
  const boardSections = active?.sections.filter((section) => section.kind === 'board') ?? [];
  const channelSections = active?.sections.filter((section) => section.kind === 'channels') ?? [];
  const listSections = active?.sections.filter((section) => section.kind !== 'board' && section.kind !== 'channels') ?? [];

  return (
    <div className="pp-root">
      <div className="pp-head">
        <div className="pp-head-main">
          <h1 className="pp-title">{active?.title ?? 'Plugin pages'}</h1>
          {active?.description ? <p className="pp-desc">{active.description}</p> : null}
        </div>
        <div className="pp-toolbar">
          {toolbar.map((action) => (
            <button
              key={action.id}
              className={`pp-btn${action.id === 'card.add' || action.id === 'add' ? ' primary' : ''}`}
              disabled={busy === action.id}
              onClick={() => beginAction(action)}
            >
              {action.id === 'card.add' || action.id === 'add' ? <PlusIcon /> : <PlayIcon />}
              {busy === action.id ? 'Running…' : action.label}
            </button>
          ))}
          {active?.settings?.length ? (
            <button className="pp-btn ghost" onClick={() => setSettingsOpen(true)}>
              <SettingsIcon />
              Settings
            </button>
          ) : null}
          <button className="pp-btn ghost" onClick={refresh} disabled={busy === '__refresh'}>
            <RefreshIcon spin={busy === '__refresh'} />
            Refresh
          </button>
        </div>
      </div>

      {pages.length > 1 && active ? (
        <div className="pp-switcher">
          <label htmlFor="pp-page-select">Page</label>
          <select id="pp-page-select" className="pp-select" value={pageKey(active)} onChange={(event) => selectPage(event.target.value)}>
            {pages.map((page) => (
              <option key={pageKey(page)} value={pageKey(page)}>
                {page.owner} · {page.title}
              </option>
            ))}
          </select>
        </div>
      ) : null}

      {error ? (
        <div className="pp-notice error" role="alert">
          <span>{error}</span>
          <button className="pp-icon" aria-label="Dismiss" onClick={() => setError('')}>
            <CloseIcon />
          </button>
        </div>
      ) : null}

      {!active ? (
        <div className="pp-empty">
          <p>No plugin pages are available.</p>
          <p className="pp-muted">Install a plugin that provides a page, then refresh.</p>
          <button className="pp-btn" onClick={refresh}>
            <RefreshIcon />
            Refresh
          </button>
        </div>
      ) : null}

      {active?.metrics?.length ? (
        <div className="pp-metrics">
          {active.metrics.map((metric) => (
            <div className="pp-metric" key={metric.label}>
              <span className="pp-metric-label">{metric.label}</span>
              <span className="pp-metric-value">{metric.value}</span>
              {metric.detail ? <span className="pp-metric-detail">{metric.detail}</span> : null}
              {metric.badge ? <span className="pp-badge">{metric.badge}</span> : null}
            </div>
          ))}
        </div>
      ) : null}

      {boardSections.map((section) => (
        <BoardSection key={section.title} section={section} busy={busy} onEdit={beginItem} onRun={beginItem} onMove={moveItem} />
      ))}

      {channelSections.map((section) => (
        <ChannelSection key={section.title} section={section} busy={busy} onEdit={beginItem} onRun={beginItem} />
      ))}

      {listSections.map((section) => (
        <section className="pp-panel" key={section.title}>
          <div className="pp-panel-head">{section.title}</div>
          {section.note ? <div className="pp-note">{section.note}</div> : null}
          {section.items.length ? (
            <div className="pp-list">
              {section.items.map((item, index) => (
                <div className={`pp-row${item.disabled ? ' disabled' : ''}`} key={`${section.title}-${item.id || item.title}-${index}`}>
                  <div className="pp-row-main">
                    <div className="pp-row-name">{item.title}</div>
                    {item.detail ? <div className="pp-row-desc">{item.detail}</div> : null}
                  </div>
                  <div className="pp-row-side">
                    {item.badge ? <span className="pp-badge">{item.badge}</span> : null}
                    {item.fields?.length && item.action ? (
                      <button className="pp-btn ghost" onClick={() => beginItem(item)}>
                        <PencilIcon />
                        Edit
                      </button>
                    ) : null}
                  </div>
                </div>
              ))}
            </div>
          ) : (
            <div className="pp-empty-inline">Nothing to show.</div>
          )}
        </section>
      ))}

      {active?.footer ? <div className="pp-footer">{active.footer}</div> : null}

      {settingsOpen && active?.settings?.length ? (
        <div className="pp-overlay" onMouseDown={(event) => event.target === event.currentTarget && setSettingsOpen(false)}>
          <div className="pp-modal" role="dialog" aria-modal="true" aria-label={`${active.title} settings`}>
            <div className="pp-modal-head">
              <span className="pp-modal-title">{active.title} settings</span>
              <button className="pp-icon" aria-label="Close" onClick={() => setSettingsOpen(false)}>
                <CloseIcon />
              </button>
            </div>
            <div className="pp-modal-body">
              <div className="pp-grid">
                {active.settings.map((field) => (
                  <label className="pp-field" key={field.id}>
                    <span className="pp-field-label">
                      {field.label || field.id}
                      {field.required ? ' *' : ''}
                    </span>
                    <FieldInput
                      field={field}
                      value={settings[field.id] ?? ''}
                      onChange={(value) => setSettings((current) => ({ ...current, [field.id]: value }))}
                    />
                  </label>
                ))}
              </div>
              <div className="pp-actions">
                <button className="pp-btn ghost" onClick={() => setSettingsOpen(false)}>
                  Cancel
                </button>
                <button
                  className="pp-btn primary"
                  disabled={busy === active.saveAction}
                  onClick={() => {
                    setSettingsOpen(false);
                    saveSettings();
                  }}
                >
                  <CheckIcon />
                  {busy === active.saveAction ? 'Saving…' : 'Save'}
                </button>
              </div>
            </div>
          </div>
        </div>
      ) : null}

      {form ? (
        <div className="pp-overlay" onMouseDown={(event) => event.target === event.currentTarget && setForm(null)}>
          <div className="pp-modal" role="dialog" aria-modal="true" aria-label={form.title}>
            <div className="pp-modal-head">
              <span className="pp-modal-title">{form.title}</span>
              <button className="pp-icon" aria-label="Close" onClick={() => setForm(null)}>
                <CloseIcon />
              </button>
            </div>
            <div className="pp-modal-body">
              {form.description ? <p className="pp-desc">{form.description}</p> : null}
              <div className="pp-grid">
                {form.fields.map((field) => (
                  <label className="pp-field" key={field.id}>
                    <span className="pp-field-label">
                      {field.label || field.id}
                      {field.required ? ' *' : ''}
                    </span>
                    <FieldInput
                      field={field}
                      value={values[field.id] ?? ''}
                      onChange={(value) => setValues((current) => ({ ...current, [field.id]: value }))}
                    />
                  </label>
                ))}
              </div>
              <div className="pp-actions">
                <button className="pp-btn ghost" onClick={() => setForm(null)}>
                  Cancel
                </button>
                <button
                  className="pp-btn primary"
                  disabled={busy === form.actionId}
                  onClick={() => {
                    const current = form;
                    setForm(null);
                    run(current.actionId, { ...(current.extra ?? {}), ...values });
                  }}
                >
                  <CheckIcon />
                  {busy === form.actionId ? 'Saving…' : 'Save'}
                </button>
              </div>
            </div>
          </div>
        </div>
      ) : null}

      {confirm ? (
        <div className="pp-overlay" onMouseDown={(event) => event.target === event.currentTarget && setConfirm(null)}>
          <div className="pp-modal" role="dialog" aria-modal="true" aria-label={confirm.label}>
            <div className="pp-modal-head">
              <span className="pp-modal-title">{confirm.label}</span>
              <button className="pp-icon" aria-label="Close" onClick={() => setConfirm(null)}>
                <CloseIcon />
              </button>
            </div>
            <div className="pp-modal-body">
              <p className="pp-desc">{confirm.message}</p>
              <div className="pp-actions">
                <button className="pp-btn ghost" onClick={() => setConfirm(null)}>
                  Cancel
                </button>
                <button
                  className="pp-btn primary"
                  onClick={() => {
                    const current = confirm;
                    setConfirm(null);
                    run(current.actionId, current.values);
                  }}
                >
                  <CheckIcon />
                  Confirm
                </button>
              </div>
            </div>
          </div>
        </div>
      ) : null}

      {toast ? (
        <div className={`pp-toast ${toast.kind}`} role="status">
          {toast.text}
        </div>
      ) : null}
    </div>
  );
}

function BoardSection({
  section,
  busy,
  onEdit,
  onRun,
  onMove,
}: {
  section: PluginPageSection;
  busy: string | null;
  onEdit: (item: PluginPageItem, actionId?: string, extra?: Record<string, string>) => void;
  onRun: (item: PluginPageItem, actionId?: string, extra?: Record<string, string>) => void;
  onMove: (item: PluginPageItem, column: string) => void;
}) {
  const columns = useMemo(() => {
    const map = new Map<string, PluginPageItem[]>();
    for (const item of section.items) {
      const key = item.column || 'Cards';
      const list = map.get(key) || [];
      list.push(item);
      map.set(key, list);
    }
    return Array.from(map.entries());
  }, [section.items]);

  const columnNames = useMemo(() => columns.map(([column]) => column), [columns]);

  return (
    <section className="pp-panel">
      <div className="pp-panel-head">{section.title}</div>
      {section.note ? <div className="pp-note">{section.note}</div> : null}
      <div className="pp-board">
        {columns.map(([column, items]) => (
          <div className="pp-column" key={column}>
            <div className="pp-column-head">
              <span>{column}</span>
              <span className="pp-badge">{items.length}</span>
            </div>
            <div className="pp-cards">
              {items.map((item) => (
                <article className={`pp-card${item.disabled ? ' disabled' : ''}`} key={item.id || item.title}>
                  <div className="pp-card-title">{item.title}</div>
                  {item.detail ? <div className="pp-card-detail">{item.detail}</div> : null}
                  <div className="pp-card-meta">
                    {item.badge ? <span className="pp-badge">{item.badge}</span> : null}
                    {item.meta
                      ? Object.entries(item.meta)
                          .filter(([key]) => !['runAction', 'deleteAction', 'testAction', 'editAction'].includes(key))
                          .map(([key, value]) => (
                            <span className="pp-badge muted" key={key}>
                              {value}
                            </span>
                          ))
                      : null}
                  </div>
                  <CardActions item={item} busy={busy} onEdit={onEdit} onRun={onRun} onMove={onMove} columnNames={columnNames} />
                </article>
              ))}
            </div>
          </div>
        ))}
      </div>
    </section>
  );
}

function CardActions({
  item,
  busy,
  onEdit,
  onRun,
  onMove,
  columnNames,
}: {
  item: PluginPageItem;
  busy: string | null;
  onEdit: (item: PluginPageItem) => void;
  onRun: (item: PluginPageItem, actionId?: string, extra?: Record<string, string>) => void;
  onMove: (item: PluginPageItem, column: string) => void;
  columnNames: string[];
}) {
  const columnField = item.fields?.find((field) => field.id === 'column');
  const currentColumn = columnField?.value || item.column || '';
  const options = columnField?.options?.length ? columnField.options : columnNames;
  const merged = options.includes(currentColumn) || !currentColumn ? options : [currentColumn, ...options];

  return (
    <div className="pp-card-actions">
      {columnField && item.action ? (
        <label className="pp-move">
          <span className="pp-sr">Move {item.title}</span>
          <select
            className="pp-select small"
            value={currentColumn}
            disabled={busy === item.action}
            onChange={(event) => onMove(item, event.target.value)}
          >
            {!currentColumn ? <option value="">Move…</option> : null}
            {merged.map((column) => (
              <option key={column} value={column}>
                {column}
              </option>
            ))}
          </select>
        </label>
      ) : null}
      {item.fields?.length && item.action ? (
        <button className="pp-btn small ghost" onClick={() => onEdit(item)}>
          <PencilIcon />
          Edit
        </button>
      ) : null}
      {item.meta?.runAction ? (
        <button
          className="pp-btn small"
          disabled={busy === item.meta.runAction}
          onClick={() => onRun(item, item.meta?.runAction)}
        >
          <PlayIcon />
          Run
        </button>
      ) : null}
      {item.meta?.deleteAction ? (
        <button
          className="pp-btn small danger"
          disabled={busy === item.meta.deleteAction}
          onClick={() => onRun(item, item.meta?.deleteAction)}
        >
          <TrashIcon />
          Delete
        </button>
      ) : null}
    </div>
  );
}

function ChannelSection({
  section,
  busy,
  onEdit,
  onRun,
}: {
  section: PluginPageSection;
  busy: string | null;
  onEdit: (item: PluginPageItem, actionId?: string, extra?: Record<string, string>) => void;
  onRun: (item: PluginPageItem, actionId?: string, extra?: Record<string, string>) => void;
}) {
  return (
    <section className="pp-panel">
      <div className="pp-panel-head">{section.title}</div>
      {section.note ? <div className="pp-note">{section.note}</div> : null}
      <div className="pp-channels">
        {section.items.map((item) => (
          <div className="pp-channel" key={item.id || item.title}>
            <div className="pp-channel-head">
              <span className={`pp-dot${item.badge === 'ready' ? ' ok' : ''}`} />
              <span className="pp-channel-name">{item.title}</span>
              {item.badge ? <span className="pp-badge">{item.badge}</span> : null}
            </div>
            {item.detail ? <div className="pp-channel-detail">{item.detail}</div> : null}
            <div className="pp-card-actions">
              {item.meta?.testAction ? (
                <button
                  className="pp-btn small"
                  disabled={busy === item.meta.testAction}
                  onClick={() => onRun(item, item.meta?.testAction, { channel: item.id ?? item.title })}
                >
                  <PlayIcon />
                  Test
                </button>
              ) : null}
              {item.fields?.length && item.action ? (
                <button className="pp-btn small ghost" onClick={() => onEdit(item)}>
                  <PencilIcon />
                  Edit
                </button>
              ) : null}
              {item.meta?.deleteAction ? (
                <button
                  className="pp-btn small danger"
                  disabled={busy === item.meta.deleteAction}
                  onClick={() => onRun(item, item.meta?.deleteAction, { name: item.id ?? item.title })}
                >
                  <TrashIcon />
                  Remove
                </button>
              ) : null}
            </div>
          </div>
        ))}
      </div>
    </section>
  );
}

function initialValues(fields: PluginPageField[]): Record<string, string> {
  const out: Record<string, string> = {};
  for (const field of fields) out[field.id] = field.value ?? '';
  return out;
}

function upsert(pages: PluginPage[], page: PluginPage): PluginPage[] {
  const key = pageKey(page);
  const index = pages.findIndex((candidate) => pageKey(candidate) === key);
  if (index < 0) return [...pages, page];
  const next = pages.slice();
  next[index] = page;
  return next;
}

function FieldInput({
  field,
  value,
  onChange,
}: {
  field: PluginPageField;
  value: string;
  onChange: (value: string) => void;
}) {
  if (field.options?.length) {
    return (
      <select className="pp-select" value={value} onChange={(event) => onChange(event.target.value)}>
        <option value="">Select…</option>
        {field.options.map((option) => (
          <option key={option} value={option}>
            {option}
          </option>
        ))}
      </select>
    );
  }
  return (
    <input
      className="pp-input"
      type={field.secret ? 'password' : 'text'}
      placeholder={field.placeholder}
      value={value}
      onChange={(event) => onChange(event.target.value)}
    />
  );
}

function PlusIcon() {
  return (
    <svg viewBox="0 0 16 16" width="13" height="13" aria-hidden="true">
      <path d="M8 3v10M3 8h10" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" fill="none" />
    </svg>
  );
}

function PlayIcon() {
  return (
    <svg viewBox="0 0 16 16" width="12" height="12" aria-hidden="true">
      <path d="M5 3.5 12 8l-7 4.5z" fill="currentColor" />
    </svg>
  );
}

function RefreshIcon({ spin }: { spin?: boolean }) {
  return (
    <svg className={spin ? 'pp-spin' : undefined} viewBox="0 0 16 16" width="13" height="13" aria-hidden="true">
      <path
        d="M12.5 6a5 5 0 1 0 .1 3.4M12.5 2.8V6H9.3"
        fill="none"
        stroke="currentColor"
        strokeWidth="1.4"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  );
}

function SettingsIcon() {
  return (
    <svg viewBox="0 0 16 16" width="13" height="13" aria-hidden="true">
      <circle cx="8" cy="8" r="2.2" fill="none" stroke="currentColor" strokeWidth="1.3" />
      <path
        d="M8 1.8v2M8 12.2v2M2.4 8h2M11.6 8h2M3.9 3.9l1.4 1.4M10.7 10.7l1.4 1.4M12.1 3.9l-1.4 1.4M5.3 10.7l-1.4 1.4"
        stroke="currentColor"
        strokeWidth="1.3"
        strokeLinecap="round"
      />
    </svg>
  );
}

function CloseIcon() {
  return (
    <svg viewBox="0 0 16 16" width="14" height="14" aria-hidden="true">
      <path d="M4 4l8 8M12 4l-8 8" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" fill="none" />
    </svg>
  );
}

function CheckIcon() {
  return (
    <svg viewBox="0 0 16 16" width="13" height="13" aria-hidden="true">
      <path d="M3.5 8.4 6.6 11.5 12.5 4.8" stroke="currentColor" strokeWidth="1.6" fill="none" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

function PencilIcon() {
  return (
    <svg viewBox="0 0 16 16" width="12" height="12" aria-hidden="true">
      <path d="m10.9 2.6 2.5 2.5-7.6 7.6-3.3.8.8-3.3z" fill="none" stroke="currentColor" strokeWidth="1.3" strokeLinejoin="round" />
    </svg>
  );
}

function TrashIcon() {
  return (
    <svg viewBox="0 0 16 16" width="12" height="12" aria-hidden="true">
      <path d="M3 4.5h10M6.5 4.5V3h3v1.5M4.5 4.5l.6 8h5.8l.6-8" fill="none" stroke="currentColor" strokeWidth="1.3" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}
