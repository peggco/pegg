import { useEffect, useState } from 'react';
import { api, onConnectStatus, onTeamStatus } from '../lib/bridge';
import { errorText, loadModels } from '../state/actions';
import { store } from '../state/store';
import { useT } from '../lib/i18n';
import type { ConnectSettings, TeamStatus } from '../types';
import { Card, Empty, Field, PageHead, Switch } from './SettingsUI';

function stateLabel(state: string): string {
  switch (state) {
    case 'connected':
      return 'Connected';
    case 'reconnecting':
      return 'Reconnecting';
    case 'starting':
      return 'Starting';
    case 'stopping':
      return 'Stopping';
    case 'waiting':
      return 'Waiting for browser';
    case 'approving':
      return 'Approving';
    case 'validated':
      return 'Validated';
    case 'running':
      return 'Running';
    case 'done':
      return 'Done';
    case 'error':
      return 'Error';
    default:
      return 'Stopped';
  }
}

function stateTone(state: string): string {
  switch (state) {
    case 'connected':
    case 'validated':
    case 'done':
      return 'ok';
    case 'error':
      return 'error';
    case 'starting':
    case 'reconnecting':
    case 'waiting':
    case 'approving':
    case 'running':
    case 'stopping':
      return 'warn';
    default:
      return 'muted';
  }
}

