import { useCallback, useEffect, useMemo, useState } from 'react';
import {
  AppWindow,
  BookOpen,
  Brain,
  Download,
  FolderOpen,
  Info,
  KeyRound,
  MessageSquare,
  Palette,
  Plug,
  Puzzle,
  Search,
  Settings2,
  ShieldCheck,
  Sparkles,
  Trash2,
  X,
} from 'lucide-react';
import { api } from '../lib/bridge';
import {
  changeModel,
  changeReasoning,
  chooseBinary,
  errorText,
  findBinary,
  installBinary,
  loadModels,
  savePrefs,
  setTheme,
  toggleSettings,
} from '../state/actions';
import { store, useStore } from '../state/store';
import { useT } from '../lib/i18n';
import type { ProviderCatalogEntry, STTProviderEntry, SettingsSnapshot, Theme } from '../types';
import {
  MCPPanel,
  MemoryPanel,
  PermissionsPanel,
  PluginsPanel,
  RulesPanel,
  SessionPanel,
  SkillsPanel,
} from './SettingsPanels';
import { Card, Field, PageHead, Range, Segmented } from './SettingsUI';

type Tab =
  | 'general'
  | 'appearance'
  | 'session'
  | 'permissions'
  | 'memory'
  | 'mcp'
  | 'skills'
  | 'rules'
  | 'plugins'
  | 'system';

const TAB_GROUPS: Array<{ label: string; tabs: Array<{ id: Tab; label: string; icon: typeof Settings2 }> }> = [
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
    label: 'settings.systemGroup',
    tabs: [{ id: 'system', label: 'settings.system', icon: AppWindow }],
  },
];

export function Settings() {
  const open = useStore((s) => s.settingsOpen);
  const [tab, setTab] = useState<Tab>('general');
  const [snap, setSnap] = useState<SettingsSnapshot | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const t = useT();
  const models = useStore((state) => state.models);

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

  useEffect(() => {
    void refresh();
  }, [refresh]);

  useEffect(() => {
    if (!open) return;
    if (!snap) void refresh();
    if (!models.length) void loadModels();
  }, [open, refresh, snap, models.length]);

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
                  {tab === 'plugins' ? <PluginsPanel snap={snap} refresh={refresh} /> : null}
                  {tab === 'system' ? <SystemPanel snap={snap} /> : null}
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
  const themes = useStore((s) => s.themes);
  const current = useStore((s) => s.theme);
  const prefs = useStore((s) => s.prefs);
  const [query, setQuery] = useState('');
  const filtered = themes.filter((theme) => theme.label.toLowerCase().includes(query.toLowerCase()));

  return (
    <>
      <PageHead
        title={t('settings.appearance')}
        description="Every color palette comes from the terminal UI. Auto follows the system light/dark preference."
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
              onChange={(mode) => void setTheme(prefs.theme, mode)}
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
      <Card title={`Themes · ${themes.length}`}>
        <div style={{ position: 'relative', marginBottom: 10 }}>
          <Search size={13} style={{ position: 'absolute', left: 10, top: 10, color: 'var(--pg-muted)' }} />
          <input
            className="input"
            style={{ width: '100%', paddingLeft: 30 }}
            placeholder="Search themes…"
            value={query}
            onChange={(event) => setQuery(event.target.value)}
          />
        </div>
        <div className="theme-grid">
          {filtered.map((theme) => (
            <ThemeCard
              key={theme.name}
              theme={theme}
              active={current?.name === theme.name}
              onClick={() => void setTheme(theme.name, prefs.themeMode)}
            />
          ))}
        </div>
      </Card>
    </>
  );
}

