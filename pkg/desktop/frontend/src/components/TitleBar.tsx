import { ChevronRight, Command, Minus, PanelLeft, Settings2, Square, X } from 'lucide-react';
import { api } from '../lib/bridge';
import { baseName } from '../lib/format';
import { chooseWorkspace, togglePalette, toggleSettings, toggleSidebar } from '../state/actions';
import { useStore } from '../state/store';

export function TitleBar() {
  const platform = useStore((s) => s.boot?.platform ?? 'linux');
  const native = useStore((s) => s.boot?.nativeTitlebar ?? false);
  const workspace = useStore((s) => s.workspace);
  const title = useStore((s) => s.chat.title);
  const view = useStore((s) => s.view);
  const sidebarCollapsed = useStore((s) => s.sidebarCollapsed);

  const showControls = platform !== 'darwin' && !native;
  const isMac = platform === 'darwin';

  return (
    <header
      className={`titlebar${isMac ? ' mac' : ''}`}
      onDoubleClick={(event) => {
        if ((event.target as HTMLElement).closest('button,a,input')) return;
        if (showControls) void api.WindowToggleMaximise();
      }}
    >
      <button
        className="icon-btn no-drag"
        title={sidebarCollapsed ? 'Show sidebar' : 'Hide sidebar'}
        aria-label="Toggle sidebar"
        onClick={toggleSidebar}
      >
        <PanelLeft size={16} />
      </button>
      <button
        className="titlebar-project no-drag"
        onClick={() => void chooseWorkspace()}
        title={workspace}
      >
        <span className="dot" />
        <span className="label">{workspace ? baseName(workspace) : 'Choose a project'}</span>
        {view === 'chat' && title ? (
          <>
            <ChevronRight size={12} style={{ opacity: 0.5 }} />
            <span className="label" style={{ color: 'var(--pg-muted)', maxWidth: 260 }}>
              {title}
            </span>
          </>
        ) : null}
      </button>
      <div className="titlebar-spacer" />
      <div className="titlebar-actions">
        <button
          className="icon-btn no-drag"
          title="Command palette (Ctrl/Cmd+K)"
          onClick={() => togglePalette(true)}
        >
          <Command size={16} />
        </button>
        <button className="icon-btn no-drag" title="Settings" onClick={() => toggleSettings(true)}>
          <Settings2 size={16} />
        </button>
      </div>
      {showControls ? <WindowControls /> : null}
    </header>
  );
}

function WindowControls() {
  return (
    <div className="window-controls no-drag">
      <button title="Minimize" aria-label="Minimize" onClick={() => void api.WindowMinimise()}>
        <Minus size={14} />
      </button>
      <button title="Maximize" aria-label="Maximize" onClick={() => void api.WindowToggleMaximise()}>
        <Square size={11} />
      </button>
      <button className="close" title="Close" aria-label="Close" onClick={() => void api.WindowClose()}>
        <X size={15} />
      </button>
    </div>
  );
}
