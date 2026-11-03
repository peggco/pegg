import { useEffect, useMemo, useRef, useState } from 'react';
import { ArrowUp, ChevronDown, FileCode2, Image as ImageIcon, Mic, Paperclip, X } from 'lucide-react';
import { api } from '../lib/bridge';
import {
  adjacentChip,
  caretOffset,
  editorText,
  endRange,
  flatten,
  focusRange,
  insertChip,
  insertPlainText,
  isChipNode,
  promptText,
  rangeForOffsets,
  removeChip,
  setEditorText,
  syncChipName,
} from '../lib/editor';
import { compactNumber } from '../lib/format';
import { slashToken } from '../lib/slash';
import {
  attachFiles,
  cancelPrompt,
  changeMode,
  changeModel,
  errorText,
  installHubSkill,
  loadModels,
  sendPrompt,
  toggleRecording,
} from '../state/actions';
import { store, useStore } from '../state/store';
import { useT } from '../lib/i18n';
import type { Attachment, HubSkill, PromptAttachment } from '../types';

type Menu = 'mode' | 'model' | null;

const MAX_EDITOR_HEIGHT = 220;

function resizeEditor(el: HTMLElement) {
  el.style.height = 'auto';
  const height = el.scrollHeight;
  if (height > 0) el.style.height = `${Math.min(height, MAX_EDITOR_HEIGHT)}px`;
}


const DEFAULT_COMMANDS = [
  { name: 'init', description: 'Explore this project and write a starting brief' },
  { name: 'review', description: 'Review the current changes' },
  { name: 'test', description: 'Run tests and fix failures' },
  { name: 'explain', description: 'Explain the code or selection' },
  { name: 'fix', description: 'Find and fix a bug' },
  { name: 'plan', description: 'Plan the change before editing' },
  { name: 'commit', description: 'Write a commit message for staged changes' },
];

const MODES: { id: 'auto' | 'plan' | 'build'; labelKey: string; hintKey: string }[] = [
  { id: 'auto', labelKey: 'composer.auto', hintKey: 'composer.autoHint' },
  { id: 'plan', labelKey: 'composer.plan', hintKey: 'composer.planHint' },
  { id: 'build', labelKey: 'composer.build', hintKey: 'composer.buildHint' },
];

function toPromptAttachment(att: Attachment): PromptAttachment {
  return {
    name: att.name,
    mediaType: att.mediaType,
    data: att.data,
    text: att.text,
    path: att.path,
    size: att.size,
  };
}

