import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react';
import {
  filterConversation,
  type ChatItem,
  type ChatState,
  type PendingPermission,
  type PendingQuestion,
  type ToolState,
} from '../chat/state';
import { renderMarkdown } from './markdown';
import { vscode } from './vscode';

interface StateMessage {
  type: 'state';
  state: ChatState;
  settings: { sendOnEnter: 'enter' | 'ctrl+enter'; defaultMode: string };
}

interface TranscriptMessage {
  type: 'transcript';
  text: string;
}

interface FilesMessage {
  type: 'files';
  files: string[];
}

interface RecordingMessage {
  type: 'recording';
  recording: boolean;
}

interface VoiceErrorMessage {
  type: 'voiceError';
  error: string;
}

interface ComposerTextMessage {
  type: 'composerText';
  text: string;
}

type Incoming =
  | StateMessage
  | TranscriptMessage
  | FilesMessage
  | RecordingMessage
  | VoiceErrorMessage
  | ComposerTextMessage;

interface PopupState {
  kind: 'mention' | 'command';
  anchor: number;
  query: string;
  selected: number;
}

interface Suggestion {
  kind: 'file' | 'command';
  value: string;
  label: string;
  detail: string;
}

const fallbackCommands = [
  { name: 'init', description: 'Analyze this codebase and create or improve AGENTS.md' },
  { name: 'review', description: 'Review code or changes for bugs and missing tests' },
  { name: 'batch', description: 'Plan and execute a large parallel change' },
  { name: 'rule', description: 'Create or update a project rule' },
];

const emptyState: ChatState = {
  title: 'New session',
  mode: 'auto',
  connected: false,
  running: false,
  items: [],
  plan: [],
  usage: { used: 0, size: 0 },
  commands: [],
  pending: [],
  composerAttachments: [],
  counter: 0,
};

const startSuggestions = [
  ['Explain this codebase', 'Explain the structure of this project and where I should start.'],
  ['Fix the current file', 'Review the active file, find bugs, and fix them.'],
  ['Review my changes', '/review'],
];

