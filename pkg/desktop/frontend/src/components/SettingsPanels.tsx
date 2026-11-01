import { useEffect, useMemo, useState } from 'react';
import { Pencil, Plus, Trash2 } from 'lucide-react';
import { api } from '../lib/bridge';
import { changeMode, errorText, startNewChat } from '../state/actions';
import { store, useStore } from '../state/store';
import { useT } from '../lib/i18n';
import type { CompactionSettings, PermissionsSettings, SettingsSnapshot } from '../types';
import { Card, Empty, Field, PageHead, Range, SaveBar, Segmented, Switch } from './SettingsUI';
import { MCPModal } from './MCPModal';

export interface PanelProps {
  snap: SettingsSnapshot;
  refresh: () => Promise<void>;
}

export function SessionPanel({ snap, refresh }: PanelProps) {
  const t = useT();
  const session = snap.session;
  const running = useStore((s) => s.chat.running);
  const usage = useStore((s) => s.chat.usage);
  const mode = useStore((s) => s.chat.mode);
  const [title, setTitle] = useState(session.title);
  const [draft, setDraft] = useState<CompactionSettings>(snap.compaction);
  const [saving, setSaving] = useState(false);
  const [confirmDelete, setConfirmDelete] = useState(false);

  useEffect(() => setTitle(session.title), [session.title]);
  useEffect(() => setDraft(snap.compaction), [snap.compaction]);

  const rename = async () => {
    if (!session.acpId || !title.trim() || title === session.title) return;
    try {
      await api.RenameSession(session.acpId, title.trim());
      store.setChat((chat) => ({ ...chat, title: title.trim() }));
      await refresh();
      store.toast('success', 'Session renamed');
    } catch (err) {
      store.toast('error', errorText(err));
    }
  };

  const remove = async () => {
    if (!session.peggId) return;
    try {
      await api.DeleteSession(session.peggId);
      startNewChat();
      await refresh();
      store.toast('success', 'Session deleted');
    } catch (err) {
      store.toast('error', errorText(err));
    }
  };

  const saveCompaction = async () => {
    setSaving(true);
    try {
      await api.SaveCompactionSettings(draft);
      await refresh();
      store.toast('success', 'Context settings saved');
    } catch (err) {
      store.toast('error', errorText(err));
    } finally {
      setSaving(false);
    }
  };

  const toggleStrategy = (name: string) => {
    setDraft((current) => ({
      ...current,
      strategies: current.strategies.includes(name)
        ? current.strategies.filter((item) => item !== name)
        : [...current.strategies, name],
    }));
  };

  return (
    <>
      <PageHead
        title={t('settings.session')}
        description="The active conversation, its model, and how Pegg keeps long contexts under control."
      />
      <Card
        title="Current session"
        actions={
          <button
            className="btn small"
            onClick={() => {
              startNewChat();
              void refresh();
              store.toast('info', 'Started a new chat');
            }}
          >
            <Plus size={13} />
            New chat
          </button>
        }
      >
        {session.acpId ? (
          <>
            <div className="settings-grid">
              <Field label="Title">
                <div style={{ display: 'flex', gap: 6 }}>
                  <input className="input" value={title} onChange={(event) => setTitle(event.target.value)} />
                  <button className="btn small" onClick={() => void rename()} title="Rename session">
                    <Pencil size={13} />
                  </button>
                </div>
              </Field>
              <Field label="Mode">
                <Segmented
                  value={(mode as 'auto' | 'plan' | 'build') || 'auto'}
                  options={[
                    { value: 'auto', label: 'Auto' },
                    { value: 'plan', label: 'Plan' },
                    { value: 'build', label: 'Build' },
                  ]}
                  onChange={(next) => void changeMode(next)}
                />
              </Field>
              <Field label="Model">
                <div className="settings-row-value">
                  {session.provider ? `${session.provider} / ` : ''}
                  {session.model || 'Provider default'}
                </div>
              </Field>
              <Field label="Workspace">
                <div className="settings-row-value" title={session.cwd}>
                  {session.cwd}
                </div>
              </Field>
              <Field label="Status">
                <div className="settings-row-value">
                  {running ? 'Running' : 'Idle'}
                  {usage.used > 0 ? ` · ${usage.used.toLocaleString()} tokens` : ''}
                </div>
              </Field>
              <Field label="Session ID">
                <div className="settings-row-value settings-mono">{session.peggId}</div>
              </Field>
            </div>
            <div className="settings-save-bar">
              <button className="btn small danger" onClick={() => setConfirmDelete(true)}>
                <Trash2 size={13} />
                Delete session
              </button>
              {confirmDelete ? (
                <>
                  <span className="settings-footnote">Delete this session permanently?</span>
                  <button className="btn small danger" onClick={() => void remove()}>
                    Confirm
                  </button>
                  <button className="btn small ghost" onClick={() => setConfirmDelete(false)}>
                    Cancel
                  </button>
                </>
              ) : null}
            </div>
          </>
        ) : (
          <Empty>No active session. Start a chat to configure its title, mode, and model.</Empty>
        )}
      </Card>
      <Card title="Context compaction">
        <div className="settings-grid">
          <div className="settings-row">
            <div className="info">
              <div className="name">Enable compaction</div>
              <div className="desc">Summarize or trim history before the context window fills.</div>
            </div>
            <div className="control">
              <Switch
                checked={draft.enabled}
                onChange={(enabled) => setDraft((current) => ({ ...current, enabled }))}
              />
            </div>
          </div>
          <Field label="Threshold" hint="Percentage of the context window before compaction starts.">
            <Range
              min={10}
              max={100}
              step={5}
              value={draft.threshold}
              onChange={(threshold) => setDraft((current) => ({ ...current, threshold }))}
              format={(value) => `${value}%`}
            />
          </Field>
          <Field label="Max messages" hint="Sliding-window upper bound.">
            <input
              className="input"
              type="number"
              min={5}
              value={draft.maxMessages}
              onChange={(event) => setDraft((current) => ({ ...current, maxMessages: Number(event.target.value) }))}
            />
          </Field>
          <Field label="Max tool output" hint="Characters kept from each tool result.">
            <input
              className="input"
              type="number"
              min={500}
              step={500}
              value={draft.maxToolOutputChars}
              onChange={(event) =>
                setDraft((current) => ({ ...current, maxToolOutputChars: Number(event.target.value) }))
              }
            />
          </Field>
        </div>
        <div className="settings-save-bar">
          <span className="settings-card-title">Strategies</span>
          <div className="settings-chip-row">
            {draft.allStrategies.map((name) => (
              <button
                key={name}
                className={`chip${draft.strategies.includes(name) ? ' selected' : ''}`}
                onClick={() => toggleStrategy(name)}
              >
                {name}
              </button>
            ))}
          </div>
        </div>
        <SaveBar onSave={() => void saveCompaction()} saving={saving} label="Save context settings" />
      </Card>
    </>
  );
}

