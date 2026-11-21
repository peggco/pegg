import { useEffect, useRef, useState, type RefObject } from 'react';
import { Check, FolderOpen, MoreHorizontal, Trash2, X } from 'lucide-react';
import { baseName, shortPath } from '../lib/format';
import { useT } from '../lib/i18n';
import { chooseWorkspace, removeRecent, setWorkspace } from '../state/actions';
import { useStore } from '../state/store';

export function ProjectMenu({
  onClose,
  placement = 'down',
  anchorRef,
}: {
  onClose: () => void;
  placement?: 'down' | 'up';
  anchorRef?: RefObject<HTMLElement | null>;
}) {
  const recents = useStore((s) => s.recents);
  const workspace = useStore((s) => s.workspace);
  const t = useT();
  const [menuFor, setMenuFor] = useState<string | null>(null);
  const [confirming, setConfirming] = useState<string | null>(null);
  const rootRef = useRef<HTMLDivElement>(null);
  const closeRef = useRef(onClose);
  closeRef.current = onClose;

  useEffect(() => {
    const onDown = (event: MouseEvent) => {
      const target = event.target as Node;
      if (rootRef.current?.contains(target) || anchorRef?.current?.contains(target)) return;
      closeRef.current();
    };
    const onKey = (event: KeyboardEvent) => {
      if (event.key !== 'Escape') return;
      if (confirming) {
        setConfirming(null);
        return;
      }
      if (menuFor) {
        setMenuFor(null);
        return;
      }
      closeRef.current();
    };
    window.addEventListener('mousedown', onDown);
    window.addEventListener('keydown', onKey);
    return () => {
      window.removeEventListener('mousedown', onDown);
      window.removeEventListener('keydown', onKey);
    };
  }, [menuFor, confirming, anchorRef]);

  useEffect(() => {
    if (!menuFor) return;
    const panel = rootRef.current?.querySelector<HTMLElement>('.project-row-menu');
    panel?.scrollIntoView?.({ block: 'nearest' });
    panel?.querySelector<HTMLElement>('button')?.focus();
  }, [menuFor, confirming]);

  const rows = recents ?? [];

  return (
    <div
      className={`popover project-menu${placement === 'up' ? ' up' : ''}`}
      ref={rootRef}
      role="menu"
      aria-label={t('nav.projects')}
    >
      <div className="popover-label">{t('nav.projects')}</div>
      {rows.length ? (
        rows.map((item) => {
          const active = item.path === workspace;
          const name = item.name || baseName(item.path);
          const actionsLabel = `${t('nav.projectActions')}: ${name}`;
          return (
            <div className={`project-row${active ? ' active' : ''}`} key={item.path}>
              <button
                className="project-open"
                role="menuitemradio"
                aria-checked={active}
                title={item.path}
                onClick={() => {
                  onClose();
                  if (!active) void setWorkspace(item.path);
                }}
              >
                <span className="project-name">{name}</span>
                <span className="project-path">{shortPath(item.path, 34)}</span>
              </button>
              {active ? <Check size={13} className="check" aria-hidden="true" /> : null}
              <button
                className="project-more"
                title={actionsLabel}
                aria-label={actionsLabel}
                aria-haspopup="menu"
                aria-expanded={menuFor === item.path}
                onClick={() => {
                  setMenuFor(menuFor === item.path ? null : item.path);
                  setConfirming(null);
                }}
              >
                <MoreHorizontal size={14} />
              </button>
              {menuFor === item.path ? (
                <div className="popover project-row-menu" role="menu" aria-label={actionsLabel}>
                  {confirming === item.path ? (
                    <>
                      <div className="project-confirm">
                        <span className="project-confirm-title">
                          {t('nav.removeProjectConfirm')}
                        </span>
                        <span className="project-confirm-detail">
                          {t('nav.removeProjectDetail')}
                        </span>
                      </div>
                      <button
                        className="popover-item danger"
                        role="menuitem"
                        onClick={() => {
                          setConfirming(null);
                          setMenuFor(null);
                          onClose();
                          void removeRecent(item.path);
                        }}
                      >
                        <Trash2 size={13} />
                        {t('common.remove')}
                      </button>
                      <button
                        className="popover-item"
                        role="menuitem"
                        onClick={() => setConfirming(null)}
                      >
                        <X size={13} />
                        {t('common.cancel')}
                      </button>
                    </>
                  ) : (
                    <button
                      className="popover-item danger"
                      role="menuitem"
                      onClick={() => setConfirming(item.path)}
                    >
                      <Trash2 size={13} />
                      {t('nav.removeProject')}
                    </button>
                  )}
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
        role="menuitem"
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