export function App() {
  const [state, setState] = useState<ChatState>(emptyState);
  const [settings, setSettings] = useState<StateMessage['settings']>({
    sendOnEnter: 'enter',
    defaultMode: 'auto',
  });
  const [text, setText] = useState('');
  const [files, setFiles] = useState<string[]>([]);
  const [popup, setPopup] = useState<PopupState | null>(null);
  const [recording, setRecording] = useState(false);
  const [localError, setLocalError] = useState('');
  const [atBottom, setAtBottom] = useState(true);
  const textarea = useRef<HTMLTextAreaElement>(null);
  const messages = useRef<HTMLElement>(null);
  const selectedRow = useRef<HTMLButtonElement>(null);
  const stick = useRef(true);
  const textRef = useRef('');
  const [menu, setMenu] = useState<{ item: ChatItem; rect: DOMRect } | null>(null);
  const [agentView, setAgentView] = useState<{ id: string; name?: string } | null>(null);

  const openMessageMenu = useCallback((item: ChatItem, rect: DOMRect) => {
    setMenu((current) => (current?.item.id === item.id ? null : { item, rect }));
  }, []);

  const openSubagent = useCallback((agentId: string, name?: string) => {
    setAgentView({ id: agentId, name });
    setMenu(null);
  }, []);

  useEffect(() => {
    setAgentView(null);
  }, [state.acpSessionId, state.peggSessionId]);

  const subagentIds = useMemo(() => {
    const ids = new Set<string>();
    for (const item of state.items) {
      if (item.kind === 'subagent') ids.add(item.agentId);
    }
    return ids;
  }, [state.items]);

  const visibleItems = useMemo(
    () => filterConversation(state.items, agentView?.id ?? null, subagentIds),
    [state.items, agentView, subagentIds],
  );

  const activeSubagent = useMemo(() => {
    if (!agentView) return undefined;
    return state.items.find(
      (item): item is Extract<ChatItem, { kind: 'subagent' }> =>
        item.kind === 'subagent' && item.agentId === agentView.id,
    );
  }, [state.items, agentView]);

  const scrollToBottom = useCallback((smooth = false) => {
    const el = messages.current;
    if (!el) return;
    el.scrollTo({ top: el.scrollHeight, behavior: smooth ? 'smooth' : 'auto' });
    stick.current = true;
    setAtBottom(true);
  }, []);

  const setCaret = useCallback((position: number) => {
    requestAnimationFrame(() => {
      const el = textarea.current;
      if (!el) return;
      el.focus();
      el.setSelectionRange(position, position);
    });
  }, []);

  const insertText = useCallback(
    (value: string, replaceAnchor: number | null = null) => {
      const el = textarea.current;
      const caret = el?.selectionStart ?? textRef.current.length;
      const start = replaceAnchor ?? caret;
      const end = replaceAnchor === null ? (el?.selectionEnd ?? caret) : Math.max(start + 1, caret);
      const current = textRef.current;
      const next = `${current.slice(0, start)}${value}${current.slice(end)}`;
      textRef.current = next;
      setText(next);
      setCaret(start + value.length);
    },
    [setCaret],
  );

  useEffect(() => {
    const onMessage = (event: MessageEvent<Incoming>) => {
      const data = event.data;
      if (data.type === 'state') {
        setState(data.state);
        setSettings(data.settings);
      } else if (data.type === 'transcript') {
        insertText(data.text);
      } else if (data.type === 'files') {
        setFiles(data.files);
      } else if (data.type === 'recording') {
        setRecording(data.recording);
      } else if (data.type === 'voiceError') {
        setRecording(false);
        setLocalError(data.error);
      } else if (data.type === 'composerText') {
        textRef.current = data.text;
        setText(data.text);
        requestAnimationFrame(() => {
          const el = textarea.current;
          if (!el) return;
          el.focus();
          el.setSelectionRange(data.text.length, data.text.length);
        });
      }
    };
    window.addEventListener('message', onMessage);
    vscode.postMessage({ type: 'ready' });
    return () => window.removeEventListener('message', onMessage);
  }, [insertText]);

  useEffect(() => {
    if (!menu) return;
    const close = () => setMenu(null);
    const onKey = (event: KeyboardEvent) => {
      if (event.key !== 'Escape') return;
      event.preventDefault();
      close();
    };
    const onPointerDown = (event: MouseEvent) => {
      const target = event.target as HTMLElement | null;
      if (target?.closest('.message-menu')) return;
      if (target?.closest('.message-body.clickable')) return;
      close();
    };
    window.addEventListener('keydown', onKey, true);
    window.addEventListener('mousedown', onPointerDown, true);
    window.addEventListener('scroll', close, true);
    window.addEventListener('resize', close);
    return () => {
      window.removeEventListener('keydown', onKey, true);
      window.removeEventListener('mousedown', onPointerDown, true);
      window.removeEventListener('scroll', close, true);
      window.removeEventListener('resize', close);
    };
  }, [menu]);

  useEffect(() => {
    if (!stick.current) return;
    const frame = requestAnimationFrame(() => scrollToBottom(false));
    return () => cancelAnimationFrame(frame);
  }, [state, scrollToBottom]);

  useEffect(() => {
    if (!textarea.current) return;
    const el = textarea.current;
    el.style.height = 'auto';
    el.style.height = `${Math.min(Math.max(el.scrollHeight, 44), 200)}px`;
  }, [text]);

  const pendingIds = useMemo(() => new Set(state.pending.map((p) => p.id)), [state.pending]);
  const commandList = state.commands.length > 0 ? state.commands : fallbackCommands;
  const activeSuggestions = useMemo(() => buildSuggestions(popup, files, commandList), [popup, files, commandList]);

  useEffect(() => {
    selectedRow.current?.scrollIntoView({ block: 'nearest' });
  }, [popup?.selected, activeSuggestions.length]);

  function onScroll() {
    const el = messages.current;
    if (!el) return;
    const distance = el.scrollHeight - el.scrollTop - el.clientHeight;
    stick.current = distance < 48;
    setAtBottom(distance < 48);
  }

  function send() {
    if (state.running) return;
    const value = text.trim();
    if (!value && state.composerAttachments.length === 0) return;
    vscode.postMessage({ type: 'send', text: value });
    setText('');
    textRef.current = '';
    setPopup(null);
    stick.current = true;
    requestAnimationFrame(() => scrollToBottom(true));
  }

  function chooseSuggestion(item: Suggestion) {
    if (!popup) return;
    const el = textarea.current;
    const caret = el?.selectionStart ?? textRef.current.length;
    const current = textRef.current;
    const start = popup.anchor;
    const end = Math.max(start + 1, caret);
    const replacement = `${popup.kind === 'mention' ? '@' : '/'}${item.value} `;
    const next = `${current.slice(0, start)}${replacement}${current.slice(end)}`;
    textRef.current = next;
    setText(next);
    setPopup(null);
    setCaret(start + replacement.length);
  }

  function onKeyDown(event: React.KeyboardEvent<HTMLTextAreaElement>) {
    if (popup && activeSuggestions.length > 0) {
      if (event.key === 'ArrowDown') {
        event.preventDefault();
        setPopup({ ...popup, selected: (popup.selected + 1) % activeSuggestions.length });
        return;
      }
      if (event.key === 'ArrowUp') {
        event.preventDefault();
        setPopup({ ...popup, selected: (popup.selected - 1 + activeSuggestions.length) % activeSuggestions.length });
        return;
      }
      if (event.key === 'Enter' || event.key === 'Tab') {
        event.preventDefault();
        chooseSuggestion(activeSuggestions[popup.selected] ?? activeSuggestions[0]);
        return;
      }
    }
    if (event.key === 'Escape') {
      setPopup(null);
      return;
    }
    if (event.key === 'Enter' && !event.shiftKey) {
      const shouldSend =
        settings.sendOnEnter === 'enter' ? !event.ctrlKey && !event.metaKey : event.ctrlKey || event.metaKey;
      if (shouldSend) {
        event.preventDefault();
        send();
      }
    }
  }

  function onChange(value: string) {
    const previous = textRef.current;
    textRef.current = value;
    setText(value);
    const caret = textarea.current?.selectionStart ?? value.length;
    const inserted = value.length === previous.length + 1 ? firstDifference(previous, value) : -1;
    let next = popup;

    if (next) {
      const marker = next.kind === 'mention' ? '@' : '/';
      const query = value.slice(next.anchor + 1, caret);
      const invalid =
        caret <= next.anchor ||
        next.anchor >= value.length ||
        value[next.anchor] !== marker ||
        /\s/.test(query);
      if (invalid) next = null;
      else if (query !== next.query || next.selected !== 0) next = { ...next, query, selected: 0 };
    }

    if (inserted >= 0) {
      const char = value[inserted];
      if (char === '@' && (inserted === 0 || /\s/.test(value[inserted - 1]))) {
        next = { kind: 'mention', anchor: inserted, query: value.slice(inserted + 1, caret), selected: 0 };
        vscode.postMessage({ type: 'listFiles' });
      } else if (char === '/' && inserted === 0) {
        next = { kind: 'command', anchor: 0, query: value.slice(1, caret), selected: 0 };
      }
    }
    setPopup(next);
  }

  function toggleRecording() {
    setLocalError('');
    if (recording) {
      vscode.postMessage({ type: 'stopVoice' });
      return;
    }
    setRecording(true);
    vscode.postMessage({ type: 'startVoice' });
  }

  function onDrop(event: React.DragEvent) {
    event.preventDefault();
    const uriList = event.dataTransfer.getData('text/uri-list');
    const paths = uriList
      .split(/\r?\n/)
      .filter((line) => line.startsWith('file://'))
      .map((line) => decodeURIComponent(new URL(line).pathname));
    if (paths.length > 0) {
      vscode.postMessage({ type: 'attachPaths', paths });
      return;
    }
    const files = Array.from(event.dataTransfer.files);
    for (const file of files) {
      if (file.type.startsWith('image/')) {
        void fileToBase64(file).then((data) =>
          vscode.postMessage({
            type: 'addAttachmentData',
            name: file.name,
            mediaType: file.type,
            size: file.size,
            data,
          }),
        );
      } else {
        setLocalError('Drop files from the explorer, or use the + button to attach.');
      }
    }
  }

  function onPaste(event: React.ClipboardEvent) {
    const image = Array.from(event.clipboardData.items).find((item) => item.type.startsWith('image/'));
    if (!image) return;
    const file = image.getAsFile();
    if (!file) return;
    event.preventDefault();
    void fileToBase64(file).then((data) =>
      vscode.postMessage({
        type: 'addAttachmentData',
        name: file.name || 'pasted-image.png',
        mediaType: file.type,
        size: file.size,
        data,
      }),
    );
  }

  const usage = state.usage;
  const cost = usage.cost !== undefined ? ` · $${usage.cost.toFixed(4)}` : '';
  const tokenText = usage.used > 0 ? `${formatNumber(usage.used)} tokens${cost}` : 'Ready';
  const modelLabel = state.model || state.provider || 'Auto';

  return (
    <div className="app" onDrop={onDrop} onDragOver={(event) => event.preventDefault()}>

      {agentView && (
        <div className="agent-view-bar">
          <button className="agent-back" onClick={() => setAgentView(null)}>
            ← Back
          </button>
          <span className="agent-view-name">{activeSubagent?.agentName || agentView.name || 'Subagent'}</span>
          {activeSubagent?.task && <span className="agent-view-task">{activeSubagent.task}</span>}
          <span className={`agent-view-status status-${activeSubagent?.status ?? 'running'}`}>
            {activeSubagent?.status === 'running' ? 'working…' : activeSubagent?.status === 'error' ? 'failed' : 'done'}
          </span>
        </div>
      )}

      <main className="messages" ref={messages} onScroll={onScroll}>
        {!agentView && state.items.length === 0 && !state.running && (
          <div className="empty">
            <div className="empty-title">Ask Pegg about this workspace</div>
            <div className="empty-subtitle">
              Explain code, fix a bug, review changes, or write tests. Type <code>/</code> for skills and{' '}
              <code>@</code> to mention files.
            </div>
            <div className="suggestions">
              {startSuggestions.map(([label, value]) => (
                <button key={label} className="suggestion" onClick={() => setText(value)}>
                  {label}
                </button>
              ))}
            </div>
          </div>
        )}

        {visibleItems.map((item) => (
          <ItemView
            key={item.id}
            item={item}
            onOpenMenu={openMessageMenu}
            onOpenSubagent={openSubagent}
          />
        ))}

        {agentView && visibleItems.length === 0 && activeSubagent?.status === 'running' && (
          <div className="agent-view-empty">
            <span className="spinner" />
            <span>{activeSubagent.agentName || 'The subagent'} is working…</span>
          </div>
        )}

        {state.pending.map((pending) =>
          pending.kind === 'permission' ? (
            <PermissionCard key={pending.id} pending={pending} />
          ) : (
            <QuestionCard key={pending.id} pending={pending} />
          ),
        )}

        {!agentView && state.running && (
          <div className="working">
            <span className="spinner" />
            <span>Pegg is working…</span>
          </div>
        )}
        <div className="messages-spacer" />
      </main>

      {!atBottom && (
        <button className="scroll-bottom" title="Scroll to bottom" onClick={() => scrollToBottom(true)}>
          <ArrowDownIcon />
        </button>
      )}

      {state.plan.length > 0 && (
        <section className="plan">
          <div className="plan-title">Plan</div>
          <ul>
            {state.plan.map((entry, index) => (
              <li key={`${entry.content}-${index}`} className={`plan-${entry.status ?? 'pending'}`}>
                <span className="plan-status">{planIcon(entry.status)}</span>
                <span>{entry.content}</span>
              </li>
            ))}
          </ul>
        </section>
      )}

      <footer className={`composer-wrap ${agentView ? 'disabled' : ''}`}>
        <div className="composer-tip">
          {agentView
            ? 'Viewing a subagent — go back to continue the conversation.'
            : (
                <>
                  Tip: Type <code>/</code> for skills and <code>@</code> to mention files or folders.
                </>
              )}
        </div>

        <div className="composer">
        {popup && activeSuggestions.length > 0 && (
          <div className="suggestions-popup" role="listbox">
            {activeSuggestions.map((item, index) => (
              <button
                key={`${item.kind}-${item.value}`}
                ref={index === popup.selected ? selectedRow : undefined}
                className={`suggestion-row ${index === popup.selected ? 'selected' : ''}`}
                role="option"
                aria-selected={index === popup.selected}
                onMouseDown={(event) => event.preventDefault()}
                onClick={() => chooseSuggestion(item)}
              >
                {item.kind === 'file' ? <FileBadge path={item.value} /> : <CommandBadge />}
                <span className="suggestion-name">{item.label}</span>
                <span className="suggestion-detail">{item.detail}</span>
              </button>
            ))}
          </div>
        )}

          {state.composerAttachments.length > 0 && (
            <div className="attachments">
              {state.composerAttachments.map((attachment) => (
                <div className="attachment" key={attachment.id} title={attachment.path ?? attachment.name}>
                  <span className="attachment-name">{attachment.name}</span>
                  <button
                    className="attachment-remove"
                    title="Remove"
                    onClick={() => vscode.postMessage({ type: 'removeAttachment', id: attachment.id })}
                  >
                    ×
                  </button>
                </div>
              ))}
            </div>
          )}
          {(localError || state.error) && <div className="composer-error">{localError || state.error}</div>}
          <textarea
            ref={textarea}
            value={text}
            rows={1}
            placeholder={agentView ? 'Viewing a subagent chat' : 'Ask a question about your code or project'}
            disabled={Boolean(agentView)}
            onChange={(event) => onChange(event.target.value)}
            onKeyDown={onKeyDown}
            onPaste={onPaste}
          />
          <div className="composer-toolbar">
            <div className="left-actions">
              <button className="icon-button" title="Attach files" onClick={() => vscode.postMessage({ type: 'pickFiles' })}>
                <PlusIcon />
              </button>
              <button className="composer-pill" onClick={() => vscode.postMessage({ type: 'pickMode' })} title="Change mode">
                <AgentIcon />
                {modeLabel(state.mode || 'auto')}
              </button>
              <button
                className="composer-pill model-pill"
                onClick={() => vscode.postMessage({ type: 'pickModel' })}
                title="Change model"
              >
                <ModelIcon />
                {modelLabel}
              </button>
            </div>
            <div className="right-actions">
              <button className="icon-button" title="Settings" onClick={() => vscode.postMessage({ type: 'openSettings' })}>
                <SlidersIcon />
              </button>
              <button
                className={`icon-button ${recording ? 'recording' : ''}`}
                title={recording ? 'Stop recording' : 'Voice input'}
                onClick={toggleRecording}
              >
                <MicIcon />
              </button>
              {state.running ? (
                <button className="send stop" title="Stop" onClick={() => vscode.postMessage({ type: 'stop' })}>
                  <StopIcon />
                </button>
              ) : (
                <button
                  className="send"
                  title="Send"
                  disabled={Boolean(agentView) || (!text.trim() && state.composerAttachments.length === 0)}
                  onClick={send}
                >
                  <ArrowUpIcon />
                </button>
              )}
            </div>
          </div>
        </div>
        <div className="status-line">
          <span>{tokenText}</span>
          {pendingIds.size > 0 && <span className="waiting">Waiting for your input</span>}
        </div>
      </footer>

      {menu && menu.item.kind === 'user' && (
        <MessageMenu
          rect={menu.rect}
          supported={Boolean(state.historySupported)}
          running={state.running}
          onCopy={() => {
            vscode.postMessage({ type: 'copyText', text: menu.item.kind === 'user' ? menu.item.text : '' });
            setMenu(null);
          }}
          onFork={() => {
            vscode.postMessage({ type: 'forkMessage', id: menu.item.id });
            setMenu(null);
          }}
          onRevert={() => {
            vscode.postMessage({ type: 'revertMessage', id: menu.item.id });
            setMenu(null);
          }}
        />
      )}
    </div>
  );
}

