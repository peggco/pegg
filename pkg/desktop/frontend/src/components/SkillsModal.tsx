import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { ArrowLeft, ArrowRight, Check, Copy, Download, ExternalLink, Search, X } from 'lucide-react';
import { api } from '../lib/bridge';
import { renderMarkdown } from '../lib/markdown';
import { useT } from '../lib/i18n';
import {
  errorText,
  fetchSkillReadme,
  installHubSkill,
  searchHubSkillsFiltered,
  toggleSkills,
} from '../state/actions';
import { store, useStore } from '../state/store';
import type { HubSkill } from '../types';

type Filter = 'all' | 'official' | 'community';
type Scope = 'global' | 'project';

const PAGE_SIZE = 100;

function initials(name: string): string {
  const words = name.replace(/[^a-zA-Z0-9]+/g, ' ').trim().split(/\s+/).filter(Boolean);
  if (words.length > 1) return (words[0][0] + words[1][0]).toUpperCase();
  return name.replace(/[^a-zA-Z0-9]/g, '').slice(0, 2).toUpperCase() || '?';
}

function skillKey(skill: HubSkill): string {
  return `${skill.source}/${skill.skillId}`;
}

export function SkillsModal() {
  const open = useStore((s) => s.skillsOpen);
  const t = useT();
  const [filter, setFilter] = useState<Filter>('all');
  const [query, setQuery] = useState('');
  const [items, setItems] = useState<HubSkill[]>([]);
  const [nextCursor, setNextCursor] = useState<number | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [selected, setSelected] = useState<HubSkill | null>(null);
  const [readme, setReadme] = useState('');
  const [readmeLoading, setReadmeLoading] = useState(false);
  const [scope, setScope] = useState<Scope>('global');
  const [busy, setBusy] = useState<string | null>(null);
  const [installed, setInstalled] = useState<Set<string>>(new Set());
  const [copied, setCopied] = useState(false);
  const requestID = useRef(0);

  const load = useCallback(async (value: string, mode: Filter, cursor = 0, append = false) => {
    const id = ++requestID.current;
    setLoading(true);
    try {
      const res = await searchHubSkillsFiltered(value.trim(), PAGE_SIZE, mode, cursor);
      if (id !== requestID.current) return;
      setItems((prev) => (append ? [...prev, ...res.items] : res.items));
      setNextCursor(res.nextCursor ?? null);
      setError('');
    } catch (err) {
      if (id === requestID.current) setError(errorText(err));
    } finally {
      if (id === requestID.current) setLoading(false);
    }
  }, []);

  useEffect(() => {
    if (!open) return;
    const timer = window.setTimeout(() => void load(query, filter), query.trim() ? 300 : 0);
    return () => window.clearTimeout(timer);
  }, [open, query, filter, load]);

  useEffect(() => {
    if (!selected) {
      setReadme('');
      return;
    }
    let alive = true;
    setReadme('');
    setReadmeLoading(true);
    fetchSkillReadme(selected.source)
      .then((text) => {
        if (alive) setReadme(text);
      })
      .catch(() => {
        if (alive) setReadme('');
      })
      .finally(() => {
        if (alive) setReadmeLoading(false);
      });
    return () => {
      alive = false;
    };
  }, [selected]);

  useEffect(() => {
    if (!copied) return;
    const timer = window.setTimeout(() => setCopied(false), 1400);
    return () => window.clearTimeout(timer);
  }, [copied]);

  const install = async (skill: HubSkill, target: Scope) => {
    const key = skillKey(skill);
    if (installed.has(key)) return;
    setBusy(key);
    try {
      await installHubSkill(skill.source, skill.skillId, target);
      setInstalled((prev) => new Set(prev).add(key));
    } catch (err) {
      store.toast('error', errorText(err));
    } finally {
      setBusy(null);
    }
  };

  const readmeHtml = useMemo(() => {
    if (!readme) return '';
    try {
      return renderMarkdown(readme);
    } catch {
      return '';
    }
  }, [readme]);

  const command = selected ? `pegg skills add ${selected.source}/${selected.skillId}` : '';
  const repoURL = selected
    ? selected.url ||
      (/^[A-Za-z0-9._-]+\/[A-Za-z0-9._-]+$/.test(selected.source)
        ? `https://github.com/${selected.source}`
        : '')
    : '';

  const copy = async () => {
    if (!command) return;
    try {
      await navigator.clipboard.writeText(command);
      setCopied(true);
    } catch {
      store.toast('warn', 'Clipboard is unavailable');
    }
  };

  const close = () => {
    toggleSkills(false);
    setSelected(null);
  };

  return (
    <div
      className={`overlay${open ? '' : ' hidden'}`}
      aria-hidden={!open}
      onMouseDown={(event) => event.target === event.currentTarget && open && close()}
    >
      <div className="modal skills-modal" role="dialog" aria-modal="true" aria-label={t('skills.title')}>
        <div className="modal-head">
          <span className="modal-title">{t('skills.title')}</span>
          <span className="badge">Pegg Hub</span>
          <div className="spacer" />
          <button className="icon-btn" aria-label="Close skills" onClick={close}>
            <X size={16} />
          </button>
        </div>
        <div className="modal-body">
          {selected ? (
            <div className="hub-pane">
              <button className="hub-back" onClick={() => setSelected(null)}>
                <ArrowLeft size={14} />
                {t('skills.back')}
              </button>

              <div className="hub-detail-head">
                <span className="hub-avatar large">{initials(selected.name)}</span>
                <div>
                  <span className={`hub-status${selected.official ? ' official' : ''}`}>
                    <span className="hub-dot" />
                    {selected.official ? t('skills.official') : t('skills.community')}
                  </span>
                  <h1 className="hub-detail-title">{selected.name}</h1>
                  <div className="hub-card-source">{selected.source}</div>
                </div>
              </div>

              <div className="hub-install-bar">
                <span className="hub-install-label">{t('skills.installCommand')}</span>
                <code>{command}</code>
                <button className="btn small" onClick={() => void copy()}>
                  {copied ? <Check size={12} /> : <Copy size={12} />}
                  {copied ? t('skills.copied') : t('skills.copy')}
                </button>
                <div className="hub-scope">
                  {(['global', 'project'] as Scope[]).map((item) => (
                    <button key={item} className={scope === item ? 'active' : ''} onClick={() => setScope(item)}>
                      {t(`skills.${item}`)}
                    </button>
                  ))}
                </div>
                <button
                  className="btn primary small"
                  disabled={busy === skillKey(selected) || installed.has(skillKey(selected))}
                  onClick={() => void install(selected, scope)}
                >
                  {installed.has(skillKey(selected)) ? <Check size={12} /> : <Download size={12} />}
                  {installed.has(skillKey(selected))
                    ? t('skills.installed')
                    : busy === skillKey(selected)
                      ? t('skills.installing')
                      : t('skills.install')}
                </button>
              </div>

              <div className="hub-detail-grid">
                <div className="hub-detail-card">
                  <div className="hub-detail-heading">
                    <span>{t('skills.readme')}</span>
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
                    <div className="hub-empty">{t('skills.noReadme')}</div>
                  )}
                </div>

                <div className="hub-detail-side">
                  <div className="hub-detail-card">
                  <div className="hub-detail-heading">
                    <span>{t('skills.details')}</span>
                  </div>
                  <dl className="hub-detail-list">
                    <div>
                      <dt>{t('skills.source')}</dt>
                      <dd>{selected.source}</dd>
                    </div>
                    <div>
                      <dt>{t('skills.status')}</dt>
                      <dd>{selected.official ? t('skills.official') : t('skills.community')}</dd>
                    </div>
                    <div>
                      <dt>{t('skills.license')}</dt>
                      <dd>{selected.license || '—'}</dd>
                    </div>
                    <div>
                      <dt>{t('skills.skillId')}</dt>
                      <dd>{selected.skillId}</dd>
                    </div>
                  </dl>
                  </div>
                  <button
                    className="btn primary hub-side-install"
                    disabled={busy === skillKey(selected) || installed.has(skillKey(selected))}
                    onClick={() => void install(selected, scope)}
                  >
                    {installed.has(skillKey(selected)) ? <Check size={13} /> : <Download size={13} />}
                    {installed.has(skillKey(selected))
                      ? t('skills.installed')
                      : busy === skillKey(selected)
                        ? t('skills.installing')
                        : t('skills.install')}
                  </button>
                </div>
              </div>
            </div>
          ) : (
            <div className="hub-pane">
              <div className="hub-hero">
                <div className="hub-eyebrow">{t('skills.directory')}</div>
                <h1 className="hub-title">{t('skills.title')}</h1>
                <p className="hub-sub">{t('skills.subtitle')}</p>
              </div>

              <div className="hub-toolbar">
                <div className="hub-search">
                  <Search size={15} />
                  <input
                    value={query}
                    placeholder={t('skills.search')}
                    onChange={(event) => setQuery(event.target.value)}
                  />
                </div>
                <div className="hub-filter">
                  {(['all', 'official', 'community'] as Filter[]).map((item) => (
                    <button key={item} className={filter === item ? 'active' : ''} onClick={() => setFilter(item)}>
                      {t(`skills.${item}`)}
                    </button>
                  ))}
                </div>
              </div>

              <div className="hub-meta">
                <span>
                  {loading && !items.length
                    ? t('common.loading')
                    : `${items.length}${nextCursor ? '+' : ''} ${t('skills.count')}`}
                </span>
              </div>

              {error ? <div className="notice error">{error}</div> : null}

              {!loading && !items.length && !error ? <div className="hub-empty">{t('skills.noResults')}</div> : null}

              <div className="hub-grid">
                {items.map((skill) => {
                  const key = skillKey(skill);
                  const isInstalled = installed.has(key);
                  return (
                    <div
                      key={key}
                      className="hub-card"
                      role="button"
                      tabIndex={0}
                      onClick={() => setSelected(skill)}
                      onKeyDown={(event) => {
                        if (event.key === 'Enter' || event.key === ' ') {
                          event.preventDefault();
                          setSelected(skill);
                        }
                      }}
                    >
                      <div className="hub-card-top">
                        <span className="hub-avatar">{initials(skill.name)}</span>
                        <span className={`hub-status${skill.official ? ' official' : ''}`}>
                          <span className="hub-dot" />
                          {skill.official ? t('skills.official') : t('skills.community')}
                        </span>
                        <button
                          className={`hub-install${isInstalled ? ' done' : ''}`}
                          disabled={busy === key || isInstalled}
                          onClick={(event) => {
                            event.stopPropagation();
                            void install(skill, scope);
                          }}
                        >
                          {isInstalled ? <Check size={12} /> : <Download size={12} />}
                          {isInstalled
                            ? t('skills.installed')
                            : busy === key
                              ? t('skills.installing')
                              : t('skills.install')}
                        </button>
                      </div>
                      <div className="hub-card-name">{skill.name}</div>
                      <div className="hub-card-source">{skill.source}</div>
                      <div className="hub-card-foot">
                        {skill.license ? <span className="hub-license">{skill.license}</span> : <span />}
                        <ArrowRight size={14} className="hub-arrow" />
                      </div>
                    </div>
                  );
                })}
              </div>

              {nextCursor ? (
                <div className="hub-more">
                  <button
                    className="btn small"
                    disabled={loading}
                    onClick={() => void load(query, filter, nextCursor, true)}
                  >
                    {loading ? t('common.loading') : t('skills.loadMore')}
                  </button>
                </div>
              ) : null}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
