import { useEffect, useRef, useState } from 'react';
import { useVirtualizer } from '@tanstack/react-virtual';
import { ArrowLeft, ChevronRight, Copy, GitFork, Info, RotateCcw, TriangleAlert } from 'lucide-react';
import { renderMarkdown } from '../lib/markdown';
import { closeSubagent, forkMessage, loadOlderMessages, openSubagent, revertMessage, setStickToBottom } from '../state/actions';
import { store, useStore } from '../state/store';
import { useT } from '../lib/i18n';
import type { ChatItem } from '../state/chat';
import { ToolCard } from './ToolViews';
import { AskCard, PermissionCard, PlanCard, SubagentCard } from './Cards';

export function Thread() {
  const mainItems = useStore((s) => s.chat.items);
  const plan = useStore((s) => s.chat.plan);
  const running = useStore((s) => s.chat.running);
  const loading = useStore((s) => s.chat.loading);
  const historyLoading = useStore((s) => s.chat.historyLoading);
  const historyHasMore = useStore((s) => s.chat.historyHasMore);
  const acpId = useStore((s) => s.chat.acpId);
  const permissions = useStore((s) => s.chat.permissions);
  const asks = useStore((s) => s.chat.asks);
  const subagents = useStore((s) => s.chat.subagents);
  const activeAgent = useStore((s) => s.subagentView);
  const activeRun = activeAgent ? subagents[activeAgent] : undefined;
  const items = activeRun?.items ?? mainItems;
  const scrollerRef = useRef<HTMLDivElement>(null);
  const stickRef = useRef(true);
  const pendingScrollRef = useRef<{ height: number; top: number } | null>(null);
  const [copied, setCopied] = useState<string | null>(null);
  const t = useT();

  const virtualizer = useVirtualizer({
    count: items.length,
    getScrollElement: () => scrollerRef.current,
    estimateSize: () => 96,
    overscan: 8,
    getItemKey: (index) => items[index]?.id ?? index,
  });
  const totalSize = virtualizer.getTotalSize();

  useEffect(() => {
    stickRef.current = true;
    pendingScrollRef.current = null;
    setStickToBottom(true);
    const frame = requestAnimationFrame(() => {
      const el = scrollerRef.current;
      if (el) el.scrollTop = el.scrollHeight;
    });
    return () => cancelAnimationFrame(frame);
  }, [acpId]);

  useEffect(() => {
    const el = scrollerRef.current;
    if (!el) return;
    if (stickRef.current) el.scrollTop = el.scrollHeight;
  }, [items, plan, permissions, asks, running, totalSize]);

  useEffect(() => {
    const pending = pendingScrollRef.current;
    const el = scrollerRef.current;
    if (!pending || !el || historyLoading) return;
    if (el.scrollHeight <= pending.height) {
      pendingScrollRef.current = null;
      return;
    }
    el.scrollTop = el.scrollHeight - pending.height + pending.top;
    pendingScrollRef.current = null;
  }, [items, totalSize, historyLoading]);

  const copy = async (text: string, key: string) => {
    try {
      await navigator.clipboard.writeText(text);
      setCopied(key);
      window.setTimeout(() => setCopied((v) => (v === key ? null : v)), 1400);
    } catch {
      store.toast('warn', 'Clipboard is unavailable');
    }
  };

  const onThreadClick = (event: React.MouseEvent) => {
    const target = event.target as HTMLElement;
    const button = target.closest('[data-copy-code]') as HTMLElement | null;
    if (!button) return;
    const block = button.closest('.pg-code');
    const code = block?.querySelector('pre code')?.textContent ?? '';
    void copy(code, 'code');
    button.textContent = 'Copied';
    window.setTimeout(() => {
      button.textContent = 'Copy';
    }, 1400);
  };

  const lastAssistantId = [...items].reverse().find((item) => item.kind === 'assistant')?.id;
  const userIndices = new Map<string, number>();
  let userIndex = 0;
  for (const item of items) {
    if (item.kind === 'user') userIndices.set(item.id, userIndex++);
  }

  if (activeAgent && activeRun) {
    return (
      <div className="subagent-view">
        <div className="subagent-view-head">
          <button className="subagent-back" onClick={closeSubagent}>
            <ArrowLeft size={15} />
            {t('subagent.back')}
          </button>
          <span className="subagent-view-title">{activeRun.agentName || 'Subagent'}</span>
          <span className="badge">{activeRun.status}</span>
        </div>
        <div
          className="thread-wrap"
          ref={scrollerRef}
          onScroll={(event) => {
            const el = event.currentTarget;
            stickRef.current = el.scrollHeight - el.scrollTop - el.clientHeight < 90;
          }}
          onClick={onThreadClick}
        >
          <div className="thread">
            {activeRun.task ? <div className="subagent-modal-task">{activeRun.task}</div> : null}
            {items.map((entry) => (
              <div className="thread-item" key={entry.id}>
                <Item
                  item={entry}
                  streaming={false}
                  copied={copied}
                  onCopy={copy}
                  onOpenSubagent={openSubagent}
                  userMessageIndex={userIndices.get(entry.id)}
                />
              </div>
            ))}
            {!items.length ? <div className="empty-state">{t('subagent.empty')}</div> : null}
          </div>
        </div>
      </div>
    );
  }

  return (
    <>
    {loading ? (
      <div className="thread-loading" style={{ padding: '6px 0' }}>
        <span className="spinner" /> {t('thread.restoring')}
      </div>
    ) : null}
    {historyLoading ? (
      <div className="thread-loading" style={{ padding: '6px 0' }}>
        <span className="spinner" /> {t('thread.loadingHistory')}
      </div>
    ) : null}
    <div
      className="thread-wrap"
      ref={scrollerRef}
      onScroll={(event) => {
        const el = event.currentTarget;
        const stick = el.scrollHeight - el.scrollTop - el.clientHeight < 90;
        stickRef.current = stick;
        setStickToBottom(stick);
        if (el.scrollTop < 80 && historyHasMore && !historyLoading && !pendingScrollRef.current) {
          pendingScrollRef.current = { height: el.scrollHeight, top: el.scrollTop };
          void loadOlderMessages();
        }
      }}
      onClick={onThreadClick}
    >
      <div className="thread">
        {items.length ? (
          <div style={{ height: totalSize, position: 'relative', width: '100%' }}>
            {virtualizer.getVirtualItems().map((virtualItem) => {
              const item = items[virtualItem.index];
              return (
                <div
                  className="thread-item"
                  key={item.id}
                  data-index={virtualItem.index}
                  ref={virtualizer.measureElement}
                  style={{
                    position: 'absolute',
                    top: 0,
                    left: 0,
                    width: '100%',
                    transform: `translateY(${virtualItem.start}px)`,
                  }}
                >
                  <Item
                    item={item}
                    streaming={running && item.id === lastAssistantId}
                    copied={copied}
                    onCopy={copy}
                    onOpenSubagent={openSubagent}
                    userMessageIndex={userIndices.get(item.id)}
                  />
                </div>
              );
            })}
          </div>
        ) : null}

        {plan.length ? <PlanCard entries={plan} /> : null}

        {items.length === 0 && !loading ? (
          <div className="notice" style={{ justifyContent: 'center', padding: '18px 14px' }}>
            <Info size={14} />
            {t('thread.empty')}
          </div>
        ) : null}

        {permissions.map((permission) => (
          <PermissionCard key={permission.requestId} request={permission} />
        ))}

        {asks.map((ask) => (
          <AskCard key={ask.requestId} request={ask} />
        ))}
      </div>
    </div>
    </>
  );
}

