import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import {
  AppWindow,
  Bell,
  BookOpen,
  Brain,
  Download,
  FolderOpen,
  Info,
  KeyRound,
  LogIn,
  MessageSquare,
  Palette,
  Plug,
  Puzzle,
  Settings2,
  ShieldCheck,
  Sparkles,
  SquareKanban,
  Trash2,
  Users,
  X,
} from 'lucide-react';
import { api, onPeggStatus } from '../lib/bridge';
import {
  changeModel,
  changeReasoning,
  checkDesktopUpdate,
  chooseBinary,
  downloadDesktopUpdate,
  errorText,
  findBinary,
  installBinary,
  loadModels,
  refreshProviderSetup,
  restartToUpdate,
  savePrefs,
  setTheme,
  toggleSettings,
} from '../state/actions';
import { store, useStore } from '../state/store';
import { setLanguage, useT } from '../lib/i18n';
import { updateDownloads, updateSummary, updateUnsupportedAgent, updateUnsupportedDesktop } from '../lib/update';
import type {
  ForeverStatus,
  ProviderCatalogEntry,
  STTProviderEntry,
  SettingsSnapshot,
  PluginSettingPage,
} from '../types';
import {
  MCPPanel,
  MemoryPanel,
  PermissionsPanel,
  PluginsPanel,
  RulesPanel,
  SessionPanel,
  SkillsPanel,
} from './SettingsPanels';
import { TeamPanel } from './TeamPanel';
import { PluginPagePanel } from './PluginPagePanel';
import { Card, Field, OptionCard, OptionGrid, PageHead, Range, Segmented } from './SettingsUI';
import { ChatHistoryPreview, DirectionPreview, LayoutPreview, SidebarModePreview } from './AppearancePreviews';

const CUSTOM_PROVIDER = '__custom__';

type BuiltinTab =
  | 'general'
  | 'appearance'
  | 'session'
  | 'permissions'
  | 'memory'
  | 'mcp'
  | 'skills'
  | 'rules'
  | 'plugins'
  | 'team'
  | 'system';

type Tab = BuiltinTab | `plugin:${string}`;

function pluginTabID(page: PluginSettingPage): Tab {
  return `plugin:${page.owner}:${page.id}`;
}

function pluginIcon(page: PluginSettingPage) {
  const key = `${page.owner} ${page.title}`.toLowerCase();
  if (key.includes('kanban')) return SquareKanban;
  if (key.includes('notify')) return Bell;
  return Puzzle;
}

const TAB_GROUPS: Array<{ label: string; tabs: Array<{ id: BuiltinTab; label: string; icon: typeof Settings2 }> }> = [
  {
    label: 'settings.workspaceGroup',
    tabs: [
      { id: 'general', label: 'settings.general', icon: Settings2 },
      { id: 'appearance', label: 'settings.appearance', icon: Palette },
    ],
  },
  {
    label: 'settings.agentGroup',
    tabs: [
      { id: 'session', label: 'settings.session', icon: MessageSquare },
      { id: 'permissions', label: 'settings.permissions', icon: ShieldCheck },
      { id: 'memory', label: 'settings.memory', icon: Brain },
    ],
  },
  {
    label: 'settings.integrationsGroup',
    tabs: [
      { id: 'mcp', label: 'settings.mcp', icon: Plug },
      { id: 'skills', label: 'settings.skills', icon: Sparkles },
      { id: 'rules', label: 'settings.rules', icon: BookOpen },
      { id: 'plugins', label: 'settings.plugins', icon: Puzzle },
    ],
  },
  {
    label: 'settings.teamGroup',
    tabs: [{ id: 'team', label: 'settings.team', icon: Users }],
  },
  {
    label: 'settings.systemGroup',
    tabs: [{ id: 'system', label: 'settings.system', icon: AppWindow }],
  },
];

