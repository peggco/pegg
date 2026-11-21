import { useEffect, useRef, useState } from 'react';
import { Check, FolderOpen, MoreHorizontal, Trash2 } from 'lucide-react';
import { baseName, shortPath } from '../lib/format';
import { useT } from '../lib/i18n';
import { chooseWorkspace, removeRecent, setWorkspace } from '../state/actions';
import { useStore } from '../state/store';

/**
 * Menu of the local projects the app knows about. The open project is marked,
 * and every row has a three-dot menu with **Remove Local Project**, which only
 * forgets the project here — nothing on disk is deleted.
 */
export function ProjectMenu({
  onClose,
  placement = 'down',
}: {
  onClose: () => void;
  placement?: 'down' | 'up';
}) {
  const recents = useStore((s) => s.recents);
  const workspace = useStore((s) => s.workspace);
  const t = useT();
  const [menuFor, setMenuFor] = useState<string | null>(null);
  const rootRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const onDown = (event: MouseEvent) => {
      if (!rootRef.current?.contains(event.target as Node)) onClose();
    };
    const onKey = (event: KeyboardEvent) => {
      if (event.key !== 'Escape') return;
      if (menuFor) {
        setMenuFor(null);
        return;
      }
      onClose();
    };
    window.addEventListener('mousedown', onDown);
    window.addEventListener('keydown', onKey);
    return () => {
      window.removeEventListener('mousedown', onDown);
      window.removeEventListener('keydown', onKey);
    };
  }, [menuFor, onClose]);

  return (
    <div
      className={`popover project-menu${placement === 'up' ? ' up' : ''}`}
      ref={rootRef}
      role="menu"
      aria-label={t('nav.projects')}
    >
      <div className="popover-label">{t('nav.projects')}</div>
      {recents?.length ? (
        recents.map((item) => {
          const active = item.path === workspace;
          return (
            <div className={`project-row${active ? ' active' : ''}`} key={item.path}>
              <button
                className="project-open"
                title={item.path}
                onClick={() => {
                  onClose();
                  if (!active) void setWorkspace(item.path);
                }}
              >
                <span className="project-name">{item.name || baseName(item.path)}</span>
                <span className="project-path">{shortPath(item.path, 34)}</span>
              </button>
              {active ? <Check size={13} className="check" /> : null}
              <button
                className="project-more"
                title={t('nav.projectActions')}
                aria-label={t('nav.projectActions')}
                aria-expanded={menuFor === item.path}
                onClick={() => setMenuFor(menuFor === item.path ? null : item.path)}
              >
                <MoreHorizontal size={14} />
              </button>
              {menuFor === item.path ? (
                <div className="popover project-row-menu" role="menu">
                  <button
                    className="popover-item danger"
                    onClick={() => {
                      setMenuFor(null);
                      onClose();
                      void removeRecent(item.path);
                    }}
                  >
                    <Trash2 size={13} />
                    {t('nav.removeProject')}
                  </button>
                </div>
              ) : null}
            </div>
          );
        })
      ) : (
        <div className="project-empty">{t('nav.noRecentProjects')}</div>
      )}
      <button
        className="popover-item"
        onClick={() => {
          onClose();
          void chooseWorkspace();
        }}
      >
        <FolderOpen size={13} />
        {t('nav.openProject')}
      </button>
    </div>
  );
}
