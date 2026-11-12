import { useId, type ReactNode } from 'react';
import type { ChatHistoryMode, Direction, LayoutMode, SidebarMode } from '../types';

function Frame({ children }: { children: ReactNode }) {
  const clip = useId().replace(/:/g, '');
  return (
    <svg className="option-preview" viewBox="0 0 128 80" aria-hidden="true">
      <defs>
        <clipPath id={clip}>
          <rect x="0" y="0" width="128" height="80" rx="6" />
        </clipPath>
      </defs>
      <rect className="thumb-bg" x="0" y="0" width="128" height="80" rx="6" />
      <g clipPath={`url(#${clip})`}>{children}</g>
    </svg>
  );
}

function SidebarLines() {
  return (
    <>
      <rect className="thumb-line strong" x="9" y="12" width="20" height="3" rx="1.5" />
      <rect className="thumb-line" x="9" y="24" width="18" height="2" rx="1" />
      <rect className="thumb-line" x="9" y="30" width="14" height="2" rx="1" />
      <rect className="thumb-line" x="9" y="36" width="20" height="2" rx="1" />
      <rect className="thumb-line" x="9" y="48" width="16" height="2" rx="1" />
      <rect className="thumb-line" x="9" y="54" width="12" height="2" rx="1" />
    </>
  );
}

export function SidebarModePreview({ mode }: { mode: SidebarMode }) {
  if (mode === 'inset') {
    return (
      <Frame>
        <rect className="thumb-side" x="8" y="8" width="30" height="64" rx="4" />
        <rect className="thumb-card" x="46" y="8" width="74" height="64" rx="4" />
        <rect className="thumb-line strong" x="15" y="16" width="16" height="3" rx="1.5" />
        <rect className="thumb-line" x="15" y="26" width="14" height="2" rx="1" />
        <rect className="thumb-line" x="15" y="32" width="18" height="2" rx="1" />
        <rect className="thumb-line" x="15" y="38" width="10" height="2" rx="1" />
        <rect className="thumb-line strong" x="55" y="20" width="42" height="3" rx="1.5" />
        <rect className="thumb-line" x="55" y="30" width="54" height="2" rx="1" />
        <rect className="thumb-line" x="55" y="36" width="38" height="2" rx="1" />
        <rect className="thumb-line" x="55" y="42" width="48" height="2" rx="1" />
      </Frame>
    );
  }
  if (mode === 'floating') {
    return (
      <Frame>
        <rect className="thumb-shadow" x="10" y="10" width="32" height="64" rx="5" />
        <rect className="thumb-card" x="7" y="7" width="32" height="64" rx="5" />
        <rect className="thumb-line strong" x="14" y="16" width="16" height="3" rx="1.5" />
        <rect className="thumb-line" x="14" y="26" width="14" height="2" rx="1" />
        <rect className="thumb-line" x="14" y="32" width="18" height="2" rx="1" />
        <rect className="thumb-line" x="14" y="38" width="10" height="2" rx="1" />
        <rect className="thumb-line strong" x="50" y="18" width="48" height="3" rx="1.5" />
        <rect className="thumb-line" x="50" y="28" width="62" height="2" rx="1" />
        <rect className="thumb-line" x="50" y="34" width="46" height="2" rx="1" />
        <rect className="thumb-line" x="50" y="40" width="54" height="2" rx="1" />
      </Frame>
    );
  }
  return (
    <Frame>
      <rect className="thumb-side" x="0" y="0" width="38" height="80" />
      <SidebarLines />
      <rect className="thumb-line strong" x="48" y="16" width="52" height="3" rx="1.5" />
      <rect className="thumb-line" x="48" y="28" width="64" height="2" rx="1" />
      <rect className="thumb-line" x="48" y="34" width="48" height="2" rx="1" />
      <rect className="thumb-line" x="48" y="40" width="58" height="2" rx="1" />
    </Frame>
  );
}

