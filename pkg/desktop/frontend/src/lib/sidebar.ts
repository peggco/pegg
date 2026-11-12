export const SIDEBAR_DEFAULT_WIDTH = 272;
export const SIDEBAR_MIN_WIDTH = 200;
export const SIDEBAR_MAX_WIDTH = 520;
export const SIDEBAR_COLLAPSE_WIDTH = 140;

export function clampSidebarWidth(width: number): number {
  if (!Number.isFinite(width)) return SIDEBAR_DEFAULT_WIDTH;
  return Math.min(SIDEBAR_MAX_WIDTH, Math.max(SIDEBAR_MIN_WIDTH, Math.round(width)));
}

export function resolveSidebarWidth(width: number | undefined): number {
  if (!width || width < SIDEBAR_MIN_WIDTH) return SIDEBAR_DEFAULT_WIDTH;
  return clampSidebarWidth(width);
}

export function dragSidebarWidth(startWidth: number, deltaX: number): number {
  return Math.max(0, Math.min(SIDEBAR_MAX_WIDTH, Math.round(startWidth + deltaX)));
}

export function shouldCollapseSidebar(width: number): boolean {
  return width < SIDEBAR_COLLAPSE_WIDTH;
}
