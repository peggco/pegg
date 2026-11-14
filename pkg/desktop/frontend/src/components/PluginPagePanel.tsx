import { useMemo, useState } from 'react';
import { Check, Pencil, Play, Plus, RefreshCw, Settings2, Trash2, X } from 'lucide-react';
import { api } from '../lib/bridge';
import { errorText } from '../state/actions';
import { store } from '../state/store';
import type {
  PluginSettingAction,
  PluginSettingField,
  PluginSettingItem,
  PluginSettingPage,
  PluginSettingSection,
} from '../types';
import { Card, Empty, Field } from './SettingsUI';

interface FormState {
  title: string;
  description?: string;
  actionId: string;
  fields: PluginSettingField[];
  extra?: Record<string, string>;
}

const TOOLBAR_ACTIONS = new Set(['start', 'open', 'stop', 'sync', 'card.add', 'add', 'enable', 'disable', 'reload']);

export function PluginPagePanel({
  page,
  onRefresh,
  onPage,
}: {
  page: PluginSettingPage;
  onRefresh: () => Promise<void>;
  onPage: (page: PluginSettingPage) => void;
}) {
  const [busy, setBusy] = useState<string | null>(null);
  const [error, setError] = useState('');
  const [form, setForm] = useState<FormState | null>(null);
  const [values, setValues] = useState<Record<string, string>>({});
  const [settings, setSettings] = useState<Record<string, string>>(() => initialValues(page.settings ?? []));
  const [settingsOpen, setSettingsOpen] = useState(false);

  const run = async (actionId: string, input: Record<string, string>, label: string) => {
    setBusy(actionId);
    setError('');
    try {
      const result = await api.PluginSettingsAction(page.owner, page.id, actionId, input);
      if (result.page) onPage(result.page);
      else await onRefresh();
      store.toast('success', result.output || `${label} completed`);
    } catch (err) {
      const message = errorText(err);
      setError(message);
      store.toast('error', message);
    } finally {
      setBusy(null);
    }
  };

  const beginAction = (action: PluginSettingAction) => {
    setError('');
    if (action.fields?.length) {
      setValues(initialValues(action.fields));
      setForm({ title: action.label, description: action.description, actionId: action.id, fields: action.fields });
      return;
    }
    void run(action.id, {}, action.label);
  };

  const beginItem = (item: PluginSettingItem, actionId?: string, extra?: Record<string, string>) => {
    const id = actionId || item.action;
    if (!id) return;
    setError('');
    if (item.fields?.length) {
      setValues(initialValues(item.fields));
      setForm({
        title: item.title,
        description: item.detail,
        actionId: id,
        fields: item.fields,
        extra: { ...(item.id ? { id: item.id } : {}), ...(extra ?? {}) },
      });
      return;
    }
    void run(id, { ...(item.id ? { id: item.id } : {}), ...(extra ?? {}) }, item.title);
  };

  const saveSettings = () => {
    if (!page.saveAction) return;
    void run(page.saveAction, settings, 'Settings');
  };

  const refresh = async () => {
    setBusy('__refresh');
    setError('');
    try {
      await onRefresh();
    } catch (err) {
      setError(errorText(err));
    } finally {
      setBusy(null);
    }
  };

  const toolbar = page.actions.filter((action) => TOOLBAR_ACTIONS.has(action.id));
  const boardSections = page.sections.filter((section) => section.kind === 'board');
  const channelSections = page.sections.filter((section) => section.kind === 'channels');
  const listSections = page.sections.filter((section) => section.kind !== 'board' && section.kind !== 'channels');

  return (
    <>
      <div className="settings-page-head plugin-page-head">
        <div>
          <h2 className="settings-page-title">{page.title}</h2>
          {page.description ? <p className="settings-page-desc">{page.description}</p> : null}
        </div>
        <div className="plugin-toolbar">
          {toolbar.map((action) => (
            <button
              key={action.id}
              className={`btn small${action.id === 'card.add' || action.id === 'add' ? ' primary' : ''}`}
              disabled={busy === action.id}
              onClick={() => beginAction(action)}
            >
              {action.id === 'card.add' || action.id === 'add' ? <Plus size={13} /> : <Play size={12} />}
              {busy === action.id ? 'Running…' : action.label}
            </button>
          ))}
          {page.settings?.length ? (
            <button className="btn small ghost" onClick={() => setSettingsOpen(true)}>
              <Settings2 size={13} />
              Settings
            </button>
          ) : null}
          <button className="btn small ghost" onClick={() => void refresh()} disabled={busy === '__refresh'}>
            <RefreshCw size={13} className={busy === '__refresh' ? 'spin' : undefined} />
            Refresh
          </button>
        </div>
      </div>

      {error ? <div className="notice error">{error}</div> : null}

      {page.metrics?.length ? (
        <div className="plugin-metrics">
          {page.metrics.map((metric) => (
            <div className="plugin-metric" key={metric.label}>
              <span className="plugin-metric-label">{metric.label}</span>
              <span className="plugin-metric-value">{metric.value}</span>
              {metric.detail ? <span className="plugin-metric-detail">{metric.detail}</span> : null}
              {metric.badge ? <span className="badge">{metric.badge}</span> : null}
            </div>
          ))}
        </div>
      ) : null}

      {boardSections.map((section) => (
        <BoardSection key={section.title} section={section} busy={busy} onEdit={beginItem} onRun={beginItem} />
      ))}

      {channelSections.map((section) => (
        <ChannelSection key={section.title} section={section} busy={busy} onEdit={beginItem} onRun={beginItem} />
      ))}

      {listSections.map((section) => (
        <Card key={section.title} title={section.title}>
          {section.note ? <div className="settings-footnote">{section.note}</div> : null}
          {section.items.length ? (
            <div className="settings-list">
              {section.items.map((item, index) => (
                <div
                  className={`settings-list-row${item.disabled ? ' disabled' : ''}`}
                  key={`${section.title}-${item.id || item.title}-${index}`}
                >
                  <div className="row-main">
                    <div className="row-name">{item.title}</div>
                    {item.detail ? <div className="row-desc">{item.detail}</div> : null}
                  </div>
                  <div className="row-side">
                    {item.badge ? <span className="badge">{item.badge}</span> : null}
                    {item.fields?.length && item.action ? (
                      <button className="btn small ghost" onClick={() => beginItem(item)}>
                        <Pencil size={12} />
                        Edit
                      </button>
                    ) : null}
                  </div>
                </div>
              ))}
            </div>
          ) : (
            <Empty>Nothing to show.</Empty>
          )}
        </Card>
      ))}

      {settingsOpen && page.settings?.length ? (
        <div className="overlay" onMouseDown={(event) => event.target === event.currentTarget && setSettingsOpen(false)}>
        <div className="modal plugin-modal" role="dialog" aria-modal="true" aria-label={`${page.title} settings`}>
          <div className="modal-head">
            <span className="modal-title">{page.title} settings</span>
            <div className="spacer" />
            <button className="icon-btn" aria-label="Close" onClick={() => setSettingsOpen(false)}>
              <X size={16} />
            </button>
          </div>
          <div className="modal-body">
            <div className="settings-grid">
              {page.settings.map((field) => (
                <Field key={field.id} label={`${field.label || field.id}${field.required ? ' *' : ''}`}>
                  <FieldInput
                    field={field}
                    value={settings[field.id] ?? ''}
                    onChange={(value) => setSettings((current) => ({ ...current, [field.id]: value }))}
                  />
                </Field>
              ))}
            </div>
            <div className="settings-save-bar">
              <span className="spacer" />
              <button className="btn small ghost" onClick={() => setSettingsOpen(false)}>
                Cancel
              </button>
              <button
                className="btn small primary"
                disabled={busy === page.saveAction}
                onClick={() => {
                  setSettingsOpen(false);
                  saveSettings();
                }}
              >
                <Check size={13} />
                {busy === page.saveAction ? 'Saving…' : 'Save'}
              </button>
            </div>
          </div>
        </div>
        </div>
      ) : null}

      {form ? (
        <div className="overlay" onMouseDown={(event) => event.target === event.currentTarget && setForm(null)}>
        <div className="modal plugin-modal" role="dialog" aria-modal="true" aria-label={form.title}>
          <div className="modal-head">
            <span className="modal-title">{form.title}</span>
            <div className="spacer" />
            <button className="icon-btn" aria-label="Close" onClick={() => setForm(null)}>
              <X size={16} />
            </button>
          </div>
          <div className="modal-body">
            {form.description ? <p className="settings-page-desc">{form.description}</p> : null}
            <div className="settings-grid">
              {form.fields.map((field) => (
                <Field key={field.id} label={`${field.label || field.id}${field.required ? ' *' : ''}`}>
                  <FieldInput
                    field={field}
                    value={values[field.id] ?? ''}
                    onChange={(value) => setValues((current) => ({ ...current, [field.id]: value }))}
                  />
                </Field>
              ))}
            </div>
            <div className="settings-save-bar">
              <span className="spacer" />
              <button className="btn small ghost" onClick={() => setForm(null)}>
                Cancel
              </button>
              <button
                className="btn small primary"
                disabled={busy === form.actionId}
                onClick={() => {
                  const current = form;
                  setForm(null);
                  void run(current.actionId, { ...(current.extra ?? {}), ...values }, current.title);
                }}
              >
                <Check size={13} />
                {busy === form.actionId ? 'Saving…' : 'Save'}
              </button>
            </div>
          </div>
        </div>
        </div>
      ) : null}
    </>
  );
}