const AUTO_RULES: Record<string, string> = {
  read: 'allow', write: 'allow', edit: 'allow', delete: 'allow', list: 'allow',
  glob: 'allow', grep: 'allow', bash: 'allow', todo: 'allow', todoread: 'allow',
  todowrite: 'allow', webfetch: 'allow', websearch: 'allow', task: 'allow',
  taskstatus: 'allow', askuserquestion: 'allow', enterplanmode: 'allow', exitplanmode: 'allow',
};

const MANUAL_RULES = Object.fromEntries(Object.keys(AUTO_RULES).map((key) => [key, 'semi-ask']));
const JUDGE_RULES = Object.fromEntries(Object.keys(AUTO_RULES).map((key) => [key, 'semi-judge']));

export function PermissionsPanel({ snap, refresh }: PanelProps) {
  const t = useT();
  const [draft, setDraft] = useState<PermissionsSettings>(snap.permissions);
  const [saving, setSaving] = useState(false);
  useEffect(() => setDraft(snap.permissions), [snap.permissions]);

  const groups = useMemo(() => {
    const map = new Map<string, typeof draft.tools>();
    for (const tool of draft.tools) {
      const list = map.get(tool.group) ?? [];
      list.push(tool);
      map.set(tool.group, list);
    }
    return Array.from(map.entries());
  }, [draft.tools]);

  const setToolMode = (name: string, mode: string) => {
    setDraft((current) => ({
      ...current,
      tools: current.tools.map((tool) => (tool.name === name ? { ...tool, mode } : tool)),
    }));
  };

  const applyPreset = (kind: 'auto' | 'manual' | 'judge' | 'recommended') => {
    const source =
      kind === 'recommended'
        ? draft.defaults
        : kind === 'auto'
          ? AUTO_RULES
          : kind === 'manual'
            ? MANUAL_RULES
            : JUDGE_RULES;
    const fallback = kind === 'auto' ? 'allow' : kind === 'judge' ? 'semi-judge' : 'semi-ask';
    setDraft((current) => ({
      ...current,
      tools: current.tools.map((tool) => ({ ...tool, mode: source[tool.name] ?? fallback })),
    }));
  };

  const save = async () => {
    setSaving(true);
    try {
      await api.SavePermissionSettings(draft);
      await refresh();
      store.toast('success', 'Permission rules saved');
    } catch (err) {
      store.toast('error', errorText(err));
    } finally {
      setSaving(false);
    }
  };

  return (
    <>
      <PageHead
        title={t('settings.permissions')}
        description="Choose how Pegg asks before reading, editing, running, or reaching the network."
      />
      <Card title="Preset">
        <div className="preset-row">
          <button className="btn small" onClick={() => applyPreset('auto')}>Auto</button>
          <button className="btn small" onClick={() => applyPreset('manual')}>Manual</button>
          <button className="btn small" onClick={() => applyPreset('judge')}>Judge</button>
          <button className="btn small" onClick={() => applyPreset('recommended')}>Recommended</button>
        </div>
        <p className="settings-footnote">
          Presets fill the tool rules below. You can adjust any tool afterwards.
        </p>
      </Card>
      <Card title="Tool rules">
        <div className="settings-grid">
          <Field label="Default mode">
            <select
              className="select"
              value={draft.default}
              onChange={(event) => setDraft((current) => ({ ...current, default: event.target.value }))}
            >
              {draft.modes.map((mode) => (
                <option key={mode} value={mode}>{mode}</option>
              ))}
            </select>
          </Field>
          <Field label="Judge provider" hint="Used by semi-judge and judge modes.">
            <select
              className="select"
              value={draft.judgeProvider}
              onChange={(event) =>
                setDraft((current) => ({ ...current, judgeProvider: event.target.value }))
              }
            >
              <option value="">None</option>
              {draft.judgeProviders.map((provider) => (
                <option key={provider} value={provider}>{provider}</option>
              ))}
            </select>
          </Field>
          <Field label="Judge model">
            <input
              className="input"
              value={draft.judgeModel}
              placeholder="Provider default"
              onChange={(event) => setDraft((current) => ({ ...current, judgeModel: event.target.value }))}
            />
          </Field>
          <Field label="Judge threshold">
            <Range
              min={0}
              max={1}
              step={0.05}
              value={draft.judgeThreshold}
              onChange={(judgeThreshold) => setDraft((current) => ({ ...current, judgeThreshold }))}
              format={(value) => value.toFixed(2)}
            />
          </Field>
        </div>
        <div className="perm-grid" style={{ marginTop: 12 }}>
          {groups.map(([group, tools]) => (
            <div key={group}>
              <div className="perm-group">{group}</div>
              {tools.map((tool) => (
                <div className="perm-row" key={tool.name}>
                  <span className="tool-name">{tool.name}</span>
                  <select
                    className="select"
                    value={tool.mode}
                    onChange={(event) => setToolMode(tool.name, event.target.value)}
                  >
                    {draft.modes.map((mode) => (
                      <option key={mode} value={mode}>{mode}</option>
                    ))}
                  </select>
                </div>
              ))}
            </div>
          ))}
        </div>
        <SaveBar onSave={() => void save()} saving={saving} />
      </Card>
    </>
  );
}

