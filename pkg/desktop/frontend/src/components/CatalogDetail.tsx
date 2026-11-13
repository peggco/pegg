import { useEffect, useMemo, useState } from 'react';
import { ArrowLeft, Check, Copy, Download, ExternalLink } from 'lucide-react';
import { api } from '../lib/bridge';
import { renderMarkdown } from '../lib/markdown';
import { useT } from '../lib/i18n';
import {
  catalogInstallCommand,
  catalogTransport,
  initials,
  skillID,
} from '../lib/catalog';
import { errorText, installCatalogMCP, installCatalogPlugin, installHubSkill } from '../state/actions';
import { store } from '../state/store';
import type { CatalogEntry, CatalogItem } from '../types';
import { MCPEnvDialog } from './MCPEnvDialog';

type Scope = 'global' | 'project';

export function CatalogDetail({
  item,
  entry,
  readme,
  readmeLoading,
  installed,
  onBack,
  onInstalled,
}: {
  item: CatalogItem;
  entry?: CatalogEntry | null;
  readme: string;
  readmeLoading: boolean;
  installed: boolean;
  onBack: () => void;
  onInstalled: (item: CatalogItem) => void;
}) {
  const t = useT();
  const [scope, setScope] = useState<Scope>('global');
  const [busy, setBusy] = useState(false);
  const [copied, setCopied] = useState(false);
  const [envOpen, setEnvOpen] = useState(false);
  const [confirming, setConfirming] = useState(false);
  const [restart, setRestart] = useState(false);
  const [warnings, setWarnings] = useState<string[]>([]);
  const [error, setError] = useState('');

  useEffect(() => {
    setRestart(false);
    setError('');
    setConfirming(false);
    setWarnings([]);
  }, [item.kind, item.id]);

  useEffect(() => {
    if (!copied) return;
    const timer = window.setTimeout(() => setCopied(false), 1400);
    return () => window.clearTimeout(timer);
  }, [copied]);

  const command = catalogInstallCommand(item);
  const transport = catalogTransport(item);
  const requiredEnv = entry?.requiredEnv ?? item.requiredEnv ?? [];
  const compatibilityWarnings = entry?.compatibilityWarnings ?? [];
  const repoURL = item.repository
    ? `https://github.com/${item.repository}`
    : item.homepage || item.url || (/^[A-Za-z0-9._-]+\/[A-Za-z0-9._-]+$/.test(item.source) ? `https://github.com/${item.source}` : '');

  const readmeHtml = useMemo(() => {
    if (!readme) return '';
    try {
      return renderMarkdown(readme);
    } catch {
      return '';
    }
  }, [readme]);

  const copy = async () => {
    try {
      await navigator.clipboard.writeText(command);
      setCopied(true);
    } catch {
      store.toast('warn', 'Clipboard is unavailable');
    }
  };

  const installSkill = async () => {
    setBusy(true);
    setError('');
    try {
      await installHubSkill(item.source, skillID(item), scope);
      onInstalled(item);
    } catch (err) {
      setError(errorText(err));
    } finally {
      setBusy(false);
    }
  };

  const installMCP = async (env: Record<string, string>) => {
    setBusy(true);
    setError('');
    try {
      const res = await installCatalogMCP(item.id, env, {}, scope);
      onInstalled(item);
      setEnvOpen(false);
      if (res.restartRequired) setRestart(true);
    } catch (err) {
      setError(errorText(err));
      throw err;
    } finally {
      setBusy(false);
    }
  };

  const installPlugin = async () => {
    setConfirming(false);
    setBusy(true);
    setError('');
    try {
      const res = await installCatalogPlugin(item.source || item.id, item.name);
      onInstalled(item);
      setWarnings(res.warnings ?? []);
      if (res.restartRequired) setRestart(true);
    } catch (err) {
      setError(errorText(err));
    } finally {
      setBusy(false);
    }
  };

  const onInstallClick = () => {
    if (busy || installed) return;
    if (item.kind === 'skills') {
      void installSkill();
      return;
    }
    if (item.kind === 'mcp') {
      if (requiredEnv.length) setEnvOpen(true);
      else void installMCP({});
      return;
    }
    setConfirming(true);
  };

  return (
    <div className="hub-pane">
      <button className="hub-back" onClick={onBack}>
        <ArrowLeft size={14} />
        {t('catalog.back')}
      </button>

      <div className="hub-detail-head">
        <span className="hub-avatar large">{initials(item.name)}</span>
        <div>
          <span className={`hub-status${item.official ? ' official' : ''}`}>
            <span className="hub-dot" />
            {item.official ? t('catalog.official') : t('catalog.community')}
          </span>
          <h1 className="hub-detail-title">{item.name}</h1>
          <div className="hub-card-source">{item.source}</div>
        </div>
      </div>

      <div className="hub-install-bar">
        <span className="hub-install-label">{t('catalog.installCommand')}</span>
        <code>{command}</code>
        <button className="btn small" onClick={() => void copy()}>
          {copied ? <Check size={12} /> : <Copy size={12} />}
          {copied ? t('catalog.copied') : t('catalog.copy')}
        </button>
        {item.kind !== 'plugins' ? (
          <div className="hub-scope">
            {(['global', 'project'] as Scope[]).map((value) => (
              <button key={value} className={scope === value ? 'active' : ''} onClick={() => setScope(value)}>
                {t(`catalog.${value}`)}
              </button>
            ))}
          </div>
        ) : null}
        <button className="btn primary small" disabled={busy || installed} onClick={onInstallClick}>
          {installed ? <Check size={12} /> : <Download size={12} />}
          {installed ? t('catalog.installed') : busy ? t('catalog.installing') : t('catalog.install')}
        </button>
      </div>

      {compatibilityWarnings.length > 0 ? (
        <div className="notice warn">
          <div style={{ flex: 1 }}>
            <strong>{t('catalog.compatibilityWarning')}</strong>
            <ul className="catalog-warnings">
              {compatibilityWarnings.map((warning) => (
                <li key={warning}>{warning}</li>
              ))}
            </ul>
            <span>{t('catalog.compatibilityContinue')}</span>
          </div>
        </div>
      ) : null}

      {warnings.length > 0 ? (
        <div className="notice warn">
          <div style={{ flex: 1 }}>
            <strong>{t('catalog.installWarnings')}</strong>
            <ul className="catalog-warnings">
              {warnings.map((warning) => (
                <li key={warning}>{warning}</li>
              ))}
            </ul>
          </div>
        </div>
      ) : null}

      {restart ? <div className="notice warn">{t('catalog.restartRequired')}</div> : null}
      {error ? <div className="notice error">{error}</div> : null}
      {confirming ? (
        <div className="notice warn">
          <span style={{ flex: 1 }}>{t('catalog.pluginConfirm', { name: item.name })}</span>
          <button className="btn small primary" onClick={() => void installPlugin()}>
            {t('catalog.confirm')}
          </button>
          <button className="btn small ghost" onClick={() => setConfirming(false)}>
            {t('common.cancel')}
          </button>
        </div>
      ) : null}

      <div className="hub-detail-grid">
        <div className="hub-detail-card">
          <div className="hub-detail-heading">
            <span>{t('catalog.readme')}</span>
            {repoURL ? (
              <button className="hub-link" onClick={() => void api.OpenExternal(repoURL)}>
                GitHub <ExternalLink size={11} />
              </button>
            ) : null}
          </div>
          {readmeLoading ? (
            <div className="thread-loading">
              <span className="spinner" /> {t('common.loading')}
            </div>
          ) : readmeHtml ? (
            <div className="assistant-text md hub-readme" dangerouslySetInnerHTML={{ __html: readmeHtml }} />
          ) : (
            <div className="hub-empty">{t('catalog.noReadme')}</div>
          )}
        </div>

        <div className="hub-detail-side">
          <div className="hub-detail-card">
            <div className="hub-detail-heading">
              <span>{t('catalog.details')}</span>
            </div>
            <dl className="hub-detail-list">
              <div>
                <dt>{t('catalog.source')}</dt>
                <dd>{item.source}</dd>
              </div>
              <div>
                <dt>{t('catalog.status')}</dt>
                <dd>{item.official ? t('catalog.official') : t('catalog.community')}</dd>
              </div>
              <div>
                <dt>{t('catalog.license')}</dt>
                <dd>{item.license || '—'}</dd>
              </div>
              {item.kind === 'skills' ? (
                <div>
                  <dt>{t('catalog.skillId')}</dt>
                  <dd>{skillID(item)}</dd>
                </div>
              ) : null}
              {item.kind === 'mcp' ? (
                <>
                  <div>
                    <dt>{t('catalog.transport')}</dt>
                    <dd>{transport || '—'}</dd>
                  </div>
                  {item.version ? (
                    <div>
                      <dt>{t('catalog.version')}</dt>
                      <dd>{item.version}</dd>
                    </div>
                  ) : null}
                  {requiredEnv.length ? (
                    <div>
                      <dt>{t('catalog.requiredEnv')}</dt>
                      <dd className="hub-required-env">{requiredEnv.join(', ')}</dd>
                    </div>
                  ) : null}
                </>
              ) : null}
              {item.kind === 'plugins' ? (
                <>
                  {item.version ? (
                    <div>
                      <dt>{t('catalog.version')}</dt>
                      <dd>{item.version}</dd>
                    </div>
                  ) : null}
                  {item.platforms?.length ? (
                    <div>
                      <dt>{t('catalog.platforms')}</dt>
                      <dd>{item.platforms.join(', ')}</dd>
                    </div>
                  ) : null}
                  {item.capabilities?.length ? (
                    <div>
                      <dt>{t('catalog.capabilities')}</dt>
                      <dd>{item.capabilities.join(', ')}</dd>
                    </div>
                  ) : null}
                  {entry?.peggRequirement ? (
                    <div>
                      <dt>{t('catalog.peggRequirement')}</dt>
                      <dd>{entry.peggRequirement}</dd>
                    </div>
                  ) : null}
                  {entry?.protocolRequirement ? (
                    <div>
                      <dt>{t('catalog.protocolRequirement')}</dt>
                      <dd>{entry.protocolRequirement}</dd>
                    </div>
                  ) : null}
                </>
              ) : null}
            </dl>
          </div>
          <button className="btn primary hub-side-install" disabled={busy || installed} onClick={onInstallClick}>
            {installed ? <Check size={13} /> : <Download size={13} />}
            {installed ? t('catalog.installed') : busy ? t('catalog.installing') : t('catalog.install')}
          </button>
        </div>
      </div>

      {envOpen ? (
        <MCPEnvDialog
          name={item.name}
          requiredEnv={requiredEnv}
          docsUrl={repoURL}
          onClose={() => setEnvOpen(false)}
          onSubmit={(env) => installMCP(env)}
        />
      ) : null}
    </div>
  );
}
