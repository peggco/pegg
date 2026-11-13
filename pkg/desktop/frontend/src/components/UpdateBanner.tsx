import { Download, RefreshCw, RotateCw } from 'lucide-react';
import { api } from '../lib/bridge';
import { useT } from '../lib/i18n';
import { updateDownloads, updateSummary, updateUnsupportedAgent, updateUnsupportedDesktop } from '../lib/update';
import { downloadDesktopUpdate, restartToUpdate } from '../state/actions';
import { useStore } from '../state/store';

export function UpdateBanner() {
  const status = useStore((s) => s.selfUpdate);
  const t = useT();
  if (!status.available) return null;

  if (status.ready) {
    return (
      <div className="update-banner ready">
        <RotateCw size={14} />
        <span>{updateSummary(status, t)}</span>
        <span className="spacer" />
        <button className="update-action primary" onClick={() => void restartToUpdate()}>
          {t('update.restart')}
        </button>
      </div>
    );
  }

  if (status.state === 'downloading') {
    return (
      <div className="update-banner">
        <Download size={14} />
        <span>{t('update.downloadingAll', { progress: status.progress })}</span>
        <span className="update-progress">
          <span style={{ width: `${status.progress}%` }} />
        </span>
      </div>
    );
  }

  const canDownload = updateDownloads(status);
  const unsupportedDesktop = updateUnsupportedDesktop(status);
  const unsupportedAgent = updateUnsupportedAgent(status);

  return (
    <div className="update-banner">
      <RefreshCw size={14} />
      <span>
        {updateSummary(status, t)}
        {unsupportedAgent
          ? ` · ${t('update.agentManaged', { version: status.agentLatestVersion ?? '' })}`
          : ''}
      </span>
      <span className="spacer" />
      {canDownload ? (
        <button className="update-action primary" onClick={() => void downloadDesktopUpdate()}>
          {t('update.download')}
        </button>
      ) : null}
      {unsupportedDesktop && status.releaseUrl ? (
        <button className="update-action" onClick={() => void api.OpenExternal(status.releaseUrl as string)}>
          {t('update.open')}
        </button>
      ) : null}
    </div>
  );
}