export function MemoryPanel({ snap, refresh }: PanelProps) {
  const t = useT();
  const system = snap.system;
  const [draft, setDraft] = useState(snap.memory);
  const [saving, setSaving] = useState(false);
  useEffect(() => setDraft(snap.memory), [snap.memory]);

  const save = async () => {
    setSaving(true);
    try {
      await api.SaveMemorySettings(draft);
      await refresh();
      store.toast('success', 'Memory settings saved');
    } catch (err) {
      store.toast('error', errorText(err));
    } finally {
      setSaving(false);
    }
  };

  return (
    <>
      <PageHead
        title={t('settings.memory')}
        description="Pegg can remember durable facts across sessions. Tune when memory is read and how much context it may use."
      />
      <Card title="Memory store">
        <div className="settings-grid">
          <div className="settings-row">
            <div className="info">
              <div className="name">Enable memory</div>
              <div className="desc">Read and write long-term memory for this user.</div>
            </div>
            <div className="control">
              <Switch
                checked={draft.enabled}
                onChange={(enabled) => setDraft((current) => ({ ...current, enabled }))}
              />
            </div>
          </div>
          <div className="settings-row">
            <div className="info">
              <div className="name">Consolidate</div>
              <div className="desc">Merge related entries when new memories arrive.</div>
            </div>
            <div className="control">
              <Switch
                checked={draft.consolidate}
                onChange={(consolidate) => setDraft((current) => ({ ...current, consolidate }))}
              />
            </div>
          </div>
          <Field label="Gate" hint="How memory is selected before a run.">
            <select
              className="select"
              value={draft.gate}
              onChange={(event) => setDraft((current) => ({ ...current, gate: event.target.value }))}
            >
              {draft.gates.map((gate) => (
                <option key={gate} value={gate}>{gate}</option>
              ))}
            </select>
          </Field>
          <Field label="Threshold">
            <Range
              min={0}
              max={1}
              step={0.05}
              value={draft.threshold}
              onChange={(threshold) => setDraft((current) => ({ ...current, threshold }))}
              format={(value) => value.toFixed(2)}
            />
          </Field>
          <Field label="Context budget" hint="Characters of memory injected into a prompt.">
            <input
              className="input"
              type="number"
              min={500}
              step={500}
              value={draft.contextBudget}
              onChange={(event) =>
                setDraft((current) => ({ ...current, contextBudget: Number(event.target.value) }))
              }
            />
          </Field>
          <Field label="Max results">
            <input
              className="input"
              type="number"
              min={1}
              value={draft.maxResults}
              onChange={(event) =>
                setDraft((current) => ({ ...current, maxResults: Number(event.target.value) }))
              }
            />
          </Field>
          <Field label="Provider">
            <input
              className="input"
              value={draft.provider}
              placeholder="Provider default"
              onChange={(event) => setDraft((current) => ({ ...current, provider: event.target.value }))}
            />
          </Field>
          <Field label="Model">
            <input
              className="input"
              value={draft.model}
              placeholder="Provider default"
              onChange={(event) => setDraft((current) => ({ ...current, model: event.target.value }))}
            />
          </Field>
        </div>
        <SaveBar onSave={() => void save()} saving={saving} />
      </Card>
      <Card title="Stats">
        <div style={{ display: 'flex', alignItems: 'center', gap: 28 }}>
          <div className="settings-stat">
            <span className="value">{draft.entries}</span>
            <span className="label">entries</span>
          </div>
          <div className="settings-stat">
            <span className="label">{draft.lastEntry || 'No entries yet'}</span>
          </div>
        </div>
        <div className="settings-save-bar">
          <button className="btn small" onClick={() => void api.RevealPath(system.memoryDir)}>
            Reveal memory folder
          </button>
        </div>
      </Card>
    </>
  );
}

