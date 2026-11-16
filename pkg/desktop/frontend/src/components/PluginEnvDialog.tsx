import { useEffect, useRef, useState } from 'react';
import { Eye, EyeOff, Plus, Trash2, X } from 'lucide-react';
import { api } from '../lib/bridge';
import { useT } from '../lib/i18n';
import { store } from '../state/store';
import { Field } from './SettingsUI';

const envKeyPattern = /^[A-Za-z_][A-Za-z0-9_]*$/;

interface EnvRow {
  id: number;
  key: string;
  value: string;
  reveal: boolean;
}

export function PluginEnvDialog({
  name,
  onClose,
  onSaved,
}: {
  name: string;
  onClose: () => void;
  onSaved?: () => Promise<void> | void;
}) {
  const t = useT();
  const [rows, setRows] = useState<EnvRow[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');
  const nextID = useRef(0);

  useEffect(() => {
    let live = true;
    setLoading(true);
    api
      .PluginEnv(name)
      .then((env) => {
        if (!live) return;
        const entries = Object.entries(env ?? {}).sort(([a], [b]) => a.localeCompare(b));
        setRows(
          entries.map(([key, value]) => ({
            id: nextID.current++,
            key,
            value,
            reveal: false,
          })),
        );
      })
      .catch((err) => {
        if (live) setError(err instanceof Error ? err.message : String(err));
      })
      .finally(() => {
        if (live) setLoading(false);
      });
    return () => {
      live = false;
    };
  }, [name]);

  const addRow = () => {
    setRows((current) => [...current, { id: nextID.current++, key: '', value: '', reveal: false }]);
  };

  const removeRow = (id: number) => {
    setRows((current) => current.filter((row) => row.id !== id));
  };

  const updateRow = (id: number, patch: Partial<EnvRow>) => {
    setRows((current) => current.map((row) => (row.id === id ? { ...row, ...patch } : row)));
  };

  const save = async () => {
    const env: Record<string, string> = {};
    for (const row of rows) {
      const key = row.key.trim();
      if (!key) {
        if (row.value.trim()) {
          setError(t('pluginEnv.keyRequired'));
          return;
        }
        continue;
      }
      if (!envKeyPattern.test(key)) {
        setError(t('pluginEnv.invalidKey', { key }));
        return;
      }
      if (env[key] !== undefined) {
        setError(t('pluginEnv.duplicateKey', { key }));
        return;
      }
      env[key] = row.value;
    }

    setSaving(true);
    setError('');
    try {
      await api.SetPluginEnv(name, env);
      await onSaved?.();
      store.toast('success', t('pluginEnv.saved', { name }));
      onClose();
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="overlay" onMouseDown={(event) => event.target === event.currentTarget && onClose()}>
      <div className="modal provider-modal" role="dialog" aria-modal="true" aria-label={t('pluginEnv.title')}>
        <div className="modal-head">
          <span className="modal-title">{t('pluginEnv.title')}</span>
          <span className="badge">{name}</span>
          <div className="spacer" />
          <button className="icon-btn" aria-label={t('common.close')} onClick={onClose}>
            <X size={15} />
          </button>
        </div>
        <div className="settings-pane" style={{ padding: 22 }}>
          <p className="desc">{t('pluginEnv.hint')}</p>
          {loading ? (
            <div className="settings-footnote">{t('common.loading')}</div>
          ) : (
            <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
              {rows.map((row) => (
                <div key={row.id} style={{ display: 'flex', gap: 8, alignItems: 'flex-end' }}>
                  <div style={{ flex: 1 }}>
                    <Field label={t('pluginEnv.key')}>
                      <input
                        className="input"
                        aria-label={t('pluginEnv.key')}
                        autoComplete="off"
                        spellCheck={false}
                        value={row.key}
                        placeholder="KEY"
                        onChange={(event) => updateRow(row.id, { key: event.target.value })}
                      />
                    </Field>
                  </div>
                  <div style={{ flex: 2 }}>
                    <Field label={t('pluginEnv.value')}>
                      <input
                        className="input"
                        aria-label={t('pluginEnv.value')}
                        type={row.reveal ? 'text' : 'password'}
                        autoComplete="off"
                        spellCheck={false}
                        value={row.value}
                        placeholder={t('pluginEnv.valuePlaceholder')}
                        onChange={(event) => updateRow(row.id, { value: event.target.value })}
                      />
                    </Field>
                  </div>
                  <button
                    className="icon-btn"
                    type="button"
                    aria-label={row.reveal ? t('pluginEnv.hide') : t('pluginEnv.reveal')}
                    onClick={() => updateRow(row.id, { reveal: !row.reveal })}
                  >
                    {row.reveal ? <EyeOff size={14} /> : <Eye size={14} />}
                  </button>
                  <button
                    className="icon-btn"
                    type="button"
                    aria-label={t('pluginEnv.remove')}
                    onClick={() => removeRow(row.id)}
                  >
                    <Trash2 size={14} />
                  </button>
                </div>
              ))}
              <div>
                <button className="btn small" type="button" onClick={addRow}>
                  <Plus size={12} />
                  {t('pluginEnv.add')}
                </button>
              </div>
            </div>
          )}
          {error ? <div className="notice error">{error}</div> : null}
          <p className="desc">{t('pluginEnv.restartHint')}</p>
          <div className="settings-save-bar">
            <button className="btn ghost" onClick={onClose}>
              {t('common.cancel')}
            </button>
            <span className="spacer" />
            <button className="btn primary" disabled={saving || loading} onClick={() => void save()}>
              {saving ? t('common.saving') : t('common.save')}
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}