export function LayoutPreview({ mode }: { mode: LayoutMode }) {
  if (mode === 'compact') {
    return (
      <Frame>
        <rect className="thumb-side" x="0" y="0" width="24" height="80" />
        <rect className="thumb-line" x="6" y="14" width="12" height="3" rx="1.5" />
        <rect className="thumb-line" x="6" y="24" width="12" height="2" rx="1" />
        <rect className="thumb-line" x="6" y="30" width="9" height="2" rx="1" />
        <rect className="thumb-line strong" x="34" y="14" width="48" height="3" rx="1.5" />
        <rect className="thumb-line" x="34" y="24" width="84" height="2" rx="1" />
        <rect className="thumb-line" x="34" y="30" width="70" height="2" rx="1" />
        <rect className="thumb-line" x="34" y="36" width="78" height="2" rx="1" />
        <rect className="thumb-line" x="34" y="46" width="60" height="2" rx="1" />
      </Frame>
    );
  }
  if (mode === 'full') {
    return (
      <Frame>
        <rect className="thumb-side" x="0" y="0" width="38" height="80" />
        <SidebarLines />
        <rect className="thumb-block" x="48" y="14" width="70" height="14" rx="3" />
        <rect className="thumb-block" x="48" y="34" width="70" height="14" rx="3" />
        <rect className="thumb-line strong" x="48" y="54" width="44" height="3" rx="1.5" />
      </Frame>
    );
  }
  return (
    <Frame>
      <rect className="thumb-side" x="0" y="0" width="38" height="80" />
      <SidebarLines />
      <rect className="thumb-line strong" x="54" y="18" width="40" height="3" rx="1.5" />
      <rect className="thumb-line" x="54" y="30" width="52" height="2" rx="1" />
      <rect className="thumb-line" x="54" y="36" width="38" height="2" rx="1" />
      <rect className="thumb-line" x="54" y="42" width="46" height="2" rx="1" />
    </Frame>
  );
}

export function DirectionPreview({ direction }: { direction: Direction }) {
  if (direction === 'rtl') {
    return (
      <Frame>
        <rect className="thumb-side" x="92" y="0" width="36" height="80" />
        <rect className="thumb-line strong" x="100" y="12" width="18" height="3" rx="1.5" />
        <rect className="thumb-line" x="100" y="24" width="16" height="2" rx="1" />
        <rect className="thumb-line" x="100" y="30" width="12" height="2" rx="1" />
        <rect className="thumb-line strong" x="18" y="16" width="50" height="3" rx="1.5" />
        <rect className="thumb-line" x="18" y="28" width="62" height="2" rx="1" />
        <rect className="thumb-line" x="18" y="34" width="46" height="2" rx="1" />
        <rect className="thumb-line" x="18" y="40" width="54" height="2" rx="1" />
      </Frame>
    );
  }
  return (
    <Frame>
      <rect className="thumb-side" x="0" y="0" width="36" height="80" />
      <rect className="thumb-line strong" x="9" y="12" width="18" height="3" rx="1.5" />
      <rect className="thumb-line" x="9" y="24" width="16" height="2" rx="1" />
      <rect className="thumb-line" x="9" y="30" width="12" height="2" rx="1" />
      <rect className="thumb-line strong" x="18" y="16" width="50" height="3" rx="1.5" />
      <rect className="thumb-line" x="46" y="28" width="62" height="2" rx="1" />
      <rect className="thumb-line" x="46" y="34" width="46" height="2" rx="1" />
      <rect className="thumb-line" x="46" y="40" width="54" height="2" rx="1" />
    </Frame>
  );
}

export function ChatHistoryPreview({ mode }: { mode: ChatHistoryMode }) {
  if (mode === 'compact') {
    return (
      <Frame>
        <rect className="thumb-bubble" x="76" y="8" width="44" height="12" rx="6" />
        <rect className="thumb-summary" x="8" y="28" width="112" height="13" rx="4" />
        <rect className="thumb-line" x="14" y="33" width="52" height="3" rx="1.5" />
        <rect className="thumb-line" x="104" y="33" width="6" height="3" rx="1.5" />
        <rect className="thumb-line" x="8" y="50" width="86" height="3" rx="1.5" />
        <rect className="thumb-line" x="8" y="58" width="74" height="3" rx="1.5" />
        <rect className="thumb-line" x="8" y="66" width="92" height="3" rx="1.5" />
      </Frame>
    );
  }
  return (
    <Frame>
      <rect className="thumb-bubble" x="76" y="8" width="44" height="12" rx="6" />
      <rect className="thumb-card" x="8" y="27" width="112" height="13" rx="3" />
      <rect className="thumb-line" x="14" y="32" width="48" height="3" rx="1.5" />
      <rect className="thumb-card" x="8" y="44" width="112" height="13" rx="3" />
      <rect className="thumb-line" x="14" y="49" width="64" height="3" rx="1.5" />
      <rect className="thumb-line" x="8" y="64" width="80" height="3" rx="1.5" />
      <rect className="thumb-line" x="8" y="71" width="66" height="3" rx="1.5" />
    </Frame>
  );
}