function MessageMenu({
  rect,
  supported,
  running,
  onCopy,
  onFork,
  onRevert,
}: {
  rect: DOMRect;
  supported: boolean;
  running: boolean;
  onCopy: () => void;
  onFork: () => void;
  onRevert: () => void;
}) {
  const ref = useRef<HTMLDivElement>(null);
  const [position, setPosition] = useState<{ top: number; left: number } | null>(null);

  useLayoutEffect(() => {
    const element = ref.current;
    if (!element) return;
    const menuRect = element.getBoundingClientRect();
    const gap = 6;
    const margin = 8;
    const below = rect.bottom + gap;
    const above = rect.top - menuRect.height - gap;
    const fitsBelow = below + menuRect.height <= window.innerHeight - margin;
    const top = fitsBelow || above < margin ? below : above;
    const maxLeft = Math.max(margin, window.innerWidth - menuRect.width - margin);
    const left = Math.min(Math.max(margin, rect.left), maxLeft);
    setPosition({ top, left });
  }, [rect, supported, running]);

  return (
    <div
      ref={ref}
      className="message-menu"
      role="menu"
      aria-label="Message actions"
      style={{ ...position, visibility: position ? 'visible' : 'hidden' }}
    >
      <button type="button" className="message-menu-item" role="menuitem" onClick={onCopy}>
        <CopyIcon />
        <span className="message-menu-text">
          <span className="message-menu-label">Copy</span>
          <span className="message-menu-detail">Copy the message text</span>
        </span>
      </button>
      <div className="message-menu-separator" role="separator" />
      <button
        type="button"
        className="message-menu-item"
        role="menuitem"
        disabled={running || !supported}
        title={supported ? undefined : 'Update the Pegg binary to use this action'}
        onClick={onFork}
      >
        <ForkIcon />
        <span className="message-menu-text">
          <span className="message-menu-label">Fork from here</span>
          <span className="message-menu-detail">
            {running
              ? 'Wait for Pegg to finish'
              : supported
                ? 'New session with the messages before this one'
                : 'Requires a newer Pegg binary'}
          </span>
        </span>
      </button>
      <button
        type="button"
        className="message-menu-item danger"
        role="menuitem"
        disabled={running || !supported}
        title={supported ? undefined : 'Update the Pegg binary to use this action'}
        onClick={onRevert}
      >
        <RevertIcon />
        <span className="message-menu-text">
          <span className="message-menu-label">Revert to here</span>
          <span className="message-menu-detail">
            {running
              ? 'Wait for Pegg to finish'
              : supported
                ? 'Removes this message and everything after'
                : 'Requires a newer Pegg binary'}
          </span>
        </span>
      </button>
      {!supported && (
        <p className="message-menu-note">
          <WarningIcon />
          The running Pegg binary is too old. Update it with <strong>Pegg: Install or Update Binary</strong>,
          then restart the server.
        </p>
      )}
    </div>
  );
}