function Item({
  item,
  streaming,
  copied,
  onCopy,
  onOpenSubagent,
  userMessageIndex,
}: {
  item: ChatItem;
  streaming: boolean;
  copied: string | null;
  onCopy: (text: string, key: string) => void;
  onOpenSubagent: (agentId: string) => void;
  userMessageIndex?: number;
}) {
  switch (item.kind) {
    case 'user':
      return (
        <UserMessage
          item={item}
          copied={copied === item.id}
          onCopy={() => onCopy(item.text, item.id)}
          userMessageIndex={userMessageIndex}
        />
      );
    case 'assistant':
      return (
        <AssistantMessage
          item={item}
          streaming={streaming}
          copied={copied === item.id}
          onCopy={() => onCopy(item.text, item.id)}
        />
      );
    case 'reasoning':
      return <Reasoning item={item} />;
    case 'tool':
      return <ToolCard tool={item.tool} />;
    case 'subagent':
      return <SubagentCard item={item} onOpen={() => onOpenSubagent(item.agentId)} />;
    case 'notice':
      return (
        <div className={`notice${item.level === 'info' ? '' : ` ${item.level}`}`}>
          {item.level === 'error' ? <TriangleAlert size={14} /> : <Info size={14} />}
          <span>{item.text}</span>
        </div>
      );
    case 'error':
      return (
        <div className="notice error">
          <TriangleAlert size={14} />
          <span>{item.text}</span>
        </div>
      );
    default:
      return null;
  }
}

