import type { SelfUpdateStatus } from '../types';

type Translate = (key: string, vars?: Record<string, string | number>) => string;

export function updateSummary(status: SelfUpdateStatus, t: Translate): string {
  const desktop = status.latestVersion ?? '';
  const agent = status.agentLatestVersion ?? status.latestVersion ?? '';
  if (status.ready) {
    if (status.desktopReady && status.agentReady) {
      return t('update.readyBoth', { desktop, agent });
    }
    if (status.agentReady) {
      return t('update.readyAgent', { version: agent });
    }
    return t('update.readyDesktop', { version: desktop });
  }
  if (status.available) {
    if (status.desktopAvailable && status.agentAvailable) {
      return t('update.availableBoth', { desktop, agent });
    }
    if (status.agentAvailable) {
      return t('update.availableAgent', { version: agent });
    }
    return t('update.availableDesktop', { version: desktop });
  }
  return t('update.upToDate');
}

export function updateDownloads(status: SelfUpdateStatus): boolean {
  return (
    (status.desktopAvailable && status.supported && !status.desktopReady) ||
    (status.agentAvailable && status.agentSupported && !status.agentReady)
  );
}

export function updateUnsupportedDesktop(status: SelfUpdateStatus): boolean {
  return status.desktopAvailable && !status.supported;
}

export function updateUnsupportedAgent(status: SelfUpdateStatus): boolean {
  return status.agentAvailable && !status.agentSupported;
}