export function MCPPanel({ snap, refresh }: PanelProps) {
  const t = useT();
  const system = snap.system;
  const [addOpen, setAddOpen] = useState(false);

  const remove = async (name: string) => {
    try {
      await api.RemoveMCPServer(name);
      await refresh();
      store.toast('success', t('mcp.removed', { name }));
    } catch (err) {
      store.toast('error', errorText(err));
    }
  };

  return (
    <>
      <PageHead title={t('settings.mcp')} description={t('mcp.description')} />
      <Card
        title={`${snap.mcp.length} server${snap.mcp.length === 1 ? '' : 's'}`}
        actions={
          <div style={{ display: 'flex', gap: 6 }}>
            <button className="btn small" onClick={() => setAddOpen(true)}>
              {t('mcp.add')}
            </button>
            <button className="btn small" onClick={() => void api.RevealPath(system.configPath)}>
              {t('mcp.openConfig')}
            </button>
          </div>
        }
      >
        {snap.mcp.length ? (
          <div className="settings-list">
            {snap.mcp.map((server) => (
              <div className="settings-list-row" key={server.name}>
                <div className="row-main">
                  <div className="row-name">{server.name}</div>
                  <div className="row-desc">
                    {server.type === 'http' ? server.url : [server.command, ...(server.args ?? [])].filter(Boolean).join(' ')}
                  </div>
                </div>
                <div className="row-side">
                  <span className="badge">{server.type}</span>
                  {server.env ? <span className="badge">{server.env} env</span> : null}
                  <button className="btn small" onClick={() => void remove(server.name)}>
                    {t('common.remove')}
                  </button>
                </div>
              </div>
            ))}
          </div>
        ) : (
          <Empty>{t('mcp.empty')}</Empty>
        )}
      </Card>
      {addOpen ? (
        <MCPModal
          onClose={() => setAddOpen(false)}
          onAdded={async () => {
            await refresh();
          }}
        />
      ) : null}
    </>
  );
}