function CopyIcon() {
  return (
    <svg {...iconProps}>
      <rect x="5.5" y="5.5" width="8" height="8" rx="1.5" />
      <path d="M10.5 3.5H4A1.5 1.5 0 0 0 2.5 5v6.5" />
    </svg>
  );
}

function WarningIcon() {
  return (
    <svg {...iconProps}>
      <path d="M8 2.5 14 13H2L8 2.5Z" />
      <path d="M8 6.5v3M8 11.2v.1" />
    </svg>
  );
}

function ForkIcon() {
  return (
    <svg {...iconProps}>
      <circle cx="4.5" cy="4" r="1.5" />
      <circle cx="4.5" cy="12" r="1.5" />
      <circle cx="11.5" cy="8" r="1.5" />
      <path d="M4.5 5.5v5M6 8h4" />
    </svg>
  );
}

function RevertIcon() {
  return (
    <svg {...iconProps}>
      <path d="M3 7.5h7a3 3 0 0 1 0 6H6" />
      <path d="M5.5 5 3 7.5 5.5 10" />
    </svg>
  );
}

function ItemView({
  item,
  onOpenMenu,
  onOpenSubagent,
}: {
  item: ChatItem;
  onOpenMenu?: (item: ChatItem, rect: DOMRect) => void;
  onOpenSubagent?: (agentId: string, name?: string) => void;
}) {
  if (item.kind === 'user') {
    const owned = Boolean(item.agentId);
    const open = (element: HTMLElement) => {
      if (owned) return;
      const selection = window.getSelection();
      if (selection && selection.toString().trim()) return;
      onOpenMenu?.(item, element.getBoundingClientRect());
    };
    return (
      <article className={`message user${owned ? ' subagent-prompt' : ''}`}>
        <div className="message-role">{owned ? 'Task' : 'You'}</div>
        <div
          className={`message-body plain${owned ? '' : ' clickable'}`}
          role="button"
          tabIndex={0}
          aria-haspopup={owned ? undefined : 'menu'}
          title={owned ? undefined : 'Message actions'}
          onClick={(event) => open(event.currentTarget)}
          onKeyDown={(event) => {
            if (event.key !== 'Enter' && event.key !== ' ') return;
            event.preventDefault();
            open(event.currentTarget);
          }}
        >
          {item.text}
          <span className="message-menu-hint" aria-hidden="true">
            ⋯
          </span>
        </div>
      </article>
    );
  }
  if (item.kind === 'assistant') {
    return (
      <article className="message assistant">
        <div className="message-role">{item.agentName && item.agentName !== 'orchestrator' ? item.agentName : 'Pegg'}</div>
        <AssistantBody text={item.text} />
      </article>
    );
  }
  if (item.kind === 'reasoning') {
    return (
      <details className="reasoning">
        <summary>{item.agentName && item.agentName !== 'orchestrator' ? `${item.agentName} reasoning` : 'Reasoning'}</summary>
        <div className="reasoning-body">{item.text}</div>
      </details>
    );
  }
  if (item.kind === 'subagent') {
    return <SubagentCard item={item} onOpen={() => onOpenSubagent?.(item.agentId, item.agentName)} />;
  }
  if (item.kind === 'tool') {
    if (item.tool.name === 'task') return null;
    return <ToolCard tool={item.tool} />;
  }
  if (item.kind === 'error') return <div className="notice error">{item.text}</div>;
  return <div className={`notice ${item.level}`}>{item.text}</div>;
}

