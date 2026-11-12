export const BOTTOM_PIN_PX = 24;
export const TOP_PREFETCH_PX = 600;

export interface ScrollMetrics {
  scrollHeight: number;
  scrollTop: number;
  clientHeight: number;
}

export function distanceFromBottom(el: ScrollMetrics): number {
  return el.scrollHeight - el.scrollTop - el.clientHeight;
}

export function isPinnedToBottom(el: ScrollMetrics, threshold = BOTTOM_PIN_PX): boolean {
  return distanceFromBottom(el) <= threshold;
}

export function clampScrollTop(
  top: number,
  el: Pick<ScrollMetrics, 'scrollHeight' | 'clientHeight'>,
): number {
  return Math.max(0, Math.min(top, el.scrollHeight - el.clientHeight));
}