function UserMessage({
  item,
  copied,
  onCopy,
  userMessageIndex,
}: {
  item: Extract<ChatItem, { kind: 'user' }>;
  copied: boolean;
  onCopy: () => void;
  userMessageIndex?: number;
}) {
  const t = useT();
  const ref = item.messageId
    ? { messageId: item.messageId }
    : userMessageIndex !== undefined
      ? { userMessageIndex }
      : null;
  return (
    <div className="msg msg-user">
      <div className="user-bubble">{item.text}</div>
      <div className="msg-actions">
        <button className="msg-action" onClick={onCopy} title="Copy message">
          <Copy size={12} />
          {copied ? t('common.copied') : t('common.copy')}
        </button>
        {ref ? (
          <>
            <button className="msg-action" onClick={() => void forkMessage(ref)} title="Fork before this message">
              <GitFork size={12} />
              {t('common.fork')}
            </button>
            <button className="msg-action" onClick={() => void revertMessage({ ...ref, text: item.text })} title="Revert to this message">
              <RotateCcw size={12} />
              {t('common.revert')}
            </button>
          </>
        ) : null}
      </div>
    </div>
  );
}

function AssistantMessage({
  item,
  streaming,
  copied,
  onCopy,
}: {
  item: Extract<ChatItem, { kind: 'assistant' }>;
  streaming: boolean;
  copied: boolean;
  onCopy: () => void;
}) {
  const t = useT();
  const html = renderMarkdown(item.text);
  return (
    <div className="msg msg-assistant">
      {item.agentName ? (
        <div className="assistant-meta">
          <span className="agent-dot" />
          <span className="agent-tag">@{item.agentName}</span>
        </div>
      ) : null}
      <div
        className={`assistant-text md${streaming ? ' is-streaming' : ''}`}
        dangerouslySetInnerHTML={{ __html: html }}
      />
      <div className="msg-actions">
        <button className="msg-action" onClick={onCopy} title="Copy message">
          <Copy size={12} />
          {copied ? t('common.copied') : t('common.copy')}
        </button>
      </div>
    </div>
  );
}

function Reasoning({ item }: { item: Extract<ChatItem, { kind: 'reasoning' }> }) {
  const t = useT();
  const [open, setOpen] = useState(false);
  const words = item.text.trim().split(/\s+/).length;
  return (
    <div className="reasoning">
      <button className="reasoning-toggle" onClick={() => setOpen((v) => !v)}>
        <ChevronRight
          size={12}
          style={{ transform: open ? 'rotate(90deg)' : undefined, transition: 'transform 120ms' }}
        />
        {t('common.thought')}{words > 1 ? ` · ${words}` : ''}
      </button>
      {open ? <div className="reasoning-body">{item.text}</div> : null}
    </div>
  );
}
