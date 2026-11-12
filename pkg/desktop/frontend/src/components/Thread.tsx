import { useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react';
import { defaultRangeExtractor, useVirtualizer } from '@tanstack/react-virtual';
import { useChatScroller } from '../lib/useChatScroll';
import { TOP_PREFETCH_PX } from '../lib/scroll';
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
import type { ChatItem, ToolState } from '../state/chat';
import { activeActivityKey, activityCounts, buildHistoryRows, formatActivityTime, type ActivityCounts, type HistoryRow } from '../lib/activity';
import { ToolCard, ToolIcon, bashCommand, classify, computeRows, outputText, writeContent } from './ToolViews';
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
  const activeGroupKey = activeActivityKey(rows, activeAgent ? activeRun?.status === 'running' : running);
  const [openGroups, setOpenGroups] = useState<Set<string>>(() => new Set());
  const [openRows, setOpenRows] = useState<Set<string>>(() => new Set());
  const [copied, setCopied] = useState<string | null>(null);
  const t = useT();
  const sessionKey = `${acpId ?? ''}:${activeAgent ?? ''}`;
  const scroll = useChatScroller({ sessionKey, syncPinned: setStickToBottom });
  const { atEndRef, scrollerRef, writeScrollTop } = scroll;
  const sentinelRef = useRef<HTMLDivElement>(null);
  const autoLoadSeqRef = useRef<number | null>(null);
  const rowsRef = useRef(rows);
  rowsRef.current = rows;
  const [measureBatch, setMeasureBatch] = useState<{ start: number; end: number } | null>(null);

  const groupOpen = (id: string, key: string) => openGroups.has(id) || openGroups.has(key);
  const toggleGroup = (id: string, key: string) =>
    setOpenGroups((current) => {
      const open = current.has(id) || current.has(key);
      const next = new Set(current);
      next.delete(id);
      next.delete(key);
      if (!open) {
        next.add(id);
        next.add(key);
      }
      return next;
    });
  const toggleRow = (id: string) => setOpenRows((current) => toggleId(current, id));

  const virtualizer = useVirtualizer({
    count: rows.length,
    getScrollElement: () => scrollerRef.current,
    estimateSize: (index) => estimateRowHeight(rows[index]),
    overscan: 12,
    getItemKey: (index) => rows[index]?.key ?? index,
    anchorTo: 'end',
    followOnAppend: 'auto',
    scrollEndThreshold: 2,
    rangeExtractor: (range) => {
      const indices = new Set(defaultRangeExtractor(range));
      if (measureBatch) {
        for (
          let index = Math.max(0, measureBatch.start);
          index <= measureBatch.end && index < rows.length;
          index += 1
        ) {
          indices.add(index);
        }
      }
      return Array.from(indices).sort((a, b) => a - b);
    },
  });
  const totalSize = virtualizer.getTotalSize();

  useEffect(() => {
    if (activeAgent) return;
    let cancelled = false;
    let timer = 0;
    const firstUnmeasured = (): number => {
      const current = rowsRef.current;
      for (let index = current.length - 1; index >= 0; index -= 1) {
        if (!virtualizer.itemSizeCache.has(current[index].key)) return index;
      }
      return -1;
    };
    const tick = () => {
      if (cancelled) return;
      if (virtualizer.isScrolling) {
        timer = window.setTimeout(tick, 150);
        return;
      }
      const end = firstUnmeasured();
      if (end < 0) {
        setMeasureBatch(null);
        return;
      }
      const start = Math.max(0, end - 23);
      setMeasureBatch({ start, end });
      timer = window.setTimeout(() => {
        if (cancelled) return;
        setMeasureBatch(null);
        timer = window.setTimeout(tick, 70);
      }, 70);
    };
    timer = window.setTimeout(tick, 60);
    return () => {
      cancelled = true;
      window.clearTimeout(timer);
    };
  }, [rows.length, historyMode, activeAgent, virtualizer]);

  useLayoutEffect(() => {
    if (!atEndRef.current) return;
    const el = scrollerRef.current;
    if (el) writeScrollTop(el.scrollHeight);
  }, [plan, permissions, asks, running, rows, totalSize, scrollerRef, atEndRef, writeScrollTop]);

  useEffect(() => {
    setOpenGroups(new Set());
    setOpenRows(new Set());
    setMeasureBatch(null);
    autoLoadSeqRef.current = null;
  }, [sessionKey]);

  const historyRef = useRef({ hasMore: historyHasMore, loading: historyLoading, enabled: !activeAgent });
  historyRef.current = { hasMore: historyHasMore, loading: historyLoading, enabled: !activeAgent };

  useEffect(() => {
    const root = scrollerRef.current;
    const target = sentinelRef.current;
    if (!root || !target || typeof IntersectionObserver === 'undefined') return;
    const observer = new IntersectionObserver(
      (entries) => {
        if (!entries.some((entry) => entry.isIntersecting)) return;
        const history = historyRef.current;
        if (!history.enabled || !history.hasMore || history.loading) return;
        void loadOlderMessages();
      },
      { root, rootMargin: `${TOP_PREFETCH_PX}px 0px 0px 0px`, threshold: 0 },
    );
    observer.observe(target);
    return () => observer.disconnect();
  }, [scrollerRef, sessionKey, activeAgent]);

  useEffect(() => {
    if (activeAgent || !historyHasMore || historyLoading) return;
    const el = scrollerRef.current;
    if (!el) return;
    if (el.scrollHeight > el.clientHeight + 32) return;
    if (autoLoadSeqRef.current === historyOldestSeq) return;
    autoLoadSeqRef.current = historyOldestSeq;
    void loadOlderMessages();
  }, [activeAgent, historyHasMore, historyLoading, historyOldestSeq, totalSize, rows, scrollerRef]);

  const onThreadScroll = (event: React.UIEvent<HTMLDivElement>) => {
    const userScroll = scroll.onScroll(event);
    if (!userScroll || activeAgent) return;
    if (
      event.currentTarget.scrollTop <= TOP_PREFETCH_PX &&
      historyHasMore &&
      !historyLoading
    ) {
      void loadOlderMessages();
    }
  };

  const jumpButton = scroll.showJump ? (
    <button
      className="scroll-bottom-btn"
      title={t('thread.scrollToBottom')}
      aria-label={t('thread.scrollToBottom')}
      onClick={scroll.jumpToBottom}
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
          <div className="thread-wrap" ref={scrollerRef} onScroll={onThreadScroll} onClick={onThreadClick}>
            <div className="thread">
              {activeRun.task ? <div className="subagent-modal-task">{activeRun.task}</div> : null}
              {rows.map((row) => (
                <div className="thread-item" key={row.key}>
                  {row.kind === 'activity' ? (
                    <ActivityGroup
                      group={row}
                      open={groupOpen(row.id, row.key)}
                      onToggle={() => toggleGroup(row.id, row.key)}
                      openRows={openRows}
                      onToggleRow={toggleRow}
                      active={row.key === activeGroupKey}
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
    <div className="thread-area">
      {loading ? (
        <div className="thread-status" role="status">
          <span className="spinner" /> {t('thread.restoring')}
        </div>
      ) : null}
      <div className="thread-wrap" ref={scrollerRef} onScroll={onThreadScroll} onClick={onThreadClick}>
        <div ref={sentinelRef} className="thread-sentinel" aria-hidden="true" />
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
                        open={groupOpen(row.id, row.key)}
                        onToggle={() => toggleGroup(row.id, row.key)}
                        openRows={openRows}
                        onToggleRow={toggleRow}
                        active={row.key === activeGroupKey}
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
  active,
  onToggle,
  openRows,
  onToggleRow,
}: {
  group: Extract<HistoryRow, { kind: 'activity' }>;
  open: boolean;
  active?: boolean;
  onToggle: () => void;
  openRows: Set<string>;
  onToggleRow: (id: string) => void;
}) {
  const t = useT();
  const language = useStore((s) => s.language);
  const counts = activityCounts(group.items);
  return (
    <div className={`activity${open ? ' open' : ''}${active ? ' is-active' : ''}`}>
      <button className="activity-head" onClick={onToggle} aria-expanded={open}>
        <span className="activity-head-icon">
          <Wrench size={13} />
        </span>
        <span className="activity-text">
          <span className="activity-summary">{activitySummary(counts, t)}</span>
          <span className="activity-time">{formatActivityTime(group.createdAt, language)}</span>
        </span>
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
const DIFF_PREVIEW_ROWS = 160;
const BASH_PREVIEW_LINES = 24;
const WRITE_PREVIEW_LINES = 30;
const rowHeightCache = new WeakMap<ChatItem, number>();

function textLines(text: string, width: number): number {
  if (!text) return 1;
  let lines = 0;
  for (const part of text.split('\n')) {
    lines += Math.max(1, Math.ceil(part.length / width));
  }
  return lines;
}

function estimateToolHeight(tool: ToolState): number {
  if (tool.task) return 150;
  const extra = tool.error ? 36 : 0;
  switch (classify(tool)) {
    case 'bash': {
      const total = outputText(tool.output).split('\n').length;
      const shown = Math.min(total, BASH_PREVIEW_LINES);
      const body = Math.min(
        340,
        24 + shown * 19 + (total > BASH_PREVIEW_LINES ? 30 : 0),
      );
      return 30 + body + extra;
    }
    case 'write': {
      const content = writeContent(tool);
      const total = content ? content.split('\n').length : 0;
      const shown = Math.min(total, WRITE_PREVIEW_LINES);
      const body = Math.min(
        360,
        20 + shown * 19 + (content.length > 16000 || total > WRITE_PREVIEW_LINES ? 30 : 0),
      );
      return 36 + body + 16 + extra;
    }
    case 'edit': {
      if (!tool.diff) return 64 + extra;
      const rows = computeRows(tool.diff.oldText ?? '', tool.diff.newText ?? '').length;
      const body = Math.min(420, rows * 19 + 8);
      return 34 + body + 16 + (rows > DIFF_PREVIEW_ROWS ? 30 : 0) + extra;
    }
    case 'files':
      return 62 + (tool.files?.length ?? 0) * 36 + extra;
    default:
      return 52 + (tool.error ? 40 : 0);
  }
}

function estimateAssistantHeight(text: string): number {
  let height = 52;
  const parts = text.split('```');
  for (let index = 0; index < parts.length; index += 1) {
    const part = parts[index];
    if (index % 2 === 0) {
      height += textLines(part, 80) * 22;
    } else {
      const lines = part.split('\n').length;
      height += 66 + Math.min(460, lines * 19 + 22);
    }
  }
  return Math.min(8000, Math.max(52, height));
}

function estimateItemHeight(item: ChatItem): number {
  switch (item.kind) {
    case 'user': {
      const attachments = item.attachments?.length ?? 0;
      return Math.min(1600, 44 + textLines(item.text, 90) * 20 + (attachments ? 28 : 0));
    }
    case 'assistant':
      return estimateAssistantHeight(item.text);
    case 'reasoning':
      return 44;
    case 'tool':
      return estimateToolHeight(item.tool);
    case 'subagent':
      return 140;
    default:
      return 48;
  }
}

const ROW_GAP = 12;

function estimateRowHeight(row: HistoryRow | undefined): number {
  if (!row) return 96 + ROW_GAP;
  if (row.kind === 'activity') return 44 + ROW_GAP;
  const cached = rowHeightCache.get(row.item);
  if (cached !== undefined) return cached + ROW_GAP;
  const height = estimateItemHeight(row.item);
  rowHeightCache.set(row.item, height);
  return height + ROW_GAP;
}