function BoardSection({
  section,
  busy,
  onEdit,
  onRun,
}: {
  section: PluginSettingSection;
  busy: string | null;
  onEdit: (item: PluginSettingItem, actionId?: string, extra?: Record<string, string>) => void;
  onRun: (item: PluginSettingItem, actionId?: string, extra?: Record<string, string>) => void;
}) {
  const columns = useMemo(() => {
    const map = new Map<string, PluginSettingItem[]>();
    for (const item of section.items) {
      const key = item.column || 'Cards';
      const list = map.get(key) || [];
      list.push(item);
      map.set(key, list);
    }
    return Array.from(map.entries());
  }, [section.items]);

  return (
    <Card title={section.title}>
      {section.note ? <div className="settings-footnote">{section.note}</div> : null}
      <div className="plugin-board">
        {columns.map(([column, items]) => (
          <div className="plugin-board-column" key={column}>
            <div className="plugin-board-head">
              <span>{column}</span>
              <span className="badge">{items.length}</span>
            </div>
            <div className="plugin-board-cards">
              {items.map((item) => (
                <div className="plugin-card" key={item.id || item.title}>
                  <div className="plugin-card-title">{item.title}</div>
                  {item.detail ? <div className="plugin-card-detail">{item.detail}</div> : null}
                  <div className="plugin-card-meta">
                    {item.badge ? <span className="badge">{item.badge}</span> : null}
                    {item.meta
                      ? Object.entries(item.meta)
                          .filter(([key]) => !['runAction', 'deleteAction', 'testAction', 'editAction'].includes(key))
                          .map(([key, value]) => (
                            <span className="badge muted" key={key}>
                              {value}
                            </span>
                          ))
                      : null}
                  </div>
                  <div className="plugin-card-actions">
                    {item.fields?.length && item.action ? (
                      <button className="btn small ghost" onClick={() => onEdit(item)}>
                        <Pencil size={12} />
                        Edit
                      </button>
                    ) : null}
                    {item.meta?.runAction ? (
                      <button
                        className="btn small"
                        disabled={busy === item.meta.runAction}
                        onClick={() => onRun(item, item.meta?.runAction)}
                      >
                        <Play size={12} />
                        Run
                      </button>
                    ) : null}
                    {item.meta?.deleteAction ? (
                      <button
                        className="btn small danger"
                        disabled={busy === item.meta.deleteAction}
                        onClick={() => onRun(item, item.meta?.deleteAction)}
                      >
                        <Trash2 size={12} />
                        Delete
                      </button>
                    ) : null}
                  </div>
                </div>
              ))}
            </div>
          </div>
        ))}
      </div>
    </Card>
  );
}

