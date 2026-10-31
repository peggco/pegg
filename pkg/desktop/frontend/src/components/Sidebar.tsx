import { useMemo, useRef, useState } from 'react';
import {
  Command,
  FolderOpen,
  Moon,
  Pencil,
  Search,
  Settings2,
  SquarePen,
  Sun,
  Trash2,
  X,
} from 'lucide-react';
import { baseName, dayGroup, relativeTime, shortPath } from '../lib/format';
import { useT } from '../lib/i18n';
import {
  chooseWorkspace,
  deleteSession,
  focusComposer,
  openSession,
  renameSession,
  setTheme,
  startNewChat,
  togglePalette,
  toggleSettings,
} from '../state/actions';
import { store, useStore } from '../state/store';
import type { SessionListItem } from '../types';
import darkLogo from '../../../assets/dark-logo.png';
import lightLogo from '../../../assets/light-logo.png';

export function Sidebar() {
  const collapsed = useStore((s) => s.sidebarCollapsed);
  const sessions = useStore((s) => s.sessions);
  const loading = useStore((s) => s.sessionsLoading);
  const current = useStore((s) => s.chat.peggId);
  const workspace = useStore((s) => s.workspace);
  const view = useStore((s) => s.view);
  const theme = useStore((s) => s.theme);
  const t = useT();
  const logo = theme && !theme.dark ? lightLogo : darkLogo;
  const [renaming, setRenaming] = useState<string | null>(null);
  const [confirming, setConfirming] = useState<string | null>(null);
  const [draft, setDraft] = useState('');
  const recentsRef = useRef<HTMLDivElement>(null);

  const dayLabel = (label: string) => {
    switch (label) {
      case 'Today':
        return t('date.today');
      case 'Yesterday':
        return t('date.yesterday');
      case 'Previous 7 days':
        return t('date.previous7');
      case 'Earlier':
        return t('date.earlier');
      default:
        return label;
    }
  };

  const grouped = useMemo(() => {
    const groups = new Map<string, SessionListItem[]>();
    for (const item of sessions) {
      const key = dayGroup(item.updatedAt);
      const list = groups.get(key) ?? [];
      list.push(item);
      groups.set(key, list);
    }
    return Array.from(groups.entries());
  }, [sessions]);

  const onRename = (item: SessionListItem) => {
    setRenaming(item.id);
    setDraft(item.title);
    setConfirming(null);
  };

  const commitRename = (item: SessionListItem) => {
    const title = draft.trim();
    setRenaming(null);
    if (title && title !== item.title) void renameSession(item, title);
  };

  return (
    <aside className={`sidebar${collapsed ? ' collapsed' : ''}`} aria-label="Sidebar">
      <div className="sidebar-inner">
        <div className="brand">
          <span className="brand-name">
            <img className="brand-logo" src={logo} alt="Pegg" />
          </span>
          <button className="icon-btn" title={t('nav.newChat')} aria-label={t('nav.newChat')} onClick={startNewChat}>
            <SquarePen size={15} />
          </button>
        </div>

        <nav className="sidebar-nav">
          <button className="nav-item" onClick={startNewChat}>
            <SquarePen size={15} />
            {t('nav.newChat')}
            <span className="shortcut">⌘N</span>
          </button>
          <button
            className={`nav-item${view === 'chat' ? ' active' : ''}`}
            onClick={() => {
              recentsRef.current?.scrollTo({ top: 0, behavior: 'smooth' });
              focusComposer();
            }}
          >
            <Search size={15} />
            {t('nav.chats')}
            <span className="shortcut">⌘K</span>
          </button>
          <button className="nav-item" onClick={() => void chooseWorkspace()}>
            <FolderOpen size={15} />
            {t('nav.projects')}
            <span className="shortcut" title={workspace}>
              {workspace ? baseName(workspace).slice(0, 12) : ''}
            </span>
          </button>
          <button className="nav-item" onClick={() => toggleSettings(true)}>
            <Settings2 size={15} />
            {t('nav.settings')}
          </button>
        </nav>

        <div className="sidebar-section">
          <span>{t('nav.recents')}</span>
          <span style={{ opacity: 0.6 }}>{sessions.length || ''}</span>
        </div>

        <div className="recents" ref={recentsRef}>
          {loading && !sessions.length ? (
            <div className="recents-empty">
              <span className="spinner" /> {t('nav.recents')}…
            </div>
          ) : null}
          {!loading && !sessions.length ? (
            <div className="recents-empty">
              {t('nav.noSessions')}
            </div>
          ) : null}
          {grouped.map(([label, items]) => (
            <div key={label}>
              <div className="recent-group-label">{dayLabel(label)}</div>
              {items.map((item) => (
                <div
                  key={item.id}
                  className={`session-row${current === item.id ? ' active' : ''}`}
                  role="button"
                  tabIndex={0}
                  onClick={() => void openSession(item)}
                  onKeyDown={(event) => {
                    if (event.key === 'Enter') void openSession(item);
                  }}
                >
                  {renaming === item.id ? (
                    <input
                      className="input"
                      style={{ height: 26, minWidth: 0, width: '100%', fontSize: 12 }}
                      autoFocus
                      value={draft}
                      onClick={(event) => event.stopPropagation()}
                      onChange={(event) => setDraft(event.target.value)}
                      onBlur={() => commitRename(item)}
                      onKeyDown={(event) => {
                        event.stopPropagation();
                        if (event.key === 'Enter') commitRename(item);
                        if (event.key === 'Escape') setRenaming(null);
                      }}
                    />
                  ) : (
                    <>
                      <span className="session-title">{item.title || 'Untitled session'}</span>
                      {item.projectDir && item.projectDir !== workspace ? (
                        <span className="project-tag">{baseName(item.projectDir)}</span>
                      ) : null}
                      <span className="session-time">{relativeTime(item.updatedAt)}</span>
                      <span className="row-actions" onClick={(event) => event.stopPropagation()}>
                        {confirming === item.id ? (
                          <>
                            <button
                              className="danger"
                              title="Confirm delete"
                              onClick={() => {
                                setConfirming(null);
                                void deleteSession(item);
                              }}
                            >
                              <Trash2 size={12} />
                            </button>
                            <button title="Cancel" onClick={() => setConfirming(null)}>
                              <X size={12} />
                            </button>
                          </>
                        ) : (
                          <>
                            <button title={t('nav.rename')} onClick={() => onRename(item)}>
                              <Pencil size={12} />
                            </button>
                            <button title={t('nav.delete')} onClick={() => setConfirming(item.id)}>
                              <Trash2 size={12} />
                            </button>
                          </>
                        )}
                      </span>
                    </>
                  )}
                </div>
              ))}
            </div>
          ))}
        </div>

        <div className="sidebar-footer">
          <button className="nav-item" onClick={() => togglePalette(true)}>
            <Command size={15} />
            {t('nav.commandPalette')}
            <span className="shortcut">⌘K</span>
          </button>
          <button
            className="nav-item"
            onClick={() => {
              const currentTheme = store.get().theme;
              if (!currentTheme) return;
              void setTheme(currentTheme.name, currentTheme.dark ? 'light' : 'dark');
            }}
          >
            {theme?.dark ? <Sun size={15} /> : <Moon size={15} />}
            {theme?.dark ? t('nav.lightAppearance') : t('nav.darkAppearance')}
          </button>
          <div className="account">
            <span className="avatar">P</span>
            <div className="account-text">
              <div className="account-name">{t('nav.localWorkspace')}</div>
              <div className="account-sub">{workspace ? shortPath(workspace, 28) : t('nav.noProject')}</div>
            </div>
            <button className="icon-btn" title="Settings" onClick={() => toggleSettings(true)}>
              <Settings2 size={14} />
            </button>
          </div>
        </div>
      </div>
    </aside>
  );
}