function ThemeCard({ theme, active, onClick }: { theme: Theme; active: boolean; onClick: () => void }) {
  const tokens = theme.tokens;
  return (
    <button className={`theme-card${active ? ' active' : ''}`} onClick={onClick} title={theme.name}>
      <div className="theme-preview" style={{ background: tokens['--pg-bg'] }}>
        <div className="p-side" style={{ background: tokens['--pg-sidebar'] }} />
        <div className="p-main" style={{ background: tokens['--pg-surface'] }}>
          <div className="p-line" style={{ background: tokens['--pg-text'], opacity: 0.72 }} />
          <div className="p-line short" style={{ background: tokens['--pg-accent'] }} />
          <div className="p-line" style={{ background: tokens['--pg-muted'], opacity: 0.55 }} />
        </div>
      </div>
      <div className="theme-name">
        <span>{theme.label}</span>
        {active ? <span style={{ color: 'var(--pg-accent)' }}>✓</span> : null}
      </div>
    </button>
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
  const [addProviderOpen, setAddProviderOpen] = useState(false);
  const [addSttOpen, setAddSttOpen] = useState(false);

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
    try {
      await api.SaveGeneralSettings(draft);
      store.set({ language: draft.language });
      document.documentElement.lang = draft.language;
      await refresh();
      store.toast('success', 'General settings saved');
    } catch (err) {
      store.toast('error', errorText(err));
    } finally {
      setSaving(false);
    }
  };

  return (
    <>
      <PageHead
        title={t('settings.general')}
        description="Model, language, and voice input. Model changes apply to the current session immediately."
      />
      <Card title="Model">
        <div className="settings-grid">
          <Field label="Provider">
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
              {providerNames.length ? null : <option value="">No providers configured</option>}
              {providerNames.map((provider) => (
                <option key={provider} value={provider}>{provider}</option>
              ))}
            </select>
          </Field>
          <Field label="Model">
            <select
              className="select"
              value={currentModel}
              onChange={(event) => {
                const model = event.target.value;
                setDraft((current) => ({ ...current, model }));
                void changeModel(currentProvider, model);
              }}
            >
              {providerModels.length ? null : <option value="">Provider default</option>}
              {providerModels.map((model) => (
                <option key={model.id} value={model.id}>{model.name || model.id}</option>
              ))}
            </select>
          </Field>
          <Field
            label="Reasoning effort"
            hint={
              effortOptions.length
                ? 'Provider-supported effort levels for this model.'
                : 'This model does not declare reasoning effort options.'
            }
          >
            <select
              className="select"
              value={reasoning ?? ''}
              onChange={(event) => void changeReasoning(event.target.value)}
            >
              <option value="">Default</option>
              {effortOptions.map((effort) => (
                <option key={effort} value={effort}>{effort}</option>
              ))}
            </select>
          </Field>
          <Field label="Language">
            <select
              className="select"
              value={draft.language}
              onChange={(event) => setDraft((current) => ({ ...current, language: event.target.value }))}
            >
              {draft.languages.map((language) => (
                <option key={language.code} value={language.code}>{language.label}</option>
              ))}
            </select>
          </Field>
        </div>
      </Card>

      <Card
        title={`Providers · ${snap.general.providers.length}`}
        actions={
          <button className="btn small" onClick={() => setAddProviderOpen(true)}>
            <KeyRound size={13} />
            Add provider
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
                      return `${count} model${count === 1 ? '' : 's'}`;
                    })()}
                  </div>
                </div>
                <div className="row-side">
                  <button
                    className="btn small ghost"
                    title={`Remove ${provider.name}`}
                    onClick={() => {
                      void api
                        .RemoveProvider(provider.name)
                        .then(async () => {
                          await refresh();
                          void loadModels();
                          store.toast('success', `Removed ${provider.name}`);
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
          <div className="empty-state">No providers configured. Add one to start a session.</div>
        )}
      </Card>

      <Card
        title={`Speech-to-text providers · ${snap.general.stt.entries.length}`}
        actions={
          <button className="btn small" onClick={() => setAddSttOpen(true)}>
            <KeyRound size={13} />
            Add STT provider
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
                    {entry.models.length} model{entry.models.length === 1 ? '' : 's'}
                  </div>
                </div>
                <div className="row-side">
                  <span className={`badge${entry.configured ? ' accent' : ''}`}>
                    {entry.configured ? 'configured' : 'not configured'}
                  </span>
                  {entry.configured ? (
                    <button
                      className="btn small ghost"
                      title={`Remove ${entry.name}`}
                      onClick={() => {
                        void api
                          .RemoveSTTProvider(entry.name)
                          .then(async () => {
                            await refresh();
                            store.toast('success', `Removed ${entry.name}`);
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
          <div className="empty-state">No speech-to-text providers found.</div>
        )}
      </Card>

      <Card title="Voice input">
        <div className="settings-grid">
          <Field label="Speech-to-text provider">
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
              <option value="">Not configured</option>
              {draft.stt.configured.map((provider) => (
                <option key={provider} value={provider}>{provider}</option>
              ))}
            </select>
          </Field>
          <Field label="Speech-to-text model">
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
              <option value="">Provider default</option>
              {draft.stt.models.map((model) => (
                <option key={model} value={model}>{model}</option>
              ))}
            </select>
          </Field>
          <Field label="Record command" hint="Optional custom recorder. Leave empty to use the platform default.">
            <input
              className="input"
              value={draft.stt.recordCommand}
              placeholder="e.g. arecord -q -f S16_LE -r 16000 -c 1 -t wav"
              onChange={(event) =>
                setDraft((current) => ({
                  ...current,
                  stt: { ...current.stt, recordCommand: event.target.value },
                }))
              }
            />
          </Field>
          <Field label="Max recording duration" hint="Seconds before the microphone stops.">
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
              ? 'STT credentials are managed by the Pegg CLI/config.'
              : 'No speech-to-text provider is configured. Run `pegg login` or set stt in ~/.pegg/pegg.json.'}
          </span>
          <span className="spacer" />
          <button className="btn primary" disabled={saving} onClick={() => void save()}>
            {saving ? 'Saving…' : 'Save changes'}
          </button>
        </div>
      </Card>
      {addProviderOpen ? (
        <AddProviderModal
          onClose={() => setAddProviderOpen(false)}
          onAdded={async () => {
            await refresh();
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
  const [catalog, setCatalog] = useState<ProviderCatalogEntry[]>([]);
  const [name, setName] = useState('');
  const [apiKey, setApiKey] = useState('');
  const [baseURL, setBaseURL] = useState('');
  const [saving, setSaving] = useState(false);

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

  const entry = catalog.find((item) => item.name === name);
  const subscription = entry?.kind === 'subscription';

  const add = async () => {
    if (!name) return;
    if (!subscription && !apiKey.trim()) {
      store.toast('warn', 'An API key is required for this provider');
      return;
    }
    setSaving(true);
    try {
      await api.AddProvider(name, apiKey, baseURL);
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
              </select>
            </Field>
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
            <Field label="Base URL" hint="Optional. Leave empty for the provider default.">
              <input
                className="input"
                value={baseURL}
                placeholder="https://api.example.com/v1"
                onChange={(event) => setBaseURL(event.target.value)}
              />
            </Field>
            <Field label="Type">
              <div className="settings-row-value">{entry?.kind === 'subscription' ? 'Subscription / OAuth' : 'API key'}</div>
            </Field>
          </div>
          {entry?.kind === 'subscription' && entry.hint ? (
            <p className="settings-footnote">{entry.hint}</p>
          ) : null}
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
  const [baseURL, setBaseURL] = useState('');
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
      await api.AddSTTProvider(name, apiKey, baseURL);
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
            <Field label="Base URL" hint="Optional. Leave empty for the provider default.">
              <input
                className="input"
                value={baseURL}
                placeholder="https://api.example.com/v1"
                onChange={(event) => setBaseURL(event.target.value)}
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
              {installing ? 'Installing…' : 'Update Pegg'}
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