function ChannelSection({
  section,
  busy,
  onEdit,
  onRun,
}: {
  section: PluginSettingSection;
  busy: string | null;
  onEdit: (item: PluginSettingItem, actionId?: string, extra?: Record<string, string>) => void;
  onRun: (item: PluginSettingItem, actionId?: string, extra?: Record<string, string>) => void;
}) {
  return (
    <Card title={section.title}>
      {section.note ? <div className="settings-footnote">{section.note}</div> : null}
      <div className="plugin-channel-grid">
        {section.items.map((item) => (
          <div className="plugin-channel" key={item.id || item.title}>
            <div className="plugin-channel-head">
              <span className={`plugin-dot${item.badge === 'ready' ? ' ok' : ''}`} />
              <span className="plugin-channel-name">{item.title}</span>
              {item.badge ? <span className="badge">{item.badge}</span> : null}
            </div>
            {item.detail ? <div className="plugin-channel-detail">{item.detail}</div> : null}
            <div className="plugin-card-actions">
              {item.meta?.testAction ? (
                <button
                  className="btn small"
                  disabled={busy === item.meta.testAction}
                  onClick={() => onRun(item, item.meta?.testAction, { channel: item.id ?? item.title })}
                >
                  <Play size={12} />
                  Test
                </button>
              ) : null}
              {item.fields?.length && item.action ? (
                <button className="btn small ghost" onClick={() => onEdit(item)}>
                  <Pencil size={12} />
                  Edit
                </button>
              ) : null}
              {item.meta?.deleteAction ? (
                <button
                  className="btn small danger"
                  disabled={busy === item.meta.deleteAction}
                  onClick={() => onRun(item, item.meta?.deleteAction, { name: item.id ?? item.title })}
                >
                  <Trash2 size={12} />
                  Remove
                </button>
              ) : null}
            </div>
          </div>
        ))}
      </div>
    </Card>
  );
}

function initialValues(fields: PluginSettingField[]): Record<string, string> {
  const out: Record<string, string> = {};
  for (const field of fields) out[field.id] = field.value ?? '';
  return out;
}

function FieldInput({
  field,
  value,
  onChange,
}: {
  field: PluginSettingField;
  value: string;
  onChange: (value: string) => void;
}) {
  if (field.options?.length) {
    return (
      <select className="select" value={value} onChange={(event) => onChange(event.target.value)}>
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
      className="input"
      type={field.secret ? 'password' : 'text'}
      placeholder={field.placeholder}
      value={value}
      onChange={(event) => onChange(event.target.value)}
    />
  );
}