function AssistantBody({ text }: { text: string }) {
  const html = useMemo(() => renderMarkdown(text), [text]);
  return <div className="message-body markdown" dangerouslySetInnerHTML={{ __html: html }} onClick={handleLinkClick} />;
}

function SubagentCard({
  item,
  onOpen,
}: {
  item: Extract<ChatItem, { kind: 'subagent' }>;
  onOpen: () => void;
}) {
  const running = item.status === 'running';
  const label = item.agentName || 'Subagent';
  return (
    <div className={`subagent-card status-${item.status}`}>
      <button className="subagent-head" onClick={onOpen} title="Open this subagent chat">
        <span className={`subagent-dot status-${item.status}`} />
        <span className="subagent-name">{label}</span>
        <span className="subagent-status">
          {running ? 'working…' : item.status === 'error' ? 'failed' : 'done'}
        </span>
        <span className="subagent-open">open chat</span>
      </button>
      {item.task && <div className="subagent-task">{item.task}</div>}
      {item.error && <div className="subagent-error">{stripAnsi(item.error)}</div>}
    </div>
  );
}

function ToolCard({ tool }: { tool: ToolState }) {
  const [open, setOpen] = useState(false);
  const name = (tool.name ?? '').toLowerCase();
  const isBash = name === 'bash' || tool.kind === 'execute' || /^bash\b/i.test(tool.title);
  const hasDiff = Boolean(tool.diff);
  const output = stripAnsi(tool.output === undefined ? '' : pretty(tool.output));
  const hasOutput = isBash && output.length > 0;
  const failed = Boolean(tool.error);
  const longOutput = hasOutput && output.split('\n').length > 9;
  const canExpand = failed || longOutput;

  const head = (
    <>
      <span className={`tool-kind kind-${isBash ? 'execute' : tool.kind || 'other'}`}>
        {kindLabel(tool.kind, tool.title)}
      </span>
      <span className="tool-title">{tool.title || name || 'Tool'}</span>
      {tool.agentName && tool.agentName !== 'orchestrator' && <span className="tool-agent">{tool.agentName}</span>}
      <span className="tool-status">{statusLabel(tool.status)}</span>
      {canExpand && <span className="tool-chevron">{open ? '−' : '+'}</span>}
    </>
  );

  return (
    <div className={`tool-card status-${tool.status}${canExpand ? '' : ' compact'}`}>
      {canExpand ? (
        <button className="tool-head" onClick={() => setOpen((value) => !value)} aria-expanded={open}>
          {head}
        </button>
      ) : (
        <div className="tool-head static">{head}</div>
      )}

      {failed && open && <div className="tool-error">{stripAnsi(tool.error ?? '')}</div>}
      {failed && !open && (
        <div className="tool-error-preview">{stripAnsi(tool.error ?? '').split('\n')[0]}</div>
      )}

      {hasOutput && (!longOutput || open) && (
        <div className="tool-output-wrap">
          <pre className="tool-output expanded">{output}</pre>
        </div>
      )}

      {(hasDiff || (open && hasOutput)) && (
        <div className="tool-buttons">
          {tool.locations?.map((location, index) => (
            <button
              key={`${location.path}-${index}`}
              className="mini-button"
              onClick={() => vscode.postMessage({ type: 'openFile', path: location.path, line: location.line ?? 1 })}
            >
              Open {location.path.split('/').pop()}
            </button>
          ))}
          {hasOutput && (
            <button className="mini-button" onClick={() => vscode.postMessage({ type: 'copyText', text: output })}>
              Copy output
            </button>
          )}
          {tool.diff && (
            <button
              className="mini-button"
              onClick={() =>
                vscode.postMessage({
                  type: 'openDiff',
                  path: tool.diff?.path,
                  oldText: tool.diff?.oldText ?? '',
                  newText: tool.diff?.newText ?? '',
                })
              }
            >
              View diff
            </button>
          )}
        </div>
      )}
    </div>
  );
}

