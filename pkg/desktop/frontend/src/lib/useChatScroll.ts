import { useCallback, useEffect, useRef, useState } from 'react';
import { BOTTOM_PIN_PX, clampScrollTop, distanceFromBottom } from './scroll';

const PROGRAMMATIC_TIMEOUT_MS = 400;
const SMOOTH_TIMEOUT_MS = 2000;
const TOLERANCE_PX = 2;
const AT_END_PX = 2;

export interface ChatScroller {
  scrollerRef: React.RefObject<HTMLDivElement | null>;
  pinnedRef: React.MutableRefObject<boolean>;
  atEndRef: React.MutableRefObject<boolean>;
  showJump: boolean;
  writeScrollTop: (top: number, smooth?: boolean) => void;
  jumpToBottom: () => void;
  onScroll: (event: React.UIEvent<HTMLDivElement>) => boolean;
}

interface ChatScrollerOptions {
  sessionKey: string;
  syncPinned: (pinned: boolean) => void;
}

export function useChatScroller({ sessionKey, syncPinned }: ChatScrollerOptions): ChatScroller {
  const scrollerRef = useRef<HTMLDivElement>(null);
  const pinnedRef = useRef(true);
  const atEndRef = useRef(true);
  const [showJump, setShowJump] = useState(false);
  const syncPinnedRef = useRef(syncPinned);
  syncPinnedRef.current = syncPinned;
  const programmaticRef = useRef<{ target: number; smooth: boolean; until: number } | null>(null);
  const lastScrollTopRef = useRef(0);
  const syncFrameRef = useRef(0);
  const timerRef = useRef(0);

  const scheduleSync = useCallback(() => {
    if (syncFrameRef.current) return;
    syncFrameRef.current = window.requestAnimationFrame(() => {
      syncFrameRef.current = 0;
      syncPinnedRef.current(pinnedRef.current);
    });
  }, []);

  const setPinned = useCallback(
    (pinned: boolean) => {
      if (pinnedRef.current === pinned) return;
      pinnedRef.current = pinned;
      setShowJump(!pinned);
      scheduleSync();
    },
    [scheduleSync],
  );

  const writeScrollTop = useCallback((top: number, smooth = false) => {
    const el = scrollerRef.current;
    if (!el) return;
    const target = clampScrollTop(top, el);
    const timeout = smooth ? SMOOTH_TIMEOUT_MS : PROGRAMMATIC_TIMEOUT_MS;
    programmaticRef.current = { target, smooth, until: Date.now() + timeout };
    if (timerRef.current) window.clearTimeout(timerRef.current);
    timerRef.current = window.setTimeout(() => {
      programmaticRef.current = null;
      timerRef.current = 0;
    }, timeout);
    if (smooth) el.scrollTo({ top: target, behavior: 'smooth' });
    else el.scrollTop = target;
    atEndRef.current = distanceFromBottom(el) <= AT_END_PX;
  }, []);

  const onScroll = useCallback(
    (event: React.UIEvent<HTMLDivElement>): boolean => {
      const el = event.currentTarget;
      const previous = lastScrollTopRef.current;
      lastScrollTopRef.current = el.scrollTop;
      const programmatic = programmaticRef.current;
      if (programmatic) {
        const reached = Math.abs(el.scrollTop - programmatic.target) <= TOLERANCE_PX;
        if (reached) {
          programmaticRef.current = null;
          if (timerRef.current) {
            window.clearTimeout(timerRef.current);
            timerRef.current = 0;
          }
          atEndRef.current = distanceFromBottom(el) <= AT_END_PX;
          return false;
        }
        if (programmatic.smooth) {
          const userScrolledUp = el.scrollTop < previous - 1;
          if (!userScrolledUp && Date.now() <= programmatic.until) return false;
        }
        programmaticRef.current = null;
      }
      const distance = distanceFromBottom(el);
      atEndRef.current = distance <= AT_END_PX;
      setPinned(distance <= BOTTOM_PIN_PX);
      return true;
    },
    [setPinned],
  );

  const jumpToBottom = useCallback(() => {
    pinnedRef.current = true;
    atEndRef.current = true;
    setShowJump(false);
    scheduleSync();
    const el = scrollerRef.current;
    if (el) writeScrollTop(el.scrollHeight, true);
  }, [scheduleSync, writeScrollTop]);

  useEffect(() => {
    pinnedRef.current = true;
    atEndRef.current = true;
    setShowJump(false);
    programmaticRef.current = null;
    lastScrollTopRef.current = 0;
    syncPinnedRef.current(true);
    const frame = window.requestAnimationFrame(() => {
      const el = scrollerRef.current;
      if (el) writeScrollTop(el.scrollHeight);
    });
    return () => window.cancelAnimationFrame(frame);
  }, [sessionKey, writeScrollTop]);

  useEffect(() => {
    const el = scrollerRef.current;
    if (!el) return;
    const unpin = () => {
      if (programmaticRef.current?.smooth) {
        el.scrollTo({ top: el.scrollTop, behavior: 'auto' });
      }
      programmaticRef.current = null;
      setPinned(false);
    };
    const onWheel = (event: WheelEvent) => {
      if (event.deltaY < 0) unpin();
    };
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'PageUp' || event.key === 'ArrowUp' || event.key === 'Home') unpin();
    };
    let touchY: number | null = null;
    const onTouchStart = (event: TouchEvent) => {
      touchY = event.touches[0]?.clientY ?? null;
    };
    const onTouchMove = (event: TouchEvent) => {
      const y = event.touches[0]?.clientY;
      if (touchY !== null && y !== undefined && y > touchY + TOLERANCE_PX) unpin();
      touchY = y ?? touchY;
    };
    el.addEventListener('wheel', onWheel, { passive: true });
    el.addEventListener('touchstart', onTouchStart, { passive: true });
    el.addEventListener('touchmove', onTouchMove, { passive: true });
    window.addEventListener('keydown', onKeyDown, true);
    return () => {
      el.removeEventListener('wheel', onWheel);
      el.removeEventListener('touchstart', onTouchStart);
      el.removeEventListener('touchmove', onTouchMove);
      window.removeEventListener('keydown', onKeyDown, true);
    };
  }, [sessionKey, setPinned]);

  useEffect(
    () => () => {
      if (syncFrameRef.current) window.cancelAnimationFrame(syncFrameRef.current);
      if (timerRef.current) window.clearTimeout(timerRef.current);
    },
    [],
  );

  return { scrollerRef, pinnedRef, atEndRef, showJump, writeScrollTop, jumpToBottom, onScroll };
}
