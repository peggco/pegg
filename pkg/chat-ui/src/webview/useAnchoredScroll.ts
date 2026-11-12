import { useCallback, useEffect, useLayoutEffect, useRef, useState, type RefObject } from 'react';

interface ScrollItem {
  id: string;
}

interface Snapshot {
  ids: string[];
  firstId: string | null;
  firstTop: number;
}

export interface AnchoredScroll {
  scrollerRef: RefObject<HTMLElement | null>;
  atBottom: boolean;
  onScroll: () => void;
  scrollToBottom: (smooth?: boolean) => void;
  follow: () => void;
}

const BOTTOM_PIN_PX = 48;
const PROGRAMMATIC_TTL_MS = 250;
const SMOOTH_TTL_MS = 600;
const TOLERANCE_PX = 2;
const SNAPSHOT_ATTEMPTS = 3;

function findMessageNode(root: HTMLElement, id: string): HTMLElement | null {
  const escaped =
    typeof CSS !== 'undefined' && typeof CSS.escape === 'function'
      ? CSS.escape(id)
      : id.replace(/["\\]/g, '\\$&');
  return root.querySelector<HTMLElement>(`[data-msg-id="${escaped}"]`);
}

function snapshot(root: HTMLElement, ids: string[]): Snapshot {
  const rootTop = root.getBoundingClientRect().top;
  const nodes = root.querySelectorAll<HTMLElement>('.msg-slot[data-msg-id]');
  let low = 0;
  let high = nodes.length - 1;
  let found = -1;
  while (low <= high) {
    const mid = (low + high) >> 1;
    const rect = nodes[mid].getBoundingClientRect();
    if (rect.bottom > rootTop) {
      found = mid;
      high = mid - 1;
    } else {
      low = mid + 1;
    }
  }
  if (found < 0) return { ids, firstId: null, firstTop: 0 };
  const node = nodes[found];
  return { ids, firstId: node.dataset.msgId ?? null, firstTop: node.getBoundingClientRect().top };
}

function isPrepend(previous: string[], next: string[]): boolean {
  if (previous.length === 0) return false;
  const start = next.indexOf(previous[0]);
  if (start <= 0 || start + previous.length > next.length) return false;
  for (let index = 0; index < previous.length; index += 1) {
    if (next[start + index] !== previous[index]) return false;
  }
  return true;
}

export function useAnchoredScroll(sessionKey: string, items: ScrollItem[]): AnchoredScroll {
  const scrollerRef = useRef<HTMLElement | null>(null);
  const pinnedRef = useRef(true);
  const [atBottom, setAtBottom] = useState(true);
  const programmaticTargetRef = useRef<number | null>(null);
  const smoothRef = useRef<{ target: number; until: number } | null>(null);
  const lastScrollTopRef = useRef(0);
  const snapshotRef = useRef<Snapshot | null>(null);
  const snapshotFrameRef = useRef(0);
  const idsRef = useRef<string[]>([]);
  const userScrollTokenRef = useRef(0);

  const setPinned = useCallback((pinned: boolean) => {
    if (pinnedRef.current === pinned) return;
    pinnedRef.current = pinned;
    setAtBottom(pinned);
  }, []);

  const scheduleSnapshot = useCallback(() => {
    if (snapshotFrameRef.current) return;
    snapshotFrameRef.current = window.requestAnimationFrame(() => {
      snapshotFrameRef.current = 0;
      const el = scrollerRef.current;
      if (el) snapshotRef.current = snapshot(el, idsRef.current);
    });
  }, []);

  const writeScrollTop = useCallback((el: HTMLElement, top: number) => {
    const target = Math.max(0, Math.min(top, el.scrollHeight - el.clientHeight));
    programmaticTargetRef.current = target;
    smoothRef.current = null;
    el.scrollTop = target;
    window.setTimeout(() => {
      if (programmaticTargetRef.current === target) programmaticTargetRef.current = null;
    }, PROGRAMMATIC_TTL_MS);
  }, []);

  const scrollToBottom = useCallback(
    (smooth = false) => {
      const el = scrollerRef.current;
      if (!el) return;
      setPinned(true);
      if (smooth && typeof el.scrollTo === 'function') {
        const target = Math.max(0, el.scrollHeight - el.clientHeight);
        smoothRef.current = { target, until: Date.now() + SMOOTH_TTL_MS };
        lastScrollTopRef.current = el.scrollTop;
        el.scrollTo({ top: el.scrollHeight, behavior: 'smooth' });
        return;
      }
      writeScrollTop(el, el.scrollHeight);
    },
    [setPinned, writeScrollTop],
  );

  const follow = useCallback(() => {
    const el = scrollerRef.current;
    if (!el || !pinnedRef.current) return;
    writeScrollTop(el, el.scrollHeight);
  }, [writeScrollTop]);

  const onScroll = useCallback(() => {
    const el = scrollerRef.current;
    if (!el) return;
    const current = el.scrollTop;
    const previous = lastScrollTopRef.current;
    lastScrollTopRef.current = current;

    const smooth = smoothRef.current;
    if (smooth) {
      if (Date.now() > smooth.until || Math.abs(current - smooth.target) <= TOLERANCE_PX) {
        smoothRef.current = null;
      } else if (current < previous - TOLERANCE_PX) {
        smoothRef.current = null;
      } else {
        return;
      }
    }

    const target = programmaticTargetRef.current;
    if (target !== null) {
      programmaticTargetRef.current = null;
      if (Math.abs(current - target) <= TOLERANCE_PX) {
        scheduleSnapshot();
        return;
      }
    }

    userScrollTokenRef.current += 1;
    const distance = el.scrollHeight - current - el.clientHeight;
    setPinned(distance < BOTTOM_PIN_PX);
    scheduleSnapshot();
  }, [scheduleSnapshot, setPinned]);

  const anchorTo = useCallback(
    (el: HTMLElement, id: string, oldTop: number, token: number) => {
      let attempts = 0;
      const step = () => {
        if (pinnedRef.current) return;
        if (attempts > 0 && userScrollTokenRef.current !== token) return;
        const node = findMessageNode(el, id);
        if (!node) return;
        const delta = node.getBoundingClientRect().top - oldTop;
        if (Math.abs(delta) > 0.5) {
          writeScrollTop(el, el.scrollTop + delta);
        }
        attempts += 1;
        if (attempts < SNAPSHOT_ATTEMPTS) {
          window.requestAnimationFrame(step);
        }
      };
      step();
    },
    [writeScrollTop],
  );

  useEffect(() => {
    pinnedRef.current = true;
    setAtBottom(true);
    programmaticTargetRef.current = null;
    smoothRef.current = null;
    lastScrollTopRef.current = 0;
    snapshotRef.current = null;
    userScrollTokenRef.current += 1;
    const frame = window.requestAnimationFrame(() => scrollToBottom(false));
    return () => window.cancelAnimationFrame(frame);
  }, [sessionKey, scrollToBottom]);

  useEffect(
    () => () => {
      if (snapshotFrameRef.current) window.cancelAnimationFrame(snapshotFrameRef.current);
    },
    [],
  );

  useLayoutEffect(() => {
    const el = scrollerRef.current;
    if (!el) return;
    const previous = snapshotRef.current;
    const nextIds = items.map((item) => item.id);
    idsRef.current = nextIds;
    if (pinnedRef.current) {
      writeScrollTop(el, el.scrollHeight);
    } else if (previous?.firstId && isPrepend(previous.ids, nextIds)) {
      anchorTo(el, previous.firstId, previous.firstTop, userScrollTokenRef.current);
    }
    snapshotRef.current = snapshot(el, nextIds);
  }, [items, anchorTo, writeScrollTop]);

  return { scrollerRef, atBottom, onScroll, scrollToBottom, follow };
}
