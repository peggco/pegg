import { describe, expect, it } from 'vitest';
import { clampScrollTop, distanceFromBottom, isPinnedToBottom } from './scroll';

describe('bottom pinning', () => {
  it('measures the distance to the bottom', () => {
    expect(distanceFromBottom({ scrollHeight: 1000, scrollTop: 700, clientHeight: 250 })).toBe(50);
  });

  it('pins within the threshold only', () => {
    expect(isPinnedToBottom({ scrollHeight: 1000, scrollTop: 780, clientHeight: 200 })).toBe(true);
    expect(isPinnedToBottom({ scrollHeight: 1000, scrollTop: 700, clientHeight: 200 })).toBe(false);
    expect(isPinnedToBottom({ scrollHeight: 1000, scrollTop: 700, clientHeight: 200 }, 120)).toBe(true);
  });

  it('tolerates overscroll', () => {
    expect(isPinnedToBottom({ scrollHeight: 1000, scrollTop: 1020, clientHeight: 200 })).toBe(true);
  });
});

describe('clampScrollTop', () => {
  it('clamps to the scrollable range', () => {
    expect(clampScrollTop(-10, { scrollHeight: 1000, clientHeight: 200 })).toBe(0);
    expect(clampScrollTop(400, { scrollHeight: 1000, clientHeight: 200 })).toBe(400);
    expect(clampScrollTop(2000, { scrollHeight: 1000, clientHeight: 200 })).toBe(800);
  });

  it('returns 0 when there is nothing to scroll', () => {
    expect(clampScrollTop(100, { scrollHeight: 100, clientHeight: 200 })).toBe(0);
  });
});