export function TeamPanel() {
  const t = useT();
  const [connect, setConnect] = useState<ConnectSettings | null>(null);
  const [team, setTeam] = useState<TeamStatus | null>(null);
  const [token, setToken] = useState('');
  const [busy, setBusy] = useState(false);
  const [teamBusy, setTeamBusy] = useState(false);

  const refresh = async () => {
    try {
      const [nextConnect, nextTeam] = await Promise.all([api.ConnectSettings(), api.TeamStatus()]);
      setConnect(nextConnect);
      setTeam(nextTeam);
    } catch (err) {
      store.toast('error', errorText(err));
    }
  };

  useEffect(() => {
    void refresh();
  }, []);

  useEffect(() => onConnectStatus((status) => setConnect((current) => (current ? { ...current, status } : current))), []);
  useEffect(
    () =>
      onTeamStatus((status) => {
        setTeam(status);
        if (status.state === 'done') void loadModels();
      }),
    [],
  );

  const save = async (patch: Partial<ConnectSettings> = {}) => {
    if (!connect) return;
    setBusy(true);
    try {
      const next = await api.SaveConnectSettings({ ...connect, token, ...patch });
      setConnect(next);
      setToken('');
      store.toast('success', t('team.saved'));
    } catch (err) {
      store.toast('error', errorText(err));
    } finally {
      setBusy(false);
    }
  };

  const toggle = async (key: 'enabled' | 'autoApprove', value: boolean) => {
    if (!connect) return;
    setConnect({ ...connect, [key]: value });
    await save({ [key]: value });
  };

  const start = async () => {
    setBusy(true);
    try {
      const status = await api.ConnectStart();
      setConnect((current) => (current ? { ...current, status } : current));
      if (status.state === 'error') store.toast('error', status.error ?? 'connect failed');
    } catch (err) {
      store.toast('error', errorText(err));
    } finally {
      setBusy(false);
    }
  };

  const stop = async () => {
    setBusy(true);
    try {
      const status = await api.ConnectStop();
      setConnect((current) => (current ? { ...current, status } : current));
    } catch (err) {
      store.toast('error', errorText(err));
    } finally {
      setBusy(false);
    }
  };

  const logout = async () => {
    setBusy(true);
    try {
      const next = await api.ConnectLogout();
      setConnect(next);
      setToken('');
      store.toast('success', t('team.loggedOut'));
    } catch (err) {
      store.toast('error', errorText(err));
    } finally {
      setBusy(false);
    }
  };

  const resetDevice = async () => {
    setBusy(true);
    try {
      const next = await api.ConnectResetDevice();
      setConnect(next);
      store.toast('success', t('team.deviceReset'));
    } catch (err) {
      store.toast('error', errorText(err));
    } finally {
      setBusy(false);
    }
  };

  const connectTeam = async () => {
    setTeamBusy(true);
    try {
      setTeam(await api.TeamConnect());
    } catch (err) {
      store.toast('error', errorText(err));
    } finally {
      setTeamBusy(false);
    }
  };

  const cancelTeam = async () => {
    setTeamBusy(true);
    try {
      setTeam(await api.TeamCancel());
    } catch (err) {
      store.toast('error', errorText(err));
    } finally {
      setTeamBusy(false);
    }
  };

  const refreshTeam = async (provider: string) => {
    setTeamBusy(true);
    try {
      setTeam(await api.TeamRefresh(provider));
      store.toast('success', t('team.providerRefreshed'));
    } catch (err) {
      store.toast('error', errorText(err));
    } finally {
      setTeamBusy(false);
    }
  };

  const removeTeam = async (provider: string) => {
    setTeamBusy(true);
    try {
      setTeam(await api.TeamRemove(provider));
      store.toast('success', t('team.providerRemoved'));
    } catch (err) {
      store.toast('error', errorText(err));
    } finally {
      setTeamBusy(false);
    }
  };

  const status = connect?.status;
  const running = status ? !['stopped', 'error'].includes(status.state) : false;
  const teamState = team?.state ?? 'idle';
  const teamRunning = ['starting', 'waiting', 'approving', 'running'].includes(teamState);

  return (
    <>
      <PageHead title={t('settings.team')} description={t('team.description')} />

      <Card
        title={t('team.relayTitle')}
        actions={
          <span className={`badge ${stateTone(status?.state ?? 'stopped')}`}>{stateLabel(status?.state ?? 'stopped')}</span>
        }
      >
        {!connect ? (
          <Empty>{t('common.loading')}</Empty>
        ) : (
          <>
            <div className="settings-grid">
              <Field label={t('team.token')} hint={connect.hasToken ? t('team.tokenSaved', { token: connect.tokenMasked }) : t('team.tokenHint')}>
                <input
                  className="input"
                  type="password"
                  value={token}
                  placeholder={connect.hasToken ? connect.tokenMasked : t('team.tokenPlaceholder')}
                  onChange={(event) => setToken(event.target.value)}
                />
              </Field>
              <Field label={t('team.deviceId')}>
                <div className="settings-mono">{connect.deviceId}</div>
              </Field>
            </div>

            <div className="settings-row">
              <div className="info">
                <div className="name">{t('team.autoStart')}</div>
                <div className="desc">{t('team.autoStartHint')}</div>
              </div>
              <div className="control">
                <Switch checked={connect.enabled} onChange={(value) => void toggle('enabled', value)} />
              </div>
            </div>
            <div className="settings-row">
              <div className="info">
                <div className="name">{t('team.autoApprove')}</div>
                <div className="desc">{t('team.autoApproveHint')}</div>
              </div>
              <div className="control">
                <Switch checked={connect.autoApprove} onChange={(value) => void toggle('autoApprove', value)} />
              </div>
            </div>

            <div className="preset-row">
              <button className="btn primary small" disabled={busy} onClick={() => void save()}>
                {t('common.save')}
              </button>
              {running ? (
                <button className="btn small" disabled={busy} onClick={() => void stop()}>
                  {t('team.disconnect')}
                </button>
              ) : (
                <button className="btn small" disabled={busy || !connect.hasToken} onClick={() => void start()}>
                  {t('team.connect')}
                </button>
              )}
              <button className="btn small" disabled={busy || !connect.hasToken} onClick={() => void logout()}>
                {t('team.logout')}
              </button>
              <button className="btn small" disabled={busy} onClick={() => void resetDevice()}>
                {t('team.resetDevice')}
              </button>
            </div>

            {status?.error ? <div className="notice error">{status.error}</div> : null}
            {status?.logs?.length ? (
              <pre className="settings-log">{status.logs.slice(-40).join('\n')}</pre>
            ) : null}
          </>
        )}
      </Card>

      <Card
        title={t('team.providersTitle')}
        actions={
          <button className="btn small" disabled={teamBusy} onClick={() => void connectTeam()}>
            {t('team.connectTeam')}
          </button>
        }
      >
        {team?.url ? (
          <div className="settings-row">
            <div className="info">
              <div className="desc">{t('team.browserOpen')}</div>
            </div>
            <div className="control">
              <button className="btn small" onClick={() => void api.OpenExternal(team.url ?? '')}>
                {t('team.openBrowser')}
              </button>
            </div>
          </div>
        ) : null}

        {teamRunning ? (
          <div className="preset-row">
            <span className={`badge ${stateTone(teamState)}`}>{stateLabel(teamState)}</span>
            <button className="btn small" disabled={teamBusy} onClick={() => void cancelTeam()}>
              {t('common.cancel')}
            </button>
          </div>
        ) : null}

        {team?.error ? <div className="notice error">{team.error}</div> : null}

        {team?.providers?.length ? (
          <div className="settings-list">
            {team.providers.map((provider) => (
              <div className="settings-list-row" key={provider.name}>
                <div className="row-main">
                  <div className="row-name">{provider.name}</div>
                </div>
                <div className="row-side">
                  <button className="btn small" disabled={teamBusy} onClick={() => void refreshTeam(provider.name)}>
                    {t('team.refresh')}
                  </button>
                  <button className="btn small" disabled={teamBusy} onClick={() => void removeTeam(provider.name)}>
                    {t('common.remove')}
                  </button>
                </div>
              </div>
            ))}
          </div>
        ) : (
          <Empty>{t('team.noProviders')}</Empty>
        )}

        {team?.logs?.length ? <pre className="settings-log">{team.logs.slice(-40).join('\n')}</pre> : null}
      </Card>
    </>
  );
}
