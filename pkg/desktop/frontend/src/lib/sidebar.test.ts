import { describe, expect, it } from 'vitest';
import {
  SIDEBAR_DEFAULT_WIDTH,
  SIDEBAR_MAX_WIDTH,
  SIDEBAR_MIN_WIDTH,
  clampSidebarWidth,
  dragSidebarWidth,
  resolveSidebarWidth,
  shouldCollapseSidebar,
} from './sidebar';

describe('sidebar width helpers', () => {
  it('resolves missing or invalid widths to the default', () => {
    expect(resolveSidebarWidth(undefined)).toBe(SIDEBAR_DEFAULT_WIDTH);
    expect(resolveSidebarWidth(0)).toBe(SIDEBAR_DEFAULT_WIDTH);
    expect(resolveSidebarWidth(120)).toBe(SIDEBAR_DEFAULT_WIDTH);
  });

  it('clamps widths to the allowed range', () => {
    expect(clampSidebarWidth(120)).toBe(SIDEBAR_MIN_WIDTH);
    expect(clampSidebarWidth(900)).toBe(SIDEBAR_MAX_WIDTH);
    expect(clampSidebarWidth(Number.NaN)).toBe(SIDEBAR_DEFAULT_WIDTH);
    expect(clampSidebarWidth(311.6)).toBe(312);
  });

  it('keeps the persisted width when it is already valid', () => {
    expect(resolveSidebarWidth(340)).toBe(340);
    expect(resolveSidebarWidth(900)).toBe(SIDEBAR_MAX_WIDTH);
  });

  it('tracks drag deltas without clamping below the minimum', () => {
    expect(dragSidebarWidth(272, -100)).toBe(172);
    expect(dragSidebarWidth(272, -400)).toBe(0);
    expect(dragSidebarWidth(272, 400)).toBe(SIDEBAR_MAX_WIDTH);
  });

  it('collapses only below the threshold', () => {
    expect(shouldCollapseSidebar(139)).toBe(true);
    expect(shouldCollapseSidebar(140)).toBe(false);
  });
});