function PermissionCard({ pending }: { pending: PendingPermission }) {
  return (
    <div className="interaction">
      <div className="interaction-title">Permission required</div>
      <pre className="interaction-question">{pending.question}</pre>
      <div className="interaction-actions">
        {pending.options.map((option) => (
          <button
            key={option.optionId}
            className={option.kind.startsWith('reject') ? 'danger' : option.kind === 'allow_always' ? 'primary' : ''}
            onClick={() => vscode.postMessage({ type: 'permissionResponse', id: pending.id, optionId: option.optionId })}
          >
            {option.name}
          </button>
        ))}
      </div>
    </div>
  );
}

function QuestionCard({ pending }: { pending: PendingQuestion }) {
  const [answers, setAnswers] = useState<Record<string, string>>(pending.answers ?? {});
  return (
    <div className="interaction">
      <div className="interaction-title">{pending.agentName ? `${pending.agentName} asks` : 'Input required'}</div>
      {pending.questions.map((question) => (
        <div className="question" key={question.id}>
          <div className="question-label">{question.question}</div>
          {question.type === 'text' ? (
            <input
              value={answers[question.id] ?? ''}
              onChange={(event) => setAnswers({ ...answers, [question.id]: event.target.value })}
            />
          ) : question.type === 'boolean' ? (
            <div className="interaction-actions">
              {['true', 'false'].map((value) => (
                <button
                  key={value}
                  className={answers[question.id] === value ? 'primary' : ''}
                  onClick={() => setAnswers({ ...answers, [question.id]: value })}
                >
                  {value === 'true' ? 'Yes' : 'No'}
                </button>
              ))}
            </div>
          ) : question.type === 'multiselect' ? (
            <div className="multi-options">
              {(question.options ?? []).map((option) => {
                const selected = (answers[question.id] ?? '').split(', ').filter(Boolean);
                return (
                  <label key={option}>
                    <input
                      type="checkbox"
                      checked={selected.includes(option)}
                      onChange={(event) => {
                        const next = event.target.checked
                          ? [...selected, option]
                          : selected.filter((value) => value !== option);
                        setAnswers({ ...answers, [question.id]: next.join(', ') });
                      }}
                    />
                    {option}
                  </label>
                );
              })}
            </div>
          ) : (
            <select
              value={answers[question.id] ?? ''}
              onChange={(event) => setAnswers({ ...answers, [question.id]: event.target.value })}
            >
              <option value="">Select…</option>
              {(question.options ?? []).map((option) => (
                <option key={option} value={option}>
                  {option}
                </option>
              ))}
            </select>
          )}
        </div>
      ))}
      <div className="interaction-actions">
        <button className="primary" onClick={() => vscode.postMessage({ type: 'questionResponse', id: pending.id, answers })}>
          Submit
        </button>
      </div>
    </div>
  );
}