export function Settings() {
  const open = useStore((s) => s.settingsOpen);
  const requestedTab = useStore((s) => s.settingsTab);
  const providerSetup = useStore((s) => s.providerSetup);
  const [tab, setTab] = useState<Tab>((requestedTab as Tab) || 'general');
  const [snap, setSnap] = useState<SettingsSnapshot | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [pluginPages, setPluginPages] = useState<PluginSettingPage[]>([]);
  const t = useT();
  const models = useStore((state) => state.models);
  const desktopView = useStore((state) => state.desktopView);
  const viewPages = desktopView?.settingsPages ?? [];
  const pages = useMemo(() => {
    const merged = [...pluginPages];
    for (const page of viewPages) {
      if (!merged.some((candidate) => candidate.owner === page.owner && candidate.id === page.id)) {
        merged.push(page);
      }
    }
    return merged;
  }, [pluginPages, viewPages]);

  const refresh = useCallback(async () => {
    setLoading(true);
    try {
      setSnap(await api.SettingsSnapshot());
      setError('');
    } catch (err) {
      setError(errorText(err));
    } finally {
      setLoading(false);
    }
  }, []);

  const refreshPlugins = useCallback(async () => {
    try {
      setPluginPages(await api.PluginSettingsPages());
    } catch {
      setPluginPages([]);
    }
  }, []);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  useEffect(() => {
    if (!open) return;
    if (!snap) void refresh();
    if (!models.length) void loadModels();
    void refreshPlugins();
  }, [open, refresh, refreshPlugins, snap, models.length]);

  useEffect(() => {
    setTab((requestedTab as Tab) || 'general');
  }, [requestedTab]);

  useEffect(() => {
    if (open) void refresh();
  }, [providerSetup, open]);

  useEffect(() => {
    if (typeof requestedTab === 'string' && requestedTab.startsWith('plugin:')) {
      if (!pages.some((page) => pluginTabID(page) === requestedTab)) {
        setTab('general');
      }
    }
  }, [requestedTab, pages]);

  const activePluginPage = pages.find((page) => pluginTabID(page) === tab) ?? null;

  return (
    <div
      className={`overlay${open ? '' : ' hidden'}`}
      aria-hidden={!open}
      onMouseDown={(event) => event.target === event.currentTarget && open && toggleSettings(false)}
    >
      <div className="modal settings-modal" role="dialog" aria-modal="true" aria-label="Settings">
        <div className="modal-head">
          <span className="modal-title">{t('nav.settings')}</span>
          <span className="badge">Pegg Desktop</span>
          <div className="spacer" />
          <button className="icon-btn" aria-label="Close settings" onClick={() => toggleSettings(false)}>
            <X size={16} />
          </button>
        </div>
        <div className="modal-body">
          <div className="settings-shell">
            <nav className="settings-nav" aria-label="Settings sections">
              {TAB_GROUPS.map((group) => (
                <div key={group.label}>
                  <div className="settings-nav-label">{t(group.label)}</div>
                  {group.tabs.map((item) => (
                    <button
                      key={item.id}
                      className={`settings-nav-item${tab === item.id ? ' active' : ''}`}
                      onClick={() => setTab(item.id)}
                    >
                      <item.icon size={14} />
                      {t(item.label)}
                    </button>
                  ))}
                </div>
              ))}
              {pages.length ? (
                <div>
                  <div className="settings-nav-label">{t('settings.pluginPagesGroup')}</div>
                  {pages.map((page) => {
                    const id = pluginTabID(page);
                    const Icon = pluginIcon(page);
                    return (
                      <button
                        key={id}
                        className={`settings-nav-item${tab === id ? ' active' : ''}`}
                        onClick={() => setTab(id)}
                      >
                        <Icon size={14} />
                        {page.title}
                      </button>
                    );
                  })}
                </div>
              ) : null}
            </nav>
            <div className="settings-pane">
              {loading && !snap ? (
                <div className="thread-loading">
                  <span className="spinner" /> Loading settings…
                </div>
              ) : null}
              {error ? <div className="notice error">{error}</div> : null}
              {snap ? (
                <>
                  {tab === 'general' ? <GeneralPanel snap={snap} refresh={refresh} /> : null}
                  {tab === 'appearance' ? <AppearancePanel /> : null}
                  {tab === 'session' ? <SessionPanel snap={snap} refresh={refresh} /> : null}
                  {tab === 'permissions' ? <PermissionsPanel snap={snap} refresh={refresh} /> : null}
                  {tab === 'memory' ? <MemoryPanel snap={snap} refresh={refresh} /> : null}
                  {tab === 'mcp' ? <MCPPanel snap={snap} refresh={refresh} /> : null}
                  {tab === 'skills' ? <SkillsPanel snap={snap} refresh={refresh} /> : null}
                  {tab === 'rules' ? <RulesPanel snap={snap} refresh={refresh} /> : null}
                  {tab === 'plugins' ? (
                    <PluginsPanel snap={snap} refresh={refresh} onPluginsChanged={refreshPlugins} />
                  ) : null}
                  {tab === 'team' ? <TeamPanel /> : null}
                  {tab === 'system' ? <SystemPanel snap={snap} /> : null}
                  {activePluginPage ? (
                    <PluginPagePanel
                      page={activePluginPage}
                      onRefresh={refreshPlugins}
                      onPage={(page) =>
                        setPluginPages((current) =>
                          current.map((item) => (pluginTabID(item) === pluginTabID(page) ? page : item)),
                        )
                      }
                    />
                  ) : null}
                </>
              ) : null}
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}

function AppearancePanel() {
  const t = useT();
  const prefs = useStore((s) => s.prefs);

  return (
    <>
      <PageHead
        title={t('settings.appearance')}
        description="Light or dark, or follow the operating system. Sidebar and layout options apply immediately."
      />
      <Card title="Color mode">
        <div className="settings-row">
          <div className="info">
            <div className="name">Mode</div>
            <div className="desc">Choose a fixed palette or follow the operating system.</div>
          </div>
          <div className="control">
            <Segmented
              value={prefs.themeMode}
              options={[
                { value: 'auto', label: 'Auto' },
                { value: 'light', label: 'Light' },
                { value: 'dark', label: 'Dark' },
              ]}
              onChange={(mode) => void setTheme(mode === 'auto' ? prefs.theme : mode, mode)}
            />
          </div>
        </div>
        <div className="settings-row">
          <div className="info">
            <div className="name">Interface scale</div>
            <div className="desc">Adjust text and spacing density.</div>
          </div>
          <div className="control" style={{ width: 240 }}>
            <Range
              min={0.9}
              max={1.2}
              step={0.05}
              value={prefs.fontScale}
              onChange={(fontScale) => {
                void savePrefs({ fontScale });
                document.documentElement.style.setProperty('--pg-font-scale', String(fontScale));
              }}
              format={(value) => `${Math.round(value * 100)}%`}
            />
          </div>
        </div>
      </Card>
      <Card title="Sidebar">
        <OptionGrid>
          <OptionCard
            selected={prefs.sidebarMode === 'inset'}
            label="Inset"
            onClick={() => void savePrefs({ sidebarMode: 'inset' })}
          >
            <SidebarModePreview mode="inset" />
          </OptionCard>
          <OptionCard
            selected={prefs.sidebarMode === 'floating'}
            label="Floating"
            onClick={() => void savePrefs({ sidebarMode: 'floating' })}
          >
            <SidebarModePreview mode="floating" />
          </OptionCard>
          <OptionCard
            selected={prefs.sidebarMode === 'sidebar'}
            label="Sidebar"
            onClick={() => void savePrefs({ sidebarMode: 'sidebar' })}
          >
            <SidebarModePreview mode="sidebar" />
          </OptionCard>
        </OptionGrid>
      </Card>
      <Card title="Layout">
        <OptionGrid>
          <OptionCard
            selected={prefs.layoutMode === 'default'}
            label="Default"
            onClick={() => void savePrefs({ layoutMode: 'default' })}
          >
            <LayoutPreview mode="default" />
          </OptionCard>
          <OptionCard
            selected={prefs.layoutMode === 'compact'}
            label="Compact"
            onClick={() => void savePrefs({ layoutMode: 'compact' })}
          >
            <LayoutPreview mode="compact" />
          </OptionCard>
          <OptionCard
            selected={prefs.layoutMode === 'full'}
            label="Full layout"
            onClick={() => void savePrefs({ layoutMode: 'full' })}
          >
            <LayoutPreview mode="full" />
          </OptionCard>
        </OptionGrid>
      </Card>
      <Card title="Direction">
        <OptionGrid columns={2}>
          <OptionCard
            selected={prefs.direction === 'ltr'}
            label="Left to Right"
            onClick={() => void savePrefs({ direction: 'ltr' })}
          >
            <DirectionPreview direction="ltr" />
          </OptionCard>
          <OptionCard
            selected={prefs.direction === 'rtl'}
            label="Right to Left"
            onClick={() => void savePrefs({ direction: 'rtl' })}
          >
            <DirectionPreview direction="rtl" />
          </OptionCard>
        </OptionGrid>
      </Card>
      <Card title="Chat history">
        <OptionGrid columns={2}>
          <OptionCard
            selected={prefs.chatHistoryMode === 'detailed'}
            label="Detailed"
            onClick={() => void savePrefs({ chatHistoryMode: 'detailed' })}
          >
            <ChatHistoryPreview mode="detailed" />
          </OptionCard>
          <OptionCard
            selected={prefs.chatHistoryMode === 'compact'}
            label="Compact"
            onClick={() => void savePrefs({ chatHistoryMode: 'compact' })}
          >
            <ChatHistoryPreview mode="compact" />
          </OptionCard>
        </OptionGrid>
      </Card>
    </>
  );
}

function GeneralPanel({ snap, refresh }: { snap: SettingsSnapshot; refresh: () => Promise<void> }) {
  const t = useT();
  const models = useStore((s) => s.models);
  const chatProvider = useStore((s) => s.chat.provider);
  const chatModel = useStore((s) => s.chat.model);
  const reasoning = useStore((s) => s.chat.reasoningEffort);
  const [draft, setDraft] = useState(snap.general);
  const [saving, setSaving] = useState(false);
  const [saveError, setSaveError] = useState('');
  const [addProviderOpen, setAddProviderOpen] = useState(false);
  const [addSttOpen, setAddSttOpen] = useState(false);
  const languageSave = useRef<Promise<void>>(Promise.resolve());

  useEffect(() => setDraft(snap.general), [snap.general]);

  const providerNames = useMemo(() => {
    const set = new Set<string>();
    for (const entry of models) set.add(entry.provider);
    for (const provider of snap.general.providers) set.add(provider.name);
    return Array.from(set).sort();
  }, [models, snap.general.providers]);

  const providerModels = useMemo(() => {
    const provider = chatProvider || draft.provider;
    const live = models.filter((entry) => entry.provider === provider);
    if (live.length) {
      return live.map((entry) => ({
        id: entry.model.id,
        name: entry.model.name,
        efforts: [] as string[],
      }));
    }
    const configured = snap.general.providers.find((entry) => entry.name === provider);
    return (configured?.models ?? []).map((entry) => ({
      id: entry.id,
      name: entry.name,
      efforts: entry.reasoningEfforts ?? [],
    }));
  }, [models, chatProvider, draft.provider, snap.general.providers]);

  const currentProvider = chatProvider || draft.provider;
  const currentModel = chatModel || draft.model;
  const currentModelInfo = providerModels.find((entry) => entry.id === currentModel);
  const effortOptions = currentModelInfo?.efforts ?? [];

  const save = async () => {
    setSaving(true);
    setSaveError('');
    try {
      await api.SaveGeneralSettings(draft);
      setLanguage(draft.language);
      await refresh();
      store.toast('success', t('settings.generalSaved'));
    } catch (err) {
      const message = errorText(err);
      setSaveError(message);
      store.toast('error', message);
    } finally {
      setSaving(false);
    }
  };

  const changeLanguage = (language: string) => {
    setDraft((current) => ({ ...current, language }));
    setLanguage(language);
    setSaveError('');
    languageSave.current = languageSave.current
      .catch(() => undefined)
      .then(() => api.SaveLanguage(language))
      .catch((err) => {
        const message = errorText(err);
        setSaveError(message);
        store.toast('error', t('settings.languageSaveFailed', { error: message }));
      });
  };

  return (
    <>
      <PageHead
        title={t('settings.general')}
        description={t('settings.generalDesc')}
      />
      {saveError ? <div className="notice error">{saveError}</div> : null}
      <Card title={t('settings.modelTitle')}>
        <div className="settings-grid">
          <Field label={t('settings.provider')}>
            <select
              className="select"
              value={currentProvider}
              onChange={(event) => {
                const provider = event.target.value;
                const nextModels = models.filter((entry) => entry.provider === provider);
                const configured = snap.general.providers.find((entry) => entry.name === provider);
                const model = nextModels[0]?.model.id ?? configured?.models[0]?.id ?? '';
                setDraft((current) => ({ ...current, provider, model }));
                if (model) void changeModel(provider, model);
              }}
            >
              {providerNames.length ? null : <option value="">{t('settings.noProviders')}</option>}
              {providerNames.map((provider) => (
                <option key={provider} value={provider}>{provider}</option>
              ))}
            </select>
          </Field>
          <Field label={t('settings.model')}>
            <select
              className="select"
              value={currentModel}
              onChange={(event) => {
                const model = event.target.value;
                setDraft((current) => ({ ...current, model }));
                void changeModel(currentProvider, model);
              }}
            >
              {providerModels.length ? null : <option value="">{t('settings.providerDefault')}</option>}
              {providerModels.map((model) => (
                <option key={model.id} value={model.id}>{model.name || model.id}</option>
              ))}
            </select>
          </Field>
          <Field
            label={t('settings.reasoningEffort')}
            hint={
              effortOptions.length
                ? t('settings.reasoningEffortHint')
                : t('settings.reasoningEffortUnsupported')
            }
          >
            <select
              className="select"
              value={reasoning ?? ''}
              onChange={(event) => void changeReasoning(event.target.value)}
            >
              <option value="">{t('settings.default')}</option>
              {effortOptions.map((effort) => (
                <option key={effort} value={effort}>{effort}</option>
              ))}
            </select>
          </Field>
          <Field label={t('settings.language')} hint={t('settings.languageDesc')}>
            <select
              className="select"
              aria-label={t('settings.language')}
              value={draft.language}
              onChange={(event) => changeLanguage(event.target.value)}
            >
              {draft.languages.map((language) => (
                <option key={language.code} value={language.code}>{language.label}</option>
              ))}
            </select>
          </Field>
        </div>
      </Card>

      <Card
        title={`${t('settings.providersTitle')} · ${snap.general.providers.length}`}
        actions={
          <button className="btn small" onClick={() => setAddProviderOpen(true)}>
            <KeyRound size={13} />
            {t('settings.addProvider')}
          </button>
        }
      >
        {snap.general.providers.length ? (
          <div className="settings-list">
            {snap.general.providers.map((provider) => (
              <div className="settings-list-row" key={provider.name}>
                <div className="row-main">
                  <div className="row-name">{provider.name}</div>
                  <div className="row-desc">
                    {provider.driver ? `${provider.driver} · ` : ''}
                    {(() => {
                      const live = models.filter((entry) => entry.provider === provider.name).length;
                      const count = live || provider.models.length;
                      return t(
                        count === 1 ? 'settings.modelCount' : 'settings.modelCountPlural',
                        { count },
                      );
                    })()}
                  </div>
                </div>
                <div className="row-side">
                  <button
                    className="btn small ghost"
                    title={t('settings.removeProvider', { name: provider.name })}
                    onClick={() => {
                      void api
                        .RemoveProvider(provider.name)
                        .then(async () => {
                          await refresh();
                          void loadModels();
                          store.toast(
                            'success',
                            t('settings.providerRemoved', { name: provider.name }),
                          );
                        })
                        .catch((err) => store.toast('error', errorText(err)));
                    }}
                  >
                    <Trash2 size={12} />
                  </button>
                </div>
              </div>
            ))}
          </div>
        ) : (
          <div className="empty-state">{t('settings.noProvidersHint')}</div>
        )}
      </Card>

      <Card
        title={`${t('settings.sttTitle')} · ${snap.general.stt.entries.length}`}
        actions={
          <button className="btn small" onClick={() => setAddSttOpen(true)}>
            <KeyRound size={13} />
            {t('settings.addSttProvider')}
          </button>
        }
      >
        {snap.general.stt.entries.length ? (
          <div className="settings-list">
            {snap.general.stt.entries.map((entry) => (
              <div className="settings-list-row" key={entry.name}>
                <div className="row-main">
                  <div className="row-name">{entry.name}</div>
                  <div className="row-desc">
                    {t(
                      entry.models.length === 1
                        ? 'settings.modelCount'
                        : 'settings.modelCountPlural',
                      { count: entry.models.length },
                    )}
                  </div>
                </div>
                <div className="row-side">
                  <span className={`badge${entry.configured ? ' accent' : ''}`}>
                    {entry.configured
                      ? t('settings.configured')
                      : t('settings.notConfigured')}
                  </span>
                  {entry.configured ? (
                    <button
                      className="btn small ghost"
                      title={t('settings.removeProvider', { name: entry.name })}
                      onClick={() => {
                        void api
                          .RemoveSTTProvider(entry.name)
                          .then(async () => {
                            await refresh();
                            store.toast(
                              'success',
                              t('settings.providerRemoved', { name: entry.name }),
                            );
                          })
                          .catch((err) => store.toast('error', errorText(err)));
                      }}
                    >
                      <Trash2 size={12} />
                    </button>
                  ) : null}
                </div>
              </div>
            ))}
          </div>
        ) : (
          <div className="empty-state">{t('settings.sttEmpty')}</div>
        )}
      </Card>

      <Card title={t('settings.voiceInput')}>
        <div className="settings-grid">
          <Field label={t('settings.sttProvider')}>
            <select
              className="select"
              value={draft.stt.provider}
              onChange={(event) => {
                const provider = event.target.value;
                const providerModels = draft.stt.modelsByProvider[provider] ?? [];
                setDraft((current) => ({
                  ...current,
                  stt: {
                    ...current.stt,
                    provider,
                    model: providerModels[0] ?? '',
                    models: providerModels,
                  },
                }));
              }}
            >
              <option value="">{t('settings.sttNotConfigured')}</option>
              {draft.stt.configured.map((provider) => (
                <option key={provider} value={provider}>{provider}</option>
              ))}
            </select>
          </Field>
          <Field label={t('settings.sttModel')}>
            <select
              className="select"
              value={draft.stt.model}
              onChange={(event) =>
                setDraft((current) => ({
                  ...current,
                  stt: { ...current.stt, model: event.target.value },
                }))
              }
            >
              <option value="">{t('settings.providerDefault')}</option>
              {draft.stt.models.map((model) => (
                <option key={model} value={model}>{model}</option>
              ))}
            </select>
          </Field>
          <Field
            label={t('settings.recordCommand')}
            hint={t('settings.recordCommandHint')}
          >
            <input
              className="input"
              value={draft.stt.recordCommand}
              placeholder={t('settings.recordCommandPlaceholder')}
              onChange={(event) =>
                setDraft((current) => ({
                  ...current,
                  stt: { ...current.stt, recordCommand: event.target.value },
                }))
              }
            />
          </Field>
          <Field
            label={t('settings.maxDuration')}
            hint={t('settings.maxDurationHint')}
          >
            <input
              className="input"
              type="number"
              min={5}
              value={draft.stt.maxDuration}
              onChange={(event) =>
                setDraft((current) => ({
                  ...current,
                  stt: { ...current.stt, maxDuration: Number(event.target.value) },
                }))
              }
            />
          </Field>
        </div>
        <div className="settings-save-bar">
          <span className="settings-footnote">
            {draft.stt.providers.length
              ? t('settings.sttCredentialsNote')
              : t('settings.sttNoProvider')}
          </span>
          <span className="spacer" />
          <button className="btn primary" disabled={saving} onClick={() => void save()}>
            {saving ? t('common.saving') : t('common.save')}
          </button>
        </div>
      </Card>
      {addProviderOpen ? (
        <AddProviderModal
          onClose={() => setAddProviderOpen(false)}
          onAdded={async () => {
            await refresh();
            await refreshProviderSetup();
            void loadModels();
          }}
        />
      ) : null}
      {addSttOpen ? (
        <AddSTTProviderModal
          entries={snap.general.stt.entries}
          onClose={() => setAddSttOpen(false)}
          onAdded={refresh}
        />
      ) : null}
    </>
  );
}

function AddProviderModal({
  onClose,
  onAdded,
}: {
  onClose: () => void;
  onAdded: () => Promise<void>;
}) {
  const t = useT();
  const [catalog, setCatalog] = useState<ProviderCatalogEntry[]>([]);
  const [name, setName] = useState('');
  const [apiKey, setApiKey] = useState('');
  const [customName, setCustomName] = useState('');
  const [driver, setDriver] = useState('');
  const [drivers, setDrivers] = useState<string[]>([]);
  const [baseURL, setBaseURL] = useState('');
  const [saving, setSaving] = useState(false);
  const [loginState, setLoginState] = useState<'idle' | 'login'>('idle');
  const [forever, setForever] = useState<ForeverStatus | null>(null);
  const [loginError, setLoginError] = useState('');
  const finished = useRef(false);

  useEffect(() => {
    void api
      .ProviderCatalog()
      .then((items) => {
        setCatalog(items);
        const first = items.find((item) => !item.configured) ?? items[0];
        if (first) setName(first.name);
      })
      .catch((err) => store.toast('error', errorText(err)));
  }, []);

  useEffect(() => {
    void api
      .ProviderDrivers()
      .then((items) => {
        setDrivers(items);
        setDriver((current) => current || (items.includes('openai') ? 'openai' : items[0]) || '');
      })
      .catch((err) => store.toast('error', errorText(err)));
  }, []);

  useEffect(() => {
    if (loginState !== 'login') return;
    return onPeggStatus((next) => {
      setForever(next);
      if (next.state === 'done' && !finished.current) {
        finished.current = true;
        void onAdded()
          .then(onClose)
          .catch((err) => setLoginError(errorText(err)));
      }
    });
  }, [loginState, onAdded, onClose]);

  const entry = catalog.find((item) => item.name === name);
  const subscription = entry?.kind === 'subscription';
  const isForever = name === 'pegg-forever';
  const isCustom = name === CUSTOM_PROVIDER;
  const canAdd = isCustom
    ? Boolean(customName.trim() && driver && baseURL.trim())
    : Boolean(name);

  const add = async () => {
    if (isCustom) {
      if (!customName.trim()) {
        store.toast('warn', 'A provider name is required');
        return;
      }
      if (!driver) {
        store.toast('warn', 'A driver is required for custom providers');
        return;
      }
      if (!baseURL.trim()) {
        store.toast('warn', 'A base URL is required for custom providers');
        return;
      }
      setSaving(true);
      try {
        const custom = customName.trim();
        await api.AddCustomProvider(custom, driver, apiKey, baseURL.trim());
        await onAdded();
        store.toast('success', `${custom} added`);
        onClose();
      } catch (err) {
        store.toast('error', errorText(err));
      } finally {
        setSaving(false);
      }
      return;
    }
    if (!name) return;
    if (!subscription && !apiKey.trim()) {
      store.toast('warn', 'An API key is required for this provider');
      return;
    }
    setSaving(true);
    try {
      await api.AddProvider(name, apiKey, '');
      await onAdded();
      store.toast('success', `${name} added`);
      onClose();
    } catch (err) {
      store.toast('error', errorText(err));
    } finally {
      setSaving(false);
    }
  };

  const startLogin = async () => {
    setLoginError('');
    finished.current = false;
    setForever({ state: 'starting', logs: [], configured: false, provider: 'pegg-forever' });
    setLoginState('login');
    try {
      const next = await api.PeggLogin();
      setForever(next);
      if (next.state === 'done' && !finished.current) {
        finished.current = true;
        await onAdded();
        onClose();
      }
    } catch (err) {
      setLoginError(errorText(err));
      setForever((current) => ({
        ...(current ?? { logs: [], configured: false, provider: 'pegg-forever' }),
        state: 'error',
        error: errorText(err),
      }));
    }
  };

  const cancelLogin = async () => {
    try {
      await api.PeggCancel();
    } catch (err) {
      setLoginError(errorText(err));
    }
    setLoginState('idle');
    setForever(null);
    setLoginError('');
  };

  const loginLabel =
    forever?.state === 'approving'
      ? t('onboarding.approving')
      : forever?.state === 'done'
        ? t('onboarding.done')
        : forever?.state === 'waiting'
          ? t('onboarding.waiting')
          : t('onboarding.starting');

  return (
    <div className="overlay" onMouseDown={(event) => event.target === event.currentTarget && onClose()}>
      <div className="modal provider-modal" role="dialog" aria-modal="true" aria-label="Add provider">
        <div className="modal-head">
          <span className="modal-title">Add provider</span>
          <div className="spacer" />
          <button className="icon-btn" aria-label="Close" onClick={onClose}>
            <X size={15} />
          </button>
        </div>
        <div className="settings-pane" style={{ padding: 22 }}>
          <div className="settings-grid">
            <Field label="Provider">
              <select className="select" value={name} onChange={(event) => setName(event.target.value)}>
                {catalog.map((item) => (
                  <option key={item.name} value={item.name}>
                    {item.name}
                    {item.configured ? ' (configured)' : ''}
                  </option>
                ))}
                <option value={CUSTOM_PROVIDER}>Custom endpoint…</option>
              </select>
            </Field>
            {isCustom ? (
              <>
                <Field label="Name" hint="Unique name for this endpoint.">
                  <input
                    className="input"
                    value={customName}
                    placeholder="my-gateway"
                    onChange={(event) => setCustomName(event.target.value)}
                  />
                </Field>
                <Field label="Driver" hint="Protocol used to reach the endpoint.">
                  <select className="select" value={driver} onChange={(event) => setDriver(event.target.value)}>
                    {drivers.map((item) => (
                      <option key={item} value={item}>{item}</option>
                    ))}
                  </select>
                </Field>
                <Field label="Base URL" hint="Required for custom endpoints.">
                  <input
                    className="input"
                    value={baseURL}
                    placeholder="https://api.example.com/v1"
                    onChange={(event) => setBaseURL(event.target.value)}
                  />
                </Field>
                <Field label="API key" hint="Optional for local or keyless endpoints.">
                  <input
                    className="input"
                    type="password"
                    autoComplete="off"
                    value={apiKey}
                    placeholder="sk-… (optional)"
                    onChange={(event) => setApiKey(event.target.value)}
                  />
                </Field>
              </>
            ) : (
              <>
                <Field label="API key" hint={subscription ? 'Optional for subscription providers.' : 'Stored in ~/.pegg/pegg.json.'}>
                  <input
                    className="input"
                    type="password"
                    autoComplete="off"
                    value={apiKey}
                    placeholder={entry?.kind === 'subscription' ? 'Use pegg login if not signed in' : 'sk-…'}
                    onChange={(event) => setApiKey(event.target.value)}
                  />
                </Field>
                <Field label="Type">
                  <div className="settings-row-value">{entry?.kind === 'subscription' ? 'Subscription / OAuth' : 'API key'}</div>
                </Field>
              </>
            )}
          </div>
          {entry?.kind === 'subscription' && entry.hint ? (
            <p className="settings-footnote">{entry.hint}</p>
          ) : null}
          {isCustom ? (
            <p className="settings-footnote">
              Custom endpoints require a driver and base URL. Add a name you can recognize in the model picker.
            </p>
          ) : null}
          {isForever ? (
            <p className="settings-footnote">
              Sign in with your browser, or paste an API key below.
            </p>
          ) : null}
          {isForever && loginState === 'login' ? (
            <div className="settings-list" style={{ marginTop: 10 }}>
              <div className="settings-list-row">
                <div className="row-main">
                  <div className="row-name">{loginLabel}</div>
                  {forever?.url ? (
                    <div className="row-desc" style={{ overflowWrap: 'anywhere' }}>
                      <code>{forever.url}</code>
                    </div>
                  ) : null}
                </div>
                <div className="row-side">
                  {forever?.url ? (
                    <button className="btn small" onClick={() => void api.OpenExternal(forever.url ?? '')}>
                      {t('onboarding.openBrowser')}
                    </button>
                  ) : null}
                  <button className="btn small" onClick={() => void cancelLogin()}>
                    {t('onboarding.cancel')}
                  </button>
                </div>
              </div>
              {forever?.error || loginError ? (
                <div className="notice error">
                  {t('onboarding.error', { error: forever?.error || loginError })}
                </div>
              ) : null}
            </div>
          ) : null}
          <div className="settings-save-bar">
            <button className="btn ghost" onClick={onClose}>Cancel</button>
            {isForever && loginState !== 'login' ? (
              <button className="btn" disabled={saving} onClick={() => void startLogin()}>
                <LogIn size={13} />
                {t('onboarding.login')}
              </button>
            ) : null}
            <span className="spacer" />
            <button
              className="btn primary"
              disabled={saving || !canAdd || loginState === 'login'}
              onClick={() => void add()}
            >
              {saving ? 'Adding…' : 'Add provider'}
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}

function AddSTTProviderModal({
  entries,
  onClose,
  onAdded,
}: {
  entries: STTProviderEntry[];
  onClose: () => void;
  onAdded: () => Promise<void>;
}) {
  const [name, setName] = useState('');
  const [apiKey, setApiKey] = useState('');
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    const first = entries.find((entry) => !entry.configured) ?? entries[0];
    if (first) setName(first.name);
  }, [entries]);

  const entry = entries.find((item) => item.name === name);
  const requiresKey = Boolean(entry && !entry.name.startsWith('pegg-team'));

  const add = async () => {
    if (!name) return;
    if (requiresKey && !apiKey.trim()) {
      store.toast('warn', 'An API key is required for this STT provider');
      return;
    }
    setSaving(true);
    try {
      await api.AddSTTProvider(name, apiKey, '');
      await onAdded();
      store.toast('success', `${name} added`);
      onClose();
    } catch (err) {
      store.toast('error', errorText(err));
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="overlay" onMouseDown={(event) => event.target === event.currentTarget && onClose()}>
      <div className="modal provider-modal" role="dialog" aria-modal="true" aria-label="Add STT provider">
        <div className="modal-head">
          <span className="modal-title">Add speech-to-text provider</span>
          <div className="spacer" />
          <button className="icon-btn" aria-label="Close" onClick={onClose}>
            <X size={15} />
          </button>
        </div>
        <div className="settings-pane" style={{ padding: 22 }}>
          <div className="settings-grid">
            <Field label="Provider">
              <select className="select" value={name} onChange={(event) => setName(event.target.value)}>
                {entries.map((item) => (
                  <option key={item.name} value={item.name}>
                    {item.name}
                    {item.configured ? ' (configured)' : ''}
                  </option>
                ))}
              </select>
            </Field>
            <Field label="API key" hint="Stored in ~/.pegg/pegg.json.">
              <input
                className="input"
                type="password"
                autoComplete="off"
                value={apiKey}
                placeholder="Paste provider key…"
                onChange={(event) => setApiKey(event.target.value)}
              />
            </Field>
            <Field label="Models">
              <div className="settings-row-value">{entry?.models.join(', ') || 'Provider default'}</div>
            </Field>
          </div>
          <div className="settings-save-bar">
            <button className="btn ghost" onClick={onClose}>Cancel</button>
            <span className="spacer" />
            <button className="btn primary" disabled={saving || !name} onClick={() => void add()}>
              {saving ? 'Adding…' : 'Add provider'}
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}

function SystemPanel({ snap }: { snap: SettingsSnapshot }) {
  const t = useT();
  const binary = useStore((s) => s.binary);
  const installing = useStore((s) => s.installing);
  const progress = useStore((s) => s.installProgress);
  const selfUpdate = useStore((s) => s.selfUpdate);
  const system = snap.system;

  const row = (label: string, value: string, reveal?: string) => (
    <div className="path-row" key={label}>
      <span className="path-label">{label}</span>
      <span className="path-value" title={value}>{value || '—'}</span>
      {reveal ? (
        <button className="btn small ghost" onClick={() => void api.RevealPath(reveal)}>
          <FolderOpen size={12} />
          Reveal
        </button>
      ) : null}
    </div>
  );

  return (
    <>
      <PageHead title={t('settings.system')} description="Versions, runtime paths, and the Pegg agent binary." />
      <Card title="Versions">
        <div className="settings-grid">
          <Field label="Pegg Desktop">
            <div className="settings-row-value">{system.appVersion}</div>
          </Field>
          <Field label="Pegg agent">
            <div className="settings-row-value">{system.agentVersion || system.binaryVersion || 'Unknown'}</div>
          </Field>
          <Field label="Platform">
            <div className="settings-row-value">{system.platform}/{system.arch}</div>
          </Field>
          <Field label="Binary">
            <div className="settings-row-value">
              {binary.version ? `${binary.version} · ${binary.source || 'found'}` : 'Not resolved'}
            </div>
          </Field>
        </div>
        <div className="settings-row">
          <div className="info">
            <div className="name">Resolve binary</div>
            <div className="desc">{binary.path || binary.error || 'No binary selected.'}</div>
          </div>
          <div className="control">
            <button className="btn small" onClick={() => void findBinary()}>Find</button>
            <button className="btn small" onClick={() => void chooseBinary()}>Locate</button>
            <button className="btn small primary" disabled={installing} onClick={() => void installBinary()}>
              <Download size={13} />
              {installing ? 'Installing…' : 'Update agent'}
            </button>
          </div>
        </div>
        {installing && progress ? (
          <div style={{ marginTop: 8 }}>
            <div className="progress-track">
              <div className="progress-fill" style={{ width: `${progress.percent}%` }} />
            </div>
            <div className="settings-footnote">{progress.stage} {progress.detail ? `· ${progress.detail}` : ''}</div>
          </div>
        ) : null}
      </Card>
      <Card title={t('update.title')}>
        <div className="settings-grid">
          <Field label={`${t('update.desktop')} · ${t('update.current')}`}>
            <div className="settings-row-value">{selfUpdate.currentVersion || system.appVersion}</div>
          </Field>
          <Field label={`${t('update.desktop')} · ${t('update.latest')}`}>
            <div className="settings-row-value">{selfUpdate.latestVersion || '—'}</div>
          </Field>
          <Field label={`${t('update.agent')} · ${t('update.current')}`}>
            <div className="settings-row-value">{selfUpdate.agentCurrentVersion || system.binaryVersion || '—'}</div>
          </Field>
          <Field label={`${t('update.agent')} · ${t('update.latest')}`}>
            <div className="settings-row-value">{selfUpdate.agentLatestVersion || '—'}</div>
          </Field>
        </div>
        {selfUpdate.state === 'downloading' ? (
          <div style={{ marginTop: 8 }}>
            <div className="progress-track">
              <div className="progress-fill" style={{ width: `${selfUpdate.progress}%` }} />
            </div>
            <div className="settings-footnote">
              {t('update.downloadingAll', { progress: selfUpdate.progress })}
            </div>
          </div>
        ) : null}
        {selfUpdate.error ? (
          <div className="notice error" style={{ marginTop: 8 }}>
            {t('update.error', { error: selfUpdate.error })}
          </div>
        ) : null}
        <div className="settings-row">
          <div className="info">
            <div className="name">{updateSummary(selfUpdate, t)}</div>
            <div className="desc">
              {selfUpdate.targetPath || ''}
              {updateUnsupportedAgent(selfUpdate)
                ? ` · ${t('update.agentManaged', { version: selfUpdate.agentLatestVersion ?? '' })}`
                : ''}
            </div>
          </div>
          <div className="control">
            <button
              className="btn small"
              disabled={selfUpdate.state === 'checking' || selfUpdate.state === 'downloading'}
              onClick={() => void checkDesktopUpdate()}
            >
              {selfUpdate.state === 'checking' ? t('update.checking') : t('update.check')}
            </button>
            {updateDownloads(selfUpdate) && selfUpdate.state !== 'downloading' ? (
              <button className="btn small primary" onClick={() => void downloadDesktopUpdate()}>
                {t('update.download')}
              </button>
            ) : null}
            {selfUpdate.ready ? (
              <button className="btn small primary" onClick={() => void restartToUpdate()}>
                {t('update.restart')}
              </button>
            ) : null}
            {updateUnsupportedDesktop(selfUpdate) && selfUpdate.releaseUrl ? (
              <button className="btn small" onClick={() => void api.OpenExternal(selfUpdate.releaseUrl as string)}>
                {t('update.open')}
              </button>
            ) : null}
          </div>
        </div>
      </Card>
      <Card title="Paths">
        {row('Config', system.configPath, system.configPath)}
        {row('Config dir', system.configDir, system.configDir)}
        {row('Project', system.projectDir, system.projectDir)}
        {row('Log', system.logPath, system.logPath)}
        {row('Memory', system.memoryDir, system.memoryDir)}
        {row('Skills', system.skillsDir, system.skillsDir)}
        {row('Rules', system.rulesDir, system.rulesDir)}
        {row('Plugins', system.pluginsDir, system.pluginsDir)}
      </Card>
      <Card title="About">
        <div className="settings-row">
          <div className="info">
            <div className="name">Pegg Desktop</div>
            <div className="desc">A native ACP client for the Pegg coding agent.</div>
          </div>
          <div className="control">
            <button className="btn small" onClick={() => void api.OpenExternal('https://pegg.dev')}>
              <Info size={13} />
              pegg.dev
            </button>
          </div>
        </div>
        <div className="settings-row">
          <div className="info">
            <div className="name">Keyboard</div>
            <div className="desc">Ctrl/Cmd+K commands · Ctrl/Cmd+N new chat · Ctrl/Cmd+, settings</div>
          </div>
        </div>
      </Card>
    </>
  );
}
