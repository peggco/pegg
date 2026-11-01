import { Bug, FolderOpen, GitPullRequest, ListChecks, Sparkles, Terminal, TriangleAlert, X } from 'lucide-react';
import { baseName } from '../lib/format';
import { greetingKey, useT } from '../lib/i18n';
import {
  bootstrap,
  chooseBinary,
  chooseWorkspace,
  findBinary,
  installBinary,
  sendPrompt,
} from '../state/actions';
import { store, useStore } from '../state/store';
import { Composer } from './Composer';
import { PeggForever } from './PeggForever';
import { Starburst } from './icons';

const SUGGESTIONS = [
  {
    icon: Sparkles,
    labelKey: 'welcome.explain',
    promptKey: 'welcome.explainPrompt',
  },
  {
    icon: Bug,
    labelKey: 'welcome.bug',
    promptKey: 'welcome.bugPrompt',
  },
  {
    icon: ListChecks,
    labelKey: 'welcome.tests',
    promptKey: 'welcome.testsPrompt',
  },
  {
    icon: GitPullRequest,
    labelKey: 'welcome.review',
    promptKey: 'welcome.reviewPrompt',
  },
];

export function Welcome() {
  const workspace = useStore((s) => s.workspace);
  const platform = useStore((s) => s.boot?.platform ?? 'linux');
  const t = useT();

  return (
    <div className="welcome">
      <PeggForever />
      <div className="welcome-inner">
        <div className="welcome-greeting">
          <h1 className="welcome-title">
            {t(greetingKey())
              .split(' ')
              .filter(Boolean)
              .map((word, index) => (
                <span className="welcome-line" key={`${word}-${index}`}>
                  {word}
                </span>
              ))}
          </h1>
        </div>
        <p className="welcome-sub">
          {workspace ? t('welcome.working', { workspace: baseName(workspace) }) : t('welcome.choose')}
        </p>
        <div className="welcome-composer">
          <Composer autoFocus />
        </div>
        <div className="chips">
          {SUGGESTIONS.map((item) => (
            <button
              key={item.labelKey}
              className="chip"
              onClick={() => {
                if (!store.get().workspace) {
                  void chooseWorkspace();
                  return;
                }
                void sendPrompt({ text: t(item.promptKey) });
              }}
            >
              <item.icon size={13} />
              {t(item.labelKey)}
            </button>
          ))}
          {!workspace ? (
            <button className="chip" onClick={() => void chooseWorkspace()}>
              <FolderOpen size={13} />
              {t('welcome.openProject')}
            </button>
          ) : null}
        </div>
        <div className="welcome-hint">
          <kbd>{platform === 'darwin' ? '⌘' : 'Ctrl'}</kbd>
          <kbd>K</kbd>
          <span>{t('welcome.forCommands')}</span>
          <span style={{ opacity: 0.5 }}>·</span>
          <kbd>Enter</kbd>
          <span>{t('welcome.toSend')}</span>
          <span style={{ opacity: 0.5 }}>·</span>
          <kbd>@</kbd>
          <span>{t('welcome.files')}</span>
          <kbd>/</kbd>
          <span>{t('welcome.commands')}</span>
        </div>
      </div>
    </div>
  );
}

export function Onboarding() {
  const installing = useStore((s) => s.installing);
  const progress = useStore((s) => s.installProgress);
  const binary = useStore((s) => s.binary);

  return (
    <div className="onboarding">
      <h1>
        <Starburst size={34} />
        Welcome to Pegg
      </h1>
      <p>
        Pegg needs its agent runtime. This app can use an existing <code>pegg</code> install or
        download a verified release into <code>~/.pegg/bin</code>.
      </p>
      <div className="steps">
        <div className="step">
          <span className="num">1</span>
          Find an existing binary on your PATH, Homebrew, or <code>~/.pegg/bin</code>.
        </div>
        <div className="step">
          <span className="num">2</span>
          Or let Pegg download the latest signed-checksum release.
        </div>
        <div className="step">
          <span className="num">3</span>
          Pick a project and start a session.
        </div>
      </div>
      {installing && progress ? (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
          <div className="progress-track">
            <div className="progress-fill" style={{ width: `${progress.percent}%` }} />
          </div>
          <div style={{ fontSize: 11.5, color: 'var(--pg-muted)' }}>
            {progress.stage}
            {progress.detail ? ` · ${progress.detail}` : ''}
          </div>
        </div>
      ) : null}
      {binary.error ? (
        <div className="notice error" style={{ marginTop: 4 }}>
          <TriangleAlert size={14} />
          {binary.error}
        </div>
      ) : null}
      <div className="actions">
        <button className="btn primary" disabled={installing} onClick={() => void installBinary()}>
          <Terminal size={14} />
          {installing ? 'Installing…' : 'Download Pegg'}
        </button>
        <button className="btn" disabled={installing} onClick={() => void findBinary()}>
          Find existing binary
        </button>
        <button className="btn ghost" disabled={installing} onClick={() => void chooseBinary()}>
          Locate…
        </button>
      </div>
    </div>
  );
}

export function StatusBanner() {
  const status = useStore((s) => s.status);
  const binary = useStore((s) => s.binary);
  const loading = useStore((s) => s.chat.loading);

  if (!binary.found) return null;
  if (status.state === 'connected' || status.state === 'connecting') return null;
  if (status.state === 'disconnected') return null;

  return (
    <div className={`banner${status.state === 'error' ? ' error' : ''}`}>
      <TriangleAlert size={14} />
      <span>
        {status.state === 'exited'
          ? 'The Pegg server stopped.'
          : status.message || 'The Pegg server is unavailable.'}
      </span>
      <span className="spacer" />
      <button
        className="btn small"
        disabled={loading}
        onClick={() => {
          void bootstrap();
        }}
      >
        Reconnect
      </button>
    </div>
  );
}

export function Toasts() {
  const toasts = useStore((s) => s.toasts);
  if (!toasts.length) return null;
  return (
    <div className="toasts" role="status" aria-live="polite">
      {toasts.map((toast) => (
        <div key={toast.id} className={`toast ${toast.kind}`}>
          <TriangleAlert size={13} />
          <span>{toast.text}</span>
          <button className="close" aria-label="Dismiss" onClick={() => store.dismissToast(toast.id)}>
            <X size={12} />
          </button>
        </div>
      ))}
    </div>
  );
}
