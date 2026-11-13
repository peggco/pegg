import { useState } from 'react';
import { ExternalLink, X } from 'lucide-react';
import { api } from '../lib/bridge';
import { useT } from '../lib/i18n';
import { Field } from './SettingsUI';

export function MCPEnvDialog({
  name,
  requiredEnv,
  docsUrl,
  onClose,
  onSubmit,
}: {
  name: string;
  requiredEnv: string[];
  docsUrl?: string;
  onClose: () => void;
  onSubmit: (env: Record<string, string>) => Promise<void> | void;
}) {
  const t = useT();
  const [values, setValues] = useState<Record<string, string>>(() =>
    Object.fromEntries(requiredEnv.map((key) => [key, ''])),
  );
  const [error, setError] = useState('');
  const [saving, setSaving] = useState(false);

  const submit = async () => {
    const missing = requiredEnv.filter((key) => !values[key]?.trim());
    if (missing.length) {
      setError(t('catalog.envRequired', { keys: missing.join(', ') }));
      return;
    }
    setSaving(true);
    setError('');
    try {
      await onSubmit(Object.fromEntries(requiredEnv.map((key) => [key, values[key].trim()])));
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="overlay" onMouseDown={(event) => event.target === event.currentTarget && onClose()}>
      <div className="modal provider-modal" role="dialog" aria-modal="true" aria-label={t('catalog.envTitle')}>
        <div className="modal-head">
          <span className="modal-title">{t('catalog.envTitle')}</span>
          <span className="badge">{name}</span>
          <div className="spacer" />
          <button className="icon-btn" aria-label={t('common.close')} onClick={onClose}>
            <X size={15} />
          </button>
        </div>
        <div className="settings-pane" style={{ padding: 22 }}>
          <p className="desc">{t('catalog.envHint')}</p>
          <div className="settings-grid">
            {requiredEnv.map((key) => (
              <Field label={key} key={key}>
                <input
                  className="input"
                  type="password"
                  name={key}
                  autoComplete="off"
                  spellCheck={false}
                  value={values[key] ?? ''}
                  placeholder={t('catalog.envPlaceholder')}
                  onChange={(event) => setValues((current) => ({ ...current, [key]: event.target.value }))}
                />
              </Field>
            ))}
          </div>
          {error ? <div className="notice error">{error}</div> : null}
          {docsUrl ? (
            <button className="hub-link" type="button" onClick={() => void api.OpenExternal(docsUrl)}>
              {t('catalog.docs')} <ExternalLink size={11} />
            </button>
          ) : null}
          <div className="settings-save-bar">
            <button className="btn ghost" onClick={onClose}>
              {t('common.cancel')}
            </button>
            <span className="spacer" />
            <button className="btn primary" disabled={saving} onClick={() => void submit()}>
              {saving ? t('catalog.installing') : t('catalog.install')}
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}
