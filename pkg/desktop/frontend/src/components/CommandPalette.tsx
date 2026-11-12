import { useEffect, useMemo, useRef, useState } from 'react';
import {
  Download,
  FolderOpen,
  MessageSquare,
  RefreshCw,
  Search,
  Settings2,
  SquarePen,
  Terminal,
} from 'lucide-react';
import { api } from '../lib/bridge';
import { relativeTime } from '../lib/format';
import {
  bootstrap,
  findBinary,
  installBinary,
  loadSessions,
  openSession,
  startNewChat,
  togglePalette,
  toggleSettings,
  toggleSidebar,
} from '../state/actions';
import { store, useStore } from '../state/store';

interface Item {
  id: string;
  label: string;
  hint?: string;
  keywords?: string;
  icon: typeof Search;
  run: () => void | Promise<void>;
}

export function CommandPalette() {
  const open = useStore((s) => s.paletteOpen);
  const sessions = useStore((s) => s.sessions);
  const [query, setQuery] = useState('');
  const [index, setIndex] = useState(0);
  const inputRef = useRef<HTMLInputElement>(null);
  const listRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (open) {
      setQuery('');
      setIndex(0);
      window.setTimeout(() => inputRef.current?.focus(), 20);
    }
  }, [open]);

  const actions = useMemo<Item[]>(() => {
    const items: Item[] = [
      {
        id: 'new',
        label: 'New chat',
        hint: '⌘N',
        keywords: 'new session start',
        icon: SquarePen,
        run: startNewChat,
      },
      {
        id: 'settings',
        label: 'Open settings',
        hint: '⌘,',
        keywords: 'preferences configuration',
        icon: Settings2,
        run: () => toggleSettings(true),
      },
      {
        id: 'project',
        label: 'Open project folder…',
        keywords: 'workspace directory',
        icon: FolderOpen,
        run: () => void import('../state/actions').then((m) => m.chooseWorkspace()),
      },
      {
        id: 'sidebar',
        label: 'Toggle sidebar',
        keywords: 'collapse expand',
        icon: Search,
        run: toggleSidebar,
      },
      {
        id: 'reconnect',
        label: 'Reconnect to Pegg server',
        keywords: 'restart acp',
        icon: RefreshCw,
        run: () => void bootstrap(),
      },
      {
        id: 'find-binary',
        label: 'Find Pegg binary',
        keywords: 'path executable',
        icon: Terminal,
        run: () => void findBinary(),
      },
      {
        id: 'install-binary',
        label: 'Install or update Pegg',
        keywords: 'download update binary',
        icon: Download,
        run: () => void installBinary(),
      },
      {
        id: 'log',
        label: 'Reveal log file',
        keywords: 'debug log',
        icon: Terminal,
        run: () => {
          const path = store.get().boot?.logPath;
          if (path) void api.RevealPath(path);
        },
      },
      {
        id: 'refresh-sessions',
        label: 'Refresh sessions',
        keywords: 'reload list',
        icon: RefreshCw,
        run: () => void loadSessions(),
      },
    ];
    return items;
  }, []);

  const all = useMemo<Item[]>(() => {
    const items = [...actions];
    for (const session of sessions.slice(0, 30)) {
      items.push({
        id: `session-${session.id}`,
        label: session.title || 'Untitled session',
        hint: relativeTime(session.updatedAt),
        keywords: `${session.projectDir ?? ''} session`,
        icon: MessageSquare,
        run: () => void openSession(session),
      });
    }
    return items;
  }, [actions, sessions]);

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return all.slice(0, 40);
    return all
      .filter((item) => `${item.label} ${item.keywords ?? ''}`.toLowerCase().includes(q))
      .slice(0, 40);
  }, [all, query]);

  useEffect(() => {
    setIndex(0);
  }, [query]);

  useEffect(() => {
    const el = listRef.current?.querySelector<HTMLElement>(`[data-index="${index}"]`);
    el?.scrollIntoView({ block: 'nearest' });
  }, [index]);

  const run = (item: Item | undefined) => {
    if (!item) return;
    togglePalette(false);
    void item.run();
  };

  return (
    <div
      className={`overlay${open ? '' : ' hidden'}`}
      aria-hidden={!open}
      onMouseDown={(event) => event.target === event.currentTarget && open && togglePalette(false)}
    >
      <div className="palette" role="dialog" aria-modal="true" aria-label="Command palette">
        <input
          ref={inputRef}
          className="palette-input"
          placeholder="Search commands, sessions…"
          value={query}
          onChange={(event) => setQuery(event.target.value)}
          onKeyDown={(event) => {
            if (event.key === 'Escape') togglePalette(false);
            if (event.key === 'ArrowDown') {
              event.preventDefault();
              setIndex((i) => Math.min(filtered.length - 1, i + 1));
            }
            if (event.key === 'ArrowUp') {
              event.preventDefault();
              setIndex((i) => Math.max(0, i - 1));
            }
            if (event.key === 'Enter') {
              event.preventDefault();
              run(filtered[index]);
            }
          }}
        />
        <div className="palette-list" ref={listRef}>
          {filtered.length === 0 ? (
            <div style={{ padding: '14px 12px', color: 'var(--pg-muted)', fontSize: 12.5 }}>
              No matches.
            </div>
          ) : null}
          {filtered.map((item, i) => (
            <button
              key={item.id}
              data-index={i}
              className={`palette-item${i === index ? ' active' : ''}`}
              onMouseEnter={() => setIndex(i)}
              onClick={() => run(item)}
            >
              <item.icon size={14} />
              <span>{item.label}</span>
              {item.hint ? <span className="hint">{item.hint}</span> : null}
            </button>
          ))}
        </div>
      </div>
    </div>
  );
}
