import { useEffect, useMemo, useRef, useState } from 'react';
import { useVirtualizer } from '@tanstack/react-virtual';
import {
  ArrowDown,
  ArrowLeft,
  Brain,
  ChevronRight,
  Copy,
  GitFork,
  Info,
  RotateCcw,
  TriangleAlert,
  Wrench,
} from 'lucide-react';
import { renderMarkdown } from '../lib/markdown';
import { closeSubagent, forkMessage, loadOlderMessages, openSubagent, revertMessage, setStickToBottom } from '../state/actions';
import { store, useStore } from '../state/store';
import { useT } from '../lib/i18n';
import type { ChatItem } from '../state/chat';
import { activityCounts, buildHistoryRows, formatActivityTime, type ActivityCounts, type HistoryRow } from '../lib/activity';
import { ToolCard, ToolIcon, bashCommand } from './ToolViews';
import { AskCard, PermissionCard, PlanCard, SubagentCard } from './Cards';
import { TaskCard } from './TaskCard';

export function Thread() {
  const mainItems = useStore((s) => s.chat.items);
  const plan = useStore((s) => s.chat.plan);
  const running = useStore((s) => s.chat.running);
  const loading = useStore((s) => s.chat.loading);
  const historyLoading = useStore((s) => s.chat.historyLoading);
  const historyHasMore = useStore((s) => s.chat.historyHasMore);
  const historyOldestSeq = useStore((s) => s.chat.historyOldestSeq);
  const acpId = useStore((s) => s.chat.acpId);
  const permissions = useStore((s) => s.chat.permissions);
  const asks = useStore((s) => s.chat.asks);
  const subagents = useStore((s) => s.chat.subagents);
  const activeAgent = useStore((s) => s.subagentView);
  const activeRun = activeAgent ? subagents[activeAgent] : undefined;
  const items = activeRun?.items ?? mainItems;
  const historyMode = useStore((s) => s.prefs.chatHistoryMode);
  const rows = useMemo(() => buildHistoryRows(items, historyMode), [items, historyMode]);
  const [openGroups, setOpenGroups] = useState<Set<string>>(() => new Set());
  const [openRows, setOpenRows] = useState<Set<string>>(() => new Set());
  const scrollerRef = useRef<HTMLDivElement>(null);
  const stickRef = useRef(true);
  const pendingScrollRef = useRef<{ height: number; top: number } | null>(null);
  const autoLoadSeqRef = useRef<number | null>(null);
  const [copied, setCopied] = useState<string | null>(null);
  const [showJump, setShowJump] = useState(false);
  const t = useT();

  const toggleGroup = (id: string) => setOpenGroups((current) => toggleId(current, id));
  const toggleRow = (id: string) => setOpenRows((current) => toggleId(current, id));

  const virtualizer = useVirtualizer({
    count: rows.length,
    getScrollElement: () => scrollerRef.current,
    estimateSize: () => 96,
    overscan: 8,
    getItemKey: (index) => rows[index]?.key ?? index,
  });
  const totalSize = virtualizer.getTotalSize();

  useEffect(() => {
    stickRef.current = true;
    pendingScrollRef.current = null;
    setShowJump(false);
    setOpenGroups(new Set());
    setOpenRows(new Set());
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

  useEffect(() => {
    if (activeAgent || !historyHasMore || historyLoading) return;
    const el = scrollerRef.current;
    if (!el) return;
    if (el.scrollHeight > el.clientHeight + 200) return;
    if (autoLoadSeqRef.current === historyOldestSeq) return;
    autoLoadSeqRef.current = historyOldestSeq;
    void loadOlderMessages();
  }, [activeAgent, rows, historyHasMore, historyLoading, historyOldestSeq, totalSize]);

  const jumpToBottom = () => {
    const el = scrollerRef.current;
    if (!el) return;
    stickRef.current = true;
    pendingScrollRef.current = null;
    setStickToBottom(true);
    setShowJump(false);
    el.scrollTo({ top: el.scrollHeight, behavior: 'smooth' });
  };

  const jumpButton = showJump ? (
    <button
      className="scroll-bottom-btn"
      title={t('thread.scrollToBottom')}
      aria-label={t('thread.scrollToBottom')}
      onClick={jumpToBottom}
    >
      <ArrowDown size={16} />
    </button>
  ) : null;

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
        <div className="thread-area">
          <div
            className="thread-wrap"
            ref={scrollerRef}
            onScroll={(event) => {
              const el = event.currentTarget;
              const stick = el.scrollHeight - el.scrollTop - el.clientHeight < 90;
              stickRef.current = stick;
              setShowJump(!stick);
            }}
            onClick={onThreadClick}
          >
            <div className="thread">
              {activeRun.task ? <div className="subagent-modal-task">{activeRun.task}</div> : null}
              {rows.map((row) => (
                <div className="thread-item" key={row.key}>
                  {row.kind === 'activity' ? (
                    <ActivityGroup
                      group={row}
                      open={openGroups.has(row.id)}
                      onToggle={() => toggleGroup(row.id)}
                      openRows={openRows}
                      onToggleRow={toggleRow}
                    />
                  ) : (
                    <Item
                      item={row.item}
                      streaming={false}
                      copied={copied}
                      onCopy={copy}
                      onOpenSubagent={openSubagent}
                      userMessageIndex={userIndices.get(row.item.id)}
                    />
                  )}
                </div>
              ))}
              {!items.length ? <div className="empty-state">{t('subagent.empty')}</div> : null}
            </div>
          </div>
          {jumpButton}
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
    <div className="thread-area">
      <div
        className="thread-wrap"
        ref={scrollerRef}
        onScroll={(event) => {
          const el = event.currentTarget;
          const stick = el.scrollHeight - el.scrollTop - el.clientHeight < 90;
          stickRef.current = stick;
          setShowJump(!stick);
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
                const row = rows[virtualItem.index];
                if (!row) return null;
                return (
                  <div
                    className="thread-item"
                    key={row.key}
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
                    {row.kind === 'activity' ? (
                      <ActivityGroup
                        group={row}
                        open={openGroups.has(row.id)}
                        onToggle={() => toggleGroup(row.id)}
                        openRows={openRows}
                        onToggleRow={toggleRow}
                      />
                    ) : (
                      <Item
                        item={row.item}
                        streaming={running && row.item.id === lastAssistantId}
                        copied={copied}
                        onCopy={copy}
                        onOpenSubagent={openSubagent}
                        userMessageIndex={userIndices.get(row.item.id)}
                      />
                    )}
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
      {jumpButton}
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
      if (item.tool.task) {
        const task = item.tool.task;
        return (
          <TaskCard
            tool={item.tool}
            onOpen={task.agentId ? () => onOpenSubagent(task.agentId as string) : undefined}
          />
        );
      }
      return <ToolCard tool={item.tool} />;
    case 'subagent':
      if (item.linkedTaskId) return null;
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
      {item.text || !item.attachments?.length ? <div className="user-bubble">{item.text}</div> : null}
      {item.attachments?.length ? (
        <div className="user-attachments">
          {item.attachments.map((attachment, index) => (
            <span
              className="user-attachment"
              key={`${attachment.name}-${index}`}
              title={attachment.path ?? attachment.name}
            >
              <span className="user-attachment-icon" aria-hidden="true">
                {attachmentIcon(attachment.mediaType, attachment.type)}
              </span>
              <span className="user-attachment-name">{attachment.name}</span>
            </span>
          ))}
        </div>
      ) : null}
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

function attachmentIcon(mediaType?: string, type?: string): string {
  const mt = (mediaType ?? '').toLowerCase();
  if (type === 'image' || mt.startsWith('image/')) return '🖼';
  if (type === 'audio' || mt.startsWith('audio/')) return '🎵';
  return '📎';
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

function toggleId(current: Set<string>, id: string): Set<string> {
  const next = new Set(current);
  if (next.has(id)) next.delete(id);
  else next.add(id);
  return next;
}

function activitySummary(counts: ActivityCounts, t: (key: string, vars?: Record<string, string | number>) => string): string {
  const plural = (count: number, one: string, many: string) => t(count === 1 ? one : many, { count });
  const parts: string[] = [];
  if (counts.commands) parts.push(plural(counts.commands, 'chat.ranCommand', 'chat.ranCommands'));
  if (counts.reads) parts.push(plural(counts.reads, 'chat.readFile', 'chat.readFiles'));
  if (counts.edits) parts.push(plural(counts.edits, 'chat.editedFile', 'chat.editedFiles'));
  if (counts.searches) parts.push(plural(counts.searches, 'chat.ranSearch', 'chat.ranSearches'));
  if (counts.fetches) parts.push(plural(counts.fetches, 'chat.fetchedPage', 'chat.fetchedPages'));
  if (!parts.length) {
    return counts.thoughts ? t('chat.activityThoughts', { count: counts.thoughts }) : t('common.thought');
  }
  return parts.join(' · ');
}

function ActivityGroup({
  group,
  open,
  onToggle,
  openRows,
  onToggleRow,
}: {
  group: Extract<HistoryRow, { kind: 'activity' }>;
  open: boolean;
  onToggle: () => void;
  openRows: Set<string>;
  onToggleRow: (id: string) => void;
}) {
  const t = useT();
  const language = useStore((s) => s.language);
  const counts = activityCounts(group.items);
  return (
    <div className={`activity${open ? ' open' : ''}`}>
      <button className="activity-head" onClick={onToggle} aria-expanded={open}>
        <span className="activity-head-icon">
          <Wrench size={13} />
        </span>
        <span className="activity-summary">{activitySummary(counts, t)}</span>
        <span className="activity-time">{formatActivityTime(group.createdAt, language)}</span>
        <ChevronRight size={13} className="activity-chevron" />
      </button>
      {open ? (
        <div className="activity-rows">
          {group.items.map((item) =>
            item.kind === 'reasoning' ? (
              <ActivityReasoning
                key={item.id}
                item={item}
                open={openRows.has(item.id)}
                onToggle={() => onToggleRow(item.id)}
                language={language}
              />
            ) : item.kind === 'tool' ? (
              <ActivityTool
                key={item.id}
                item={item}
                open={openRows.has(item.id)}
                onToggle={() => onToggleRow(item.id)}
                language={language}
              />
            ) : null,
          )}
        </div>
      ) : null}
    </div>
  );
}

function ActivityReasoning({
  item,
  open,
  onToggle,
  language,
}: {
  item: Extract<ChatItem, { kind: 'reasoning' }>;
  open: boolean;
  onToggle: () => void;
  language: string;
}) {
  return (
    <>
      <button className="activity-row reasoning" onClick={onToggle} aria-expanded={open}>
        <span className="activity-row-icon">
          <Brain size={13} />
        </span>
        <span className="activity-row-text">{item.text}</span>
        <span className="activity-row-time">{formatActivityTime(item.createdAt, language)}</span>
        <ChevronRight size={12} className={`activity-row-chevron${open ? ' open' : ''}`} />
      </button>
      {open ? <div className="activity-detail-reasoning">{item.text}</div> : null}
    </>
  );
}

function ActivityTool({
  item,
  open,
  onToggle,
  language,
}: {
  item: Extract<ChatItem, { kind: 'tool' }>;
  open: boolean;
  onToggle: () => void;
  language: string;
}) {
  const tool = item.tool;
  const label = tool.kind === 'execute' || /^bash\b/i.test(tool.title) ? bashCommand(tool) : tool.title;
  return (
    <>
      <button className="activity-row" onClick={onToggle} aria-expanded={open}>
        <span className="activity-row-icon">
          <ToolIcon kind={tool.kind} />
        </span>
        <span className="activity-row-text" title={label}>
          {label}
        </span>
        <span className="activity-row-time">{formatActivityTime(item.createdAt, language)}</span>
        <ChevronRight size={12} className={`activity-row-chevron${open ? ' open' : ''}`} />
      </button>
      {open ? (
        <div className="activity-detail">
          <ToolCard tool={tool} />
        </div>
      ) : null}
    </>
  );
}
