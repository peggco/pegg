import { useEffect } from 'react';
import { PanelLeft } from 'lucide-react';
import {
  onAsk,
  onBinaryProgress,
  onFileDrop,
  onLoading,
  onPendingClear,
  onPermission,
  onStatus,
  onUpdate,
} from './lib/bridge';
import {
  bootstrap,
  clearPending,
  handleAsk,
  handleLoading,
  handlePermission,
  handleUpdates,
  readDropped,
  respondPermission,
  setBinaryProgress,
  setServerStatus,
  startNewChat,
  togglePalette,
  toggleSettings,
  toggleSidebar,
} from './state/actions';
import { store, useStore } from './state/store';
import { CommandPalette } from './components/CommandPalette';
import { Composer } from './components/Composer';
import { Onboarding, StatusBanner, Toasts, Welcome } from './components/Welcome';
import { Settings } from './components/Settings';
import { Sidebar } from './components/Sidebar';
import { Thread } from './components/Thread';
import { TitleBar } from './components/TitleBar';

export function App() {
  const ready = useStore((s) => s.ready);
  const view = useStore((s) => s.view);
  const binary = useStore((s) => s.binary);
  const sidebarCollapsed = useStore((s) => s.sidebarCollapsed);
  const subagentView = useStore((s) => s.subagentView);
  const platform = useStore((s) => s.boot?.platform ?? 'linux');
  const nativeTitlebar = useStore((s) => s.boot?.nativeTitlebar ?? false);
  const showTitlebar = platform === 'darwin' || !nativeTitlebar;

  useEffect(() => {
    void bootstrap();
  }, []);

  useEffect(() => {
    const preventDefault = (event: DragEvent) => {
      event.preventDefault();
    };
    window.addEventListener('dragover', preventDefault);
    window.addEventListener('drop', preventDefault);
    return () => {
      window.removeEventListener('dragover', preventDefault);
      window.removeEventListener('drop', preventDefault);
    };
  }, []);

  useEffect(() => {
    const subscriptions = [
      onUpdate((sessionId, updates) => handleUpdates(sessionId, updates as never)),
      onPermission((request) => handlePermission(request.requestId, request.params)),
      onAsk((request) => handleAsk(request.requestId, request.params)),
      onStatus(setServerStatus),
      onLoading(handleLoading),
      onPendingClear(clearPending),
      onBinaryProgress(setBinaryProgress),
      onFileDrop((paths) => {
        void readDropped(paths).then((attachments) => {
          if (attachments.length) {
            window.dispatchEvent(new CustomEvent('pegg:attachments', { detail: attachments }));
            store.toast('info', `${attachments.length} file${attachments.length === 1 ? '' : 's'} attached`);
          }
        });
      }),
    ];
    return () => {
      for (const unsubscribe of subscriptions) unsubscribe();
    };
  }, []);

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      const target = event.target as HTMLElement | null;
      const typing =
        target?.tagName === 'INPUT' || target?.tagName === 'TEXTAREA' || target?.isContentEditable === true;
      const mod = event.metaKey || event.ctrlKey;

      if (mod && event.key.toLowerCase() === 'k') {
        event.preventDefault();
        togglePalette(true);
        return;
      }
      if (mod && event.key.toLowerCase() === 'n') {
        event.preventDefault();
        startNewChat();
        return;
      }
      if (mod && event.key === ',') {
        event.preventDefault();
        toggleSettings(true);
        return;
      }
      if (event.key === 'Escape') {
        const state = store.get();
        if (state.paletteOpen) togglePalette(false);
        else if (state.settingsOpen) toggleSettings(false);
        else if (state.modelsOpen) store.set({ modelsOpen: false });
        return;
      }
      if (!typing && /^[1-9]$/.test(event.key)) {
        const state = store.get();
        const permission = state.chat.permissions[0];
        if (!permission) return;
        const options =
          permission.params.options?.length
            ? permission.params.options
            : [
                { optionId: 'allow_once', name: 'Allow', kind: 'allow_once' },
                { optionId: 'allow_always', name: 'Always allow', kind: 'allow_always' },
                { optionId: 'reject_once', name: 'Reject', kind: 'reject_once' },
              ];
        const option = options[Number(event.key) - 1];
        if (option) {
          event.preventDefault();
          void respondPermission(permission.requestId, option.optionId);
        }
      }
    };
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, []);

  if (!ready) {
    return (
      <div className="app">
        {showTitlebar ? <TitleBar /> : null}
        <div className="thread-loading" style={{ flex: 1 }}>
          <span className="spinner" /> Starting Pegg…
        </div>
      </div>
    );
  }

  if (!binary.found) {
    return (
      <div className="app">
        {showTitlebar ? <TitleBar /> : null}
        <div className="main-scroll">
          <Onboarding />
        </div>
        <Toasts />
      </div>
    );
  }

  return (
    <div className="app">
      {showTitlebar ? <TitleBar /> : null}
      <StatusBanner />
      <div className="shell">
        <Sidebar />
        <main className="main">
          {sidebarCollapsed ? (
            <button className="floating-sidebar-toggle" title="Show sidebar" onClick={toggleSidebar}>
              <PanelLeft size={15} />
            </button>
          ) : null}
          {view === 'welcome' ? (
            <div className="main-scroll">
              <Welcome />
            </div>
          ) : (
            <>
              <Thread />
              <div className={`composer-dock${subagentView ? ' hidden' : ''}`}>
                <Composer compact />
                <div className="composer-hint">
                  Type @ for files · / for commands · Pegg can run commands and edit files.
                </div>
              </div>
            </>
          )}
        </main>
      </div>
      <Settings />
      <CommandPalette />
      <Toasts />
    </div>
  );
}