export function Composer({ autoFocus = false, compact = false }: { autoFocus?: boolean; compact?: boolean }) {
  const running = useStore((s) => s.chat.running);
  const mode = useStore((s) => s.chat.mode);
  const provider = useStore((s) => s.chat.provider);
  const model = useStore((s) => s.chat.model);
  const commands = useStore((s) => s.chat.commands);
  const catalog = useStore((s) => s.commandsCatalog);
  const models = useStore((s) => s.models);
  const modelsLoading = useStore((s) => s.modelsLoading);
  const usage = useStore((s) => s.chat.usage);
  const sendOnEnter = useStore((s) => s.prefs.sendOnEnter);
  const focusKey = useStore((s) => s.composerFocus);
  const workspace = useStore((s) => s.workspace);

  const t = useT();
  const [text, setText] = useState('');
  const [caret, setCaret] = useState(0);
  const [chipCount, setChipCount] = useState(0);
  const [attachments, setAttachments] = useState<Attachment[]>([]);
  const [focused, setFocused] = useState(false);
  const [dragging, setDragging] = useState(false);
  const [menu, setMenu] = useState<Menu>(null);
  const [slashIndex, setSlashIndex] = useState(0);
  const [dismissed, setDismissed] = useState<{ start: number; query: string } | null>(null);
  const [remoteSkills, setRemoteSkills] = useState<HubSkill[]>([]);
  const [remoteLoading, setRemoteLoading] = useState(false);
  const [hubBusy, setHubBusy] = useState(false);
  const [mention, setMention] = useState<{ start: number; query: string } | null>(null);
  const [mentionFiles, setMentionFiles] = useState<string[]>([]);
  const [mentionIndex, setMentionIndex] = useState(0);
  const editorRef = useRef<HTMLDivElement>(null);
  const composingRef = useRef(false);
  const rootRef = useRef<HTMLDivElement>(null);
  const mentionRefs = useRef<Array<HTMLButtonElement | null>>([]);
  const slashRefs = useRef<Array<HTMLButtonElement | null>>([]);

  /**
   * Re-reads the contenteditable buffer. Chips serialize to a space here (the
   * prompt form is only produced in `submit`), so a caret sitting after a chip
   * can never look like an open `/query` token and the hub search stays closed.
   */
  const sync = () => {
    const el = editorRef.current;
    if (!el) {
      setText('');
      setCaret(0);
      setChipCount(0);
      return;
    }
    const value = editorText(el);
    const caretAt = caretOffset(el) ?? value.length;
    const chips = el.querySelectorAll('[data-chip]').length;
    setText(value);
    setCaret(caretAt);
    setChipCount(chips);
    el.dataset.empty = value.trim() === '' && chips === 0 ? 'true' : 'false';
    resizeEditor(el);
    const match = value.slice(0, caretAt).match(/(^|[\s(])@([^\s@]*)$/);
    if (match) {
      const next = { start: caretAt - match[2].length - 1, query: match[2] };
      setMention((prev) => (prev && prev.start === next.start && prev.query === next.query ? prev : next));
    } else {
      setMention(null);
    }
  };

  const token = useMemo(() => (caret < text.length ? null : slashToken(text)), [text, caret]);
  const tokenVisible =
    token !== null && !(dismissed && dismissed.start === token.start && dismissed.query === token.query);
  const slashQuery = tokenVisible && token ? token.query.toLowerCase() : null;

  const slashItems = useMemo(() => {
    if (slashQuery === null) return [];
    const merged = [...commands];
    for (const item of [...catalog, ...DEFAULT_COMMANDS]) {
      if (!merged.some((c) => c.name === item.name)) merged.push(item);
    }
    return merged.filter((c) => c.name.toLowerCase().includes(slashQuery));
  }, [commands, catalog, slashQuery]);

  const slashCount = slashQuery === null ? 0 : slashItems.length + remoteSkills.length;

  const searchRemote =
    slashQuery !== null && !mention && slashItems.length === 0 && slashQuery.trim().length >= 2;

  useEffect(() => {
    if (!searchRemote) {
      setRemoteSkills([]);
      setRemoteLoading(false);
      return;
    }
    const query = slashQuery!.trim();
    let alive = true;
    setRemoteLoading(true);
    const timer = window.setTimeout(() => {
      api
        .SearchHubSkills(query, 8)
        .then((items) => {
          if (alive) setRemoteSkills(items);
        })
        .catch(() => {
          if (alive) setRemoteSkills([]);
        })
        .finally(() => {
          if (alive) setRemoteLoading(false);
        });
    }, 300);
    return () => {
      alive = false;
      window.clearTimeout(timer);
    };
  }, [searchRemote, slashQuery]);

  useEffect(() => {
    if (autoFocus && editorRef.current) editorRef.current.focus();
  }, [autoFocus]);

  useEffect(() => {
    if (focusKey > 0 && editorRef.current) editorRef.current.focus();
  }, [focusKey]);

  useEffect(() => {
    const el = editorRef.current;
    if (!el) return;
    el.dataset.empty = editorText(el).trim() === '' ? 'true' : 'false';
    resizeEditor(el);
  }, []);

  useEffect(() => {
    setSlashIndex(0);
  }, [slashQuery]);

  useEffect(() => {
    if (slashIndex >= slashCount && slashCount > 0) setSlashIndex(0);
  }, [slashIndex, slashCount]);

  useEffect(() => {
    setMentionIndex(0);
  }, [mention?.query]);

  useEffect(() => {
    if (!mention) {
      setMentionFiles([]);
      return;
    }
    let cancelled = false;
    const timer = window.setTimeout(() => {
      api
        .SearchFiles(mention.query, 12)
        .then((files) => {
          if (!cancelled) setMentionFiles(files ?? []);
        })
        .catch(() => {
          if (!cancelled) setMentionFiles([]);
        });
    }, 70);
    return () => {
      cancelled = true;
      window.clearTimeout(timer);
    };
  }, [mention?.query]);

  useEffect(() => {
    mentionRefs.current[mentionIndex]?.scrollIntoView({ block: 'nearest' });
  }, [mentionIndex, mentionFiles.length]);

  useEffect(() => {
    slashRefs.current[slashIndex]?.scrollIntoView({ block: 'nearest' });
  }, [slashIndex, slashCount]);

  const commitChip = (name: string) => {
    const el = editorRef.current;
    if (!el || !token) return null;
    const start = token.start;
    const end = Math.min(text.length, start + 1 + token.query.length);
    const chip = insertChip(el, rangeForOffsets(el, start, end), name, t('composer.removeSkill'));
    setRemoteSkills([]);
    el.focus();
    sync();
    return chip;
  };

  const pickSlash = (index: number) => {
    if (index < slashItems.length) {
      const item = slashItems[index];
      if (item) commitChip(item.name);
      return;
    }
    const skill = remoteSkills[index - slashItems.length];
    if (!skill || hubBusy) return;
    const chip = commitChip(skill.name);
    if (!chip) return;
    setHubBusy(true);
    void (async () => {
      try {
        const res = await installHubSkill(skill.source, skill.skillId, 'global');
        const installed = (res.name ?? '').trim();
        if (installed && installed !== skill.name) syncChipName(chip, installed, t('composer.removeSkill'));
        sync();
      } catch (err) {
        chip.remove();
        editorRef.current?.focus();
        sync();
        store.toast('error', errorText(err));
      } finally {
        setHubBusy(false);
      }
    })();
  };

  const pickMention = (path: string) => {
    const el = editorRef.current;
    if (!el || !mention) return;
    const start = mention.start;
    const end = Math.min(text.length, start + 1 + mention.query.length);
    insertPlainText(el, `@${path} `, rangeForOffsets(el, start, end) ?? endRange(el));
    setMention(null);
    el.focus();
    sync();
    const full = workspace ? `${workspace.replace(/[\\/]+$/, '')}/${path}` : path;
    void api
      .ReadAttachment(full)
      .then((att) => {
        setAttachments((prev) =>
          prev.some((item) => item.path === att.path) ? prev : [...prev, att].slice(0, 8),
        );
      })
      .catch(() => undefined);
  };

  useEffect(() => {
    const onTranscript = (event: Event) => {
      const value = (event as CustomEvent<string>).detail ?? '';
      const el = editorRef.current;
      if (!el) return;
      const current = editorText(el);
      insertPlainText(el, current.trim() ? ` ${value}` : value, endRange(el));
      el.focus();
      sync();
    };
    const onAttachments = (event: Event) => {
      const items = (event as CustomEvent<Attachment[]>).detail ?? [];
      if (items.length) setAttachments((prev) => [...prev, ...items].slice(0, 8));
    };
    const onComposerText = (event: Event) => {
      const value = (event as CustomEvent<string>).detail ?? '';
      const el = editorRef.current;
      if (!el) return;
      setEditorText(el, value);
      el.focus();
      focusRange(endRange(el));
      sync();
    };
    window.addEventListener('pegg:transcript', onTranscript);
    window.addEventListener('pegg:attachments', onAttachments);
    window.addEventListener('pegg:composer-text', onComposerText);
    return () => {
      window.removeEventListener('pegg:transcript', onTranscript);
      window.removeEventListener('pegg:attachments', onAttachments);
      window.removeEventListener('pegg:composer-text', onComposerText);
    };
  }, []);

  useEffect(() => {
    if (!menu) return;
    const close = (event: MouseEvent) => {
      if (!rootRef.current?.contains(event.target as Node)) setMenu(null);
    };
    window.addEventListener('mousedown', close);
    return () => window.removeEventListener('mousedown', close);
  }, [menu]);

  const canSend = Boolean(text.trim() || chipCount > 0 || attachments.length);

  const submit = () => {
    if (running) {
      void cancelPrompt();
      return;
    }
    const el = editorRef.current;
    if (!el || hubBusy) return;
    const value = promptText(el).trim();
    if (!value && !attachments.length) return;
    if (!workspace) {
      store.toast('warn', 'Choose a project folder first');
      return;
    }
    const payload = attachments.map(toPromptAttachment);
    setEditorText(el, '');
    setAttachments([]);
    setMention(null);
    setMenu(null);
    sync();
    void sendPrompt({ text: value, attachments: payload });
  };

  const pickFiles = async () => {
    const files = await attachFiles();
    if (files.length) setAttachments((prev) => [...prev, ...files].slice(0, 8));
  };

  const onKeyDown = (event: React.KeyboardEvent<HTMLDivElement>) => {
    const native = event.nativeEvent as KeyboardEvent;
    if (native.isComposing || native.keyCode === 229) return;
    if (event.key === 'Escape') {
      setMenu(null);
      setMention(null);
      setRemoteSkills([]);
      if (token) setDismissed(token);
      return;
    }
    if (mention && mentionFiles.length && (event.key === 'ArrowDown' || event.key === 'ArrowUp')) {
      event.preventDefault();
      setMentionIndex((i) => {
        const next = event.key === 'ArrowDown' ? i + 1 : i - 1;
        return (next + mentionFiles.length) % mentionFiles.length;
      });
      return;
    }
    if (mention && mentionFiles.length && event.key === 'Enter' && !event.shiftKey) {
      event.preventDefault();
      pickMention(mentionFiles[mentionIndex]);
      return;
    }
    if (slashCount > 0 && (event.key === 'ArrowDown' || event.key === 'ArrowUp')) {
      event.preventDefault();
      setSlashIndex((i) => {
        const next = event.key === 'ArrowDown' ? i + 1 : i - 1;
        return (next + slashCount) % slashCount;
      });
      return;
    }
    if (
      event.key === 'Enter' &&
      !event.shiftKey &&
      !event.metaKey &&
      !event.ctrlKey &&
      slashQuery !== null &&
      remoteLoading &&
      slashCount === 0
    ) {
      event.preventDefault();
      return;
    }
    if (slashCount > 0 && event.key === 'Enter' && !event.shiftKey && !event.metaKey && !event.ctrlKey) {
      event.preventDefault();
      pickSlash(slashIndex);
      return;
    }
    if (event.key === 'Enter' && event.shiftKey) {
      event.preventDefault();
      const el = editorRef.current;
      if (el) insertPlainText(el, '\n');
      sync();
      return;
    }
    if ((event.key === 'Backspace' || event.key === 'Delete') && !event.metaKey && !event.ctrlKey) {
      const el = editorRef.current;
      const chip = el ? adjacentChip(el, event.key === 'Backspace' ? 'before' : 'after') : null;
      if (el && chip) {
        event.preventDefault();
        removeChip(chip);
        sync();
        return;
      }
    }
    const send = event.key === 'Enter' && !event.shiftKey && (event.metaKey || event.ctrlKey || sendOnEnter);
    if (send) {
      event.preventDefault();
      submit();
    }
  };

  const onEditorInput = () => {
    const el = editorRef.current;
    if (el && !composingRef.current) flatten(el);
    setDismissed(null);
    sync();
  };

  const onEditorPaste = (event: React.ClipboardEvent<HTMLDivElement>) => {
    event.preventDefault();
    const el = editorRef.current;
    if (!el) return;
    const value = event.clipboardData.getData('text/plain');
    insertPlainText(el, value.replace(/\r\n?/g, '\n'));
    sync();
  };

  const onEditorMouseDown = (event: React.MouseEvent<HTMLDivElement>) => {
    const target = event.target as HTMLElement | null;
    const remove = target?.closest('[data-chip-remove]');
    if (!remove) return;
    const chip = remove.closest('[data-chip]');
    if (!isChipNode(chip)) return;
    event.preventDefault();
    removeChip(chip);
    editorRef.current?.focus();
    sync();
  };

  const onEditorBeforeInput = (event: React.FormEvent<HTMLDivElement>) => {
    const type = (event.nativeEvent as InputEvent).inputType;
    if (type === 'formatBold' || type === 'formatItalic' || type === 'formatUnderline' || type === 'insertFromDrop') {
      event.preventDefault();
    }
  };

  const currentModelLabel = model || t('composer.model');
  const modeLabel = t(MODES.find((m) => m.id === mode)?.labelKey ?? 'composer.auto');

  return (
    <div
      ref={rootRef}
      className={`composer${focused ? ' focused' : ''}${dragging ? ' dragging' : ''}`}
      onDragOver={(event) => {
        event.preventDefault();
        setDragging(true);
      }}
      onDragLeave={(event) => {
        if (event.currentTarget === event.target) setDragging(false);
      }}
      onDrop={(event) => {
        event.preventDefault();
        setDragging(false);
      }}
    >
      {attachments.length ? (
        <div className="composer-attachments">
          {attachments.map((att, index) => (
            <span className="attachment" key={`${att.name}-${index}`} title={att.path ?? att.name}>
              {att.mediaType.startsWith('image/') && att.data ? (
                <img src={`data:${att.mediaType};base64,${att.data}`} alt="" />
              ) : (
                <ImageIcon size={13} />
              )}
              <span className="name">{att.name}</span>
              <button
                aria-label={`Remove ${att.name}`}
                onClick={() => setAttachments((prev) => prev.filter((_, i) => i !== index))}
              >
                <X size={11} />
              </button>
            </span>
          ))}
        </div>
      ) : null}

      <div
        ref={editorRef}
        className="composer-input"
        role="textbox"
        aria-multiline="true"
        aria-label={compact ? t('composer.reply') : t('composer.placeholder')}
        contentEditable
        suppressContentEditableWarning
        spellCheck
        data-placeholder={compact ? t('composer.reply') : t('composer.placeholder')}
        data-empty="true"
        onInput={onEditorInput}
        onBeforeInput={onEditorBeforeInput}
        onKeyDown={onKeyDown}
        onKeyUp={(event) => {
          if (['ArrowDown', 'ArrowUp', 'Escape', 'Tab'].includes(event.key)) return;
          sync();
        }}
        onPaste={onEditorPaste}
        onClick={() => sync()}
        onMouseDown={onEditorMouseDown}
        onMouseUp={() => sync()}
        onFocus={() => setFocused(true)}
        onBlur={() => setFocused(false)}
        onCompositionStart={() => {
          composingRef.current = true;
        }}
        onCompositionEnd={() => {
          composingRef.current = false;
          const el = editorRef.current;
          if (el) flatten(el);
          sync();
        }}
      />

      {mention && mentionFiles.length ? (
        <div
          className="slash-menu"
          role="listbox"
          aria-label="Files"
          onWheel={(event) => event.stopPropagation()}
        >
          <div className="popover-label">{t('welcome.files')}</div>
          {mentionFiles.map((path, index) => (
            <button
              key={path}
              ref={(el) => {
                mentionRefs.current[index] = el;
              }}
              className={`slash-item${index === mentionIndex ? ' active' : ''}`}
              onMouseDown={(event) => {
                event.preventDefault();
                pickMention(path);
              }}
            >
              <FileCode2 size={13} style={{ color: 'var(--pg-muted)', flex: 'none' }} />
              <span className="desc" style={{ flex: 1 }}>
                {path}
              </span>
            </button>
          ))}
        </div>
      ) : null}

      {slashQuery !== null && (slashItems.length > 0 || remoteSkills.length > 0 || remoteLoading) && !mention ? (
        <div className="slash-menu" role="listbox" aria-label="Slash commands">
          {slashItems.map((item, index) => (
            <button
              key={item.name}
              ref={(el) => {
                slashRefs.current[index] = el;
              }}
              className={`slash-item command${index === slashIndex ? ' active' : ''}`}
              onMouseDown={(event) => {
                event.preventDefault();
                pickSlash(index);
              }}
            >
              <span className="cmd">/{item.name}</span>
              <span className="desc">{item.description}</span>
            </button>
          ))}
          {!slashItems.length && remoteLoading && !remoteSkills.length ? (
            <div className="slash-loading">
              <span className="spinner" /> {t('common.loading')}
            </div>
          ) : null}
          {remoteSkills.map((item, index) => {
            const itemIndex = slashItems.length + index;
            return (
              <button
                key={`${item.source}-${item.skillId}`}
                ref={(el) => {
                  slashRefs.current[itemIndex] = el;
                }}
                className={`slash-item command hub${itemIndex === slashIndex ? ' active' : ''}`}
                onMouseDown={(event) => {
                  event.preventDefault();
                  pickSlash(itemIndex);
                }}
              >
                <span className="cmd">/{item.name}</span>
                <span className="desc">
                  {item.source}
                  <span className="hub-badge">{t('skills.hub')}</span>
                  <span className="hub-hint">{t('skills.fromHub')}</span>
                </span>
              </button>
            );
          })}
        </div>
      ) : null}

      <div className="composer-bar">
        <div className="composer-tools">
          <button className="tool-btn" title={t('composer.attach')} aria-label={t('composer.attach')} onClick={() => void pickFiles()}>
            <Paperclip size={15} />
          </button>

          <div style={{ position: 'relative' }}>
            <button
              className={`pill-btn${menu === 'mode' ? ' active' : ''}`}
              onClick={() => setMenu(menu === 'mode' ? null : 'mode')}
              title={t('composer.mode')}
            >
              <span>{modeLabel}</span>
              <ChevronDown size={12} />
            </button>
            {menu === 'mode' ? (
              <div className="popover" style={{ bottom: 'calc(100% + 6px)', left: 0 }}>
                <div className="popover-label">{t('composer.mode')}</div>
                {MODES.map((m) => (
                  <button
                    key={m.id}
                    className={`popover-item${mode === m.id ? ' active' : ''}`}
                    onClick={() => {
                      setMenu(null);
                      void changeMode(m.id);
                    }}
                  >
                    <span>{t(m.labelKey)}</span>
                    <span className="model-id" style={{ marginLeft: 'auto' }}>
                      {t(m.hintKey)}
                    </span>
                  </button>
                ))}
              </div>
            ) : null}
          </div>
        </div>

        <div className="composer-spacer" />

        {usage.used > 0 ? (
          <div
            className="usage-meter"
            title={`${compactNumber(usage.used)} tokens${usage.cost ? ` · $${usage.cost.toFixed(4)}` : ''}`}
          >
            <span
              className="usage-ring"
              style={{
                ['--pct' as string]: `${usage.size ? Math.min(100, (usage.used / usage.size) * 100) : 0}%`,
              }}
            />
            {compactNumber(usage.used)}
          </div>
        ) : null}

        <div style={{ position: 'relative' }}>
          <button
            className={`pill-btn${menu === 'model' ? ' active' : ''}`}
            onClick={() => {
              setMenu(menu === 'model' ? null : 'model');
              if (!models.length) void loadModels();
            }}
            title={provider ? `${provider} / ${model}` : t('composer.chooseModel')}
          >
            <span>{currentModelLabel}</span>
            <ChevronDown size={12} />
          </button>
          {menu === 'model' ? (
            <div className="popover" style={{ bottom: 'calc(100% + 6px)', right: 0, minWidth: 280 }}>
              <div className="popover-label">{modelsLoading ? '…' : t('composer.model')}</div>
              {!modelsLoading && !models.length ? (
                <div style={{ padding: '8px 9px', color: 'var(--pg-muted)', fontSize: 12 }}>
                  {t('composer.noModels')}
                </div>
              ) : null}
              {models.map((entry) => {
                const id = entry.model.id;
                const label = entry.model.name || id;
                const active = provider === entry.provider && model === id;
                return (
                  <button
                    key={`${entry.provider}-${id}`}
                    className={`popover-item${active ? ' active' : ''}`}
                    onClick={() => {
                      setMenu(null);
                      void changeModel(entry.provider, id);
                    }}
                  >
                    <span>{label}</span>
                    <span className="model-id">{entry.provider}</span>
                    {active ? <span className="check">✓</span> : null}
                  </button>
                );
              })}
            </div>
          ) : null}
        </div>

        <button
          className="tool-btn"
          title="Voice input"
          aria-label="Voice input"
          onClick={() => void toggleRecording()}
        >
          <Mic size={16} />
        </button>

        <button
          className={`send-btn${running ? ' stop' : ''}`}
          disabled={!running && !canSend}
          aria-label={running ? t('composer.stop') : t('composer.send')}
          onClick={submit}
          title={running ? t('composer.stop') : t('composer.send')}
        >
          {running ? (
            <span
              style={{
                width: 9,
                height: 9,
                background: 'currentColor',
                borderRadius: 2,
                display: 'block',
              }}
            />
          ) : (
            <ArrowUp size={17} />
          )}
        </button>
      </div>
    </div>
  );
}
