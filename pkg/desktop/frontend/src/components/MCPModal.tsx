import { useState } from 'react';
import { X } from 'lucide-react';
import { api } from '../lib/bridge';
import { errorText } from '../state/actions';
import { store } from '../state/store';
import { useT } from '../lib/i18n';
import { Field } from './SettingsUI';

type Transport = 'stdio' | 'http';

function parseKeyValues(text: string): Record<string, string> {
  const out: Record<string, string> = {};
  for (const raw of text.split('\n')) {
    const line = raw.trim();
    if (!line || line.startsWith('#')) continue;
    const idx = line.indexOf('=');
    if (idx <= 0) continue;
    const key = line.slice(0, idx).trim();
    if (key) out[key] = line.slice(idx + 1).trim();
  }
  return out;
}

function parseArgs(text: string): string[] {
  return text
    .split('\n')
    .map((line) => line.trim())
    .filter(Boolean);
}

export function MCPModal({ onClose, onAdded }: { onClose: () => void; onAdded: () => Promise<void> }) {
  const t = useT();
  const [name, setName] = useState('');
  const [transport, setTransport] = useState<Transport>('stdio');
  const [command, setCommand] = useState('');
  const [args, setArgs] = useState('');
  const [url, setUrl] = useState('');
  const [env, setEnv] = useState('');
  const [headers, setHeaders] = useState('');
  const [saving, setSaving] = useState(false);

  const add = async () => {
    const trimmed = name.trim();
    if (!trimmed) {
      store.toast('warn', t('mcp.nameRequired'));
      return;
    }
    if (transport === 'stdio' && !command.trim()) {
      store.toast('warn', t('mcp.commandRequired'));
      return;
    }
    if (transport === 'http' && !url.trim()) {
      store.toast('warn', t('mcp.urlRequired'));
      return;
    }
    setSaving(true);
    try {
      await api.AddMCPServer({
        name: trimmed,
        command: transport === 'stdio' ? command.trim() : '',
        args: transport === 'stdio' ? parseArgs(args) : [],
        env: transport === 'stdio' ? parseKeyValues(env) : {},
        url: transport === 'http' ? url.trim() : '',
        headers: transport === 'http' ? parseKeyValues(headers) : {},
      });
      await onAdded();
      store.toast('success', t('mcp.added', { name: trimmed }));
      onClose();
    } catch (err) {
      store.toast('error', errorText(err));
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="overlay" onMouseDown={(event) => event.target === event.currentTarget && onClose()}>
      <div className="modal provider-modal" role="dialog" aria-modal="true" aria-label={t('mcp.addTitle')}>
        <div className="modal-head">
          <span className="modal-title">{t('mcp.addTitle')}</span>
          <div className="spacer" />
          <button className="icon-btn" aria-label={t('common.close')} onClick={onClose}>
            <X size={15} />
          </button>
        </div>
        <div className="settings-pane" style={{ padding: 22 }}>
          <div className="settings-grid">
            <Field label={t('mcp.name')} hint={t('mcp.nameHint')}>
              <input
                className="input"
                value={name}
                placeholder="github"
                autoFocus
                onChange={(event) => setName(event.target.value)}
              />
            </Field>
            <Field label={t('mcp.transport')}>
              <select
                className="select"
                value={transport}
                onChange={(event) => setTransport(event.target.value as Transport)}
              >
                <option value="stdio">stdio</option>
                <option value="http">HTTP</option>
              </select>
            </Field>

            {transport === 'stdio' ? (
              <>
                <Field label={t('mcp.command')} hint={t('mcp.commandHint')}>
                  <input
                    className="input"
                    value={command}
                    placeholder="npx"
                    onChange={(event) => setCommand(event.target.value)}
                  />
                </Field>
                <Field label={t('mcp.args')} hint={t('mcp.argsHint')}>
                  <textarea
                    className="input"
                    rows={4}
                    value={args}
                    placeholder={'-y\n@modelcontextprotocol/server-github'}
                    onChange={(event) => setArgs(event.target.value)}
                  />
                </Field>
                <Field label={t('mcp.env')} hint={t('mcp.envHint')}>
                  <textarea
                    className="input"
                    rows={3}
                    value={env}
                    placeholder="GITHUB_TOKEN=ghp_…"
                    onChange={(event) => setEnv(event.target.value)}
                  />
                </Field>
              </>
            ) : (
              <>
                <Field label={t('mcp.url')} hint={t('mcp.urlHint')}>
                  <input
                    className="input"
                    value={url}
                    placeholder="https://example.com/mcp"
                    onChange={(event) => setUrl(event.target.value)}
                  />
                </Field>
                <Field label={t('mcp.headers')} hint={t('mcp.headersHint')}>
                  <textarea
                    className="input"
                    rows={3}
                    value={headers}
                    placeholder="Authorization=Bearer …"
                    onChange={(event) => setHeaders(event.target.value)}
                  />
                </Field>
              </>
            )}
          </div>
          <p className="desc">{t('mcp.restartHint')}</p>
          <div className="settings-save-bar">
            <button className="btn ghost" onClick={onClose}>
              {t('common.cancel')}
            </button>
            <span className="spacer" />
            <button className="btn primary" disabled={saving} onClick={() => void add()}>
              {saving ? t('common.saving') : t('common.add')}
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}