function handleLinkClick(event: React.MouseEvent) {
  const target = (event.target as HTMLElement).closest('a');
  if (!target) return;
  event.preventDefault();
  const href = target.getAttribute('data-href') || target.getAttribute('href') || '';
  if (/^https?:\/\//i.test(href)) {
    vscode.postMessage({ type: 'openExternal', url: href });
  } else if (href.startsWith('file://')) {
    vscode.postMessage({ type: 'openFile', path: decodeURIComponent(new URL(href).pathname) });
  }
}

function buildSuggestions(
  popup: PopupState | null,
  files: string[],
  commands: { name: string; description?: string }[],
): Suggestion[] {
  if (!popup) return [];
  const query = popup.query.toLowerCase();
  if (popup.kind === 'command') {
    return commands
      .filter((command) => !query || command.name.toLowerCase().includes(query))
      .sort((a, b) => {
        const ap = a.name.toLowerCase().startsWith(query) ? 1 : 0;
        const bp = b.name.toLowerCase().startsWith(query) ? 1 : 0;
        return bp - ap || a.name.localeCompare(b.name);
      })
      .slice(0, 50)
      .map((command) => ({
        kind: 'command' as const,
        value: command.name,
        label: `/${command.name}`,
        detail: command.description ?? '',
      }));
  }

  const scored = files
    .map((path) => {
      const base = path.split('/').pop() ?? path;
      const lower = path.toLowerCase();
      const lowerBase = base.toLowerCase();
      let score = 0;
      if (!query) score = 1;
      else if (lowerBase === query) score = 100;
      else if (lowerBase.startsWith(query)) score = 50;
      else if (lower.startsWith(query)) score = 40;
      else if (lower.includes(query)) score = 10;
      return { path, score };
    })
    .filter((item) => item.score > 0)
    .sort((a, b) => b.score - a.score || a.path.length - b.path.length || a.path.localeCompare(b.path))
    .slice(0, 80);

  return scored.map(({ path }) => {
    const base = path.split('/').pop() ?? path;
    return {
      kind: 'file' as const,
      value: path,
      label: base,
      detail: path,
    };
  });
}