export function SkillsPanel({ snap }: PanelProps) {
  const t = useT();
  return (
    <>
      <PageHead
        title={t('settings.skills')}
        description="Reusable instructions Pegg can load for specialized tasks. Global skills live in ~/.pegg/skills; project skills live in .pegg/skills."
      />
      <Card
        title={`${snap.skills.length} skill${snap.skills.length === 1 ? '' : 's'}`}
        actions={
          <button className="btn small" onClick={() => void api.RevealPath(snap.system.skillsDir)}>
            Open project skills
          </button>
        }
      >
        {snap.skills.length ? (
          <div className="settings-list">
            {snap.skills.map((skill) => (
              <button
                className="settings-list-row"
                key={`${skill.source}-${skill.name}`}
                onClick={() => skill.path && void api.RevealPath(skill.path)}
              >
                <div className="row-main">
                  <div className="row-name">{skill.name}</div>
                  <div className="row-desc">{skill.description || skill.path}</div>
                </div>
                <div className="row-side">
                  <span className="badge">{skill.source}</span>
                </div>
              </button>
            ))}
          </div>
        ) : (
          <Empty>No skills found. Add a SKILL.md folder under ~/.pegg/skills or .pegg/skills.</Empty>
        )}
      </Card>
    </>
  );
}

export function RulesPanel({ snap }: PanelProps) {
  const t = useT();
  const [selected, setSelected] = useState<number | null>(snap.rules.length ? 0 : null);
  useEffect(() => setSelected(snap.rules.length ? 0 : null), [snap.rules.length]);
  const rule = selected != null ? snap.rules[selected] : undefined;

  return (
    <>
      <PageHead
        title={t('settings.rules')}
        description="Project and global rules are injected into every conversation. They are plain Markdown files."
      />
      <Card
        title={`${snap.rules.length} rule${snap.rules.length === 1 ? '' : 's'}`}
        actions={
          <button className="btn small" onClick={() => void api.RevealPath(snap.system.rulesDir)}>
            Open project rules
          </button>
        }
      >
        {snap.rules.length ? (
          <>
            <div className="settings-list">
              {snap.rules.map((ruleItem, index) => (
                <button
                  key={ruleItem.path}
                  className={`settings-list-row${selected === index ? ' selected' : ''}`}
                  onClick={() => setSelected(index)}
                >
                  <div className="row-main">
                    <div className="row-name">{ruleItem.name}</div>
                    <div className="row-desc">{ruleItem.path}</div>
                  </div>
                  <div className="row-side">
                    <span className="badge">{ruleItem.source}</span>
                  </div>
                </button>
              ))}
            </div>
            {rule ? <pre className="preview-box">{rule.preview || 'Empty file.'}</pre> : null}
          </>
        ) : (
          <Empty>No rules found. Create a Markdown file under .pegg/rules or ~/.pegg/rules.</Empty>
        )}
      </Card>
    </>
  );
}

export function PluginsPanel({ snap, refresh }: PanelProps) {
  const t = useT();
  const [busy, setBusy] = useState<string | null>(null);

  const toggle = async (name: string) => {
    setBusy(name);
    try {
      await api.TogglePluginSettings(name);
      await refresh();
      store.toast('success', `Plugin ${name} updated`);
    } catch (err) {
      store.toast('error', errorText(err));
    } finally {
      setBusy(null);
    }
  };

  return (
    <>
      <PageHead
        title={t('settings.plugins')}
        description="Executable plugins extend Pegg with tools and integrations. Disabled plugins are added to the exclusion list."
      />
      <Card
        title={`${snap.plugins.length} plugin${snap.plugins.length === 1 ? '' : 's'}`}
        actions={
          <button className="btn small" onClick={() => void api.RevealPath(snap.system.pluginsDir)}>
            Open plugins folder
          </button>
        }
      >
        {!snap.pluginsEnabled ? (
          <div className="notice warn" style={{ marginBottom: 10 }}>
            All plugins are globally disabled in pegg.json.
          </div>
        ) : null}
        {snap.plugins.length ? (
          <div className="settings-list">
            {snap.plugins.map((plugin) => (
              <div className="settings-list-row" key={plugin.name}>
                <div className="row-main">
                  <div className="row-name">{plugin.name}</div>
                  <div className="row-desc">{plugin.path}</div>
                </div>
                <div className="row-side">
                  <span className="badge">{plugin.status}</span>
                  <Switch
                    checked={plugin.enabled}
                    onChange={() => void toggle(plugin.name)}
                    label={`Toggle ${plugin.name}`}
                  />
                  {busy === plugin.name ? <span className="spinner" /> : null}
                </div>
              </div>
            ))}
          </div>
        ) : (
          <Empty>No plugins installed in ~/.pegg/plugins.</Empty>
        )}
      </Card>
    </>
  );
}