function FileBadge({ path }: { path: string }) {
  const ext = (path.split('.').pop() ?? '').toLowerCase();
  const labels: Record<string, string> = {
    ts: 'TS',
    tsx: 'TS',
    js: 'JS',
    jsx: 'JS',
    go: 'GO',
    md: 'MD',
    json: '{}',
    py: 'PY',
    rs: 'RS',
    css: '#',
    html: '<>',
    yml: 'YML',
    yaml: 'YML',
    sh: '$',
    toml: 'TOM',
  };
  const label = labels[ext] ?? (ext ? ext.slice(0, 3).toUpperCase() : '•');
  return <span className={`file-badge ext-${safeClass(ext)}`}>{label}</span>;
}

function CommandBadge() {
  return <span className="file-badge command-badge">&gt;_</span>;
}

function safeClass(value: string): string {
  return /^[a-z0-9]+$/.test(value) ? value : 'file';
}

function firstDifference(a: string, b: string): number {
  let index = 0;
  while (index < a.length && index < b.length && a[index] === b[index]) index++;
  return index;
}

const ANSI_RE = new RegExp(`${String.fromCharCode(27)}\\[[0-9;?]*[ -/]*[@-~]`, 'g');

function stripAnsi(value: string): string {
  return value.replace(ANSI_RE, '').replace(/\r\n/g, '\n').replace(/\r/g, '\n');
}

function pretty(value: unknown): string {
  if (typeof value === 'string') return value;
  try {
    return JSON.stringify(value, null, 2);
  } catch {
    return String(value);
  }
}

function kindLabel(kind?: string, title?: string): string {
  if (kind === 'execute' || (title && /^bash\b/i.test(title))) return 'Bash';
  switch (kind) {
    case 'read':
      return 'Read';
    case 'edit':
      return 'Edit';
    case 'search':
      return 'Search';
    case 'fetch':
      return 'Fetch';
    case 'think':
      return 'Agent';
    case 'delete':
      return 'Delete';
    case 'move':
      return 'Move';
    case 'switch_mode':
      return 'Mode';
    default:
      return 'Tool';
  }
}

function statusLabel(status: string): string {
  if (status === 'completed') return 'done';
  if (status === 'failed') return 'failed';
  if (status === 'cancelled') return 'cancelled';
  if (status === 'in_progress') return 'running';
  return 'running';
}

function modeLabel(mode: string): string {
  if (mode === 'plan') return 'Plan';
  if (mode === 'build') return 'Build';
  return 'Auto';
}

function planIcon(status?: string): string {
  if (status === 'completed') return '✓';
  if (status === 'in_progress') return '•';
  if (status === 'cancelled') return '×';
  return '○';
}

function formatNumber(value: number): string {
  if (value >= 1_000_000) return `${(value / 1_000_000).toFixed(1)}M`;
  if (value >= 1_000) return `${(value / 1_000).toFixed(1)}K`;
  return String(value);
}

function blobToBase64(blob: Blob): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onerror = () => reject(reader.error);
    reader.onload = () => {
      const result = String(reader.result ?? '');
      resolve(result.slice(result.indexOf(',') + 1));
    };
    reader.readAsDataURL(blob);
  });
}

function fileToBase64(file: File): Promise<string> {
  return blobToBase64(file);
}

const iconProps = {
  width: 16,
  height: 16,
  viewBox: '0 0 16 16',
  fill: 'none',
  stroke: 'currentColor',
  strokeWidth: 1.5,
  strokeLinecap: 'round' as const,
  strokeLinejoin: 'round' as const,
  'aria-hidden': true,
};

function PlusIcon() {
  return (
    <svg {...iconProps}>
      <path d="M8 3v10M3 8h10" />
    </svg>
  );
}

function MicIcon() {
  return (
    <svg {...iconProps}>
      <rect x="6" y="2" width="4" height="7" rx="2" />
      <path d="M4 8a4 4 0 0 0 8 0M8 12v2" />
    </svg>
  );
}

function AgentIcon() {
  return (
    <svg {...iconProps}>
      <path d="M5.5 4 2 8l3.5 4M10.5 4 14 8l-3.5 4" />
    </svg>
  );
}

function ModelIcon() {
  return (
    <svg {...iconProps}>
      <path d="M8 2.2 13 5v6l-5 2.8L3 11V5l5-2.8Z" />
      <path d="M3 5l5 2.8L13 5M8 7.8v7" />
    </svg>
  );
}

function SlidersIcon() {
  return (
    <svg {...iconProps}>
      <path d="M2.5 4.5h4M9.5 4.5h4M2.5 8h7M12.5 8h1M2.5 11.5h2M7.5 11.5h6" />
      <circle cx="8" cy="4.5" r="1.4" />
      <circle cx="11" cy="8" r="1.4" />
      <circle cx="5.5" cy="11.5" r="1.4" />
    </svg>
  );
}

function ArrowUpIcon() {
  return (
    <svg {...iconProps}>
      <path d="M8 13V3M4 7l4-4 4 4" />
    </svg>
  );
}

function ArrowDownIcon() {
  return (
    <svg {...iconProps}>
      <path d="M8 3v10M4 9l4 4 4-4" />
    </svg>
  );
}

function StopIcon() {
  return (
    <svg {...iconProps} fill="currentColor" stroke="none">
      <rect x="4" y="4" width="8" height="8" rx="1.5" />
    </svg>
  );
}
