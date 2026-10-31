import type { SVGProps } from 'react';
import markUrl from '../assets/pegg-mark.png';

export function Starburst({ size = 24, ...props }: SVGProps<SVGSVGElement> & { size?: number }) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 32 32"
      fill="none"
      stroke="currentColor"
      strokeWidth="2.6"
      strokeLinecap="round"
      aria-hidden="true"
      {...props}
    >
      <path d="M16 3.2v25.6" />
      <path d="M3.2 16h25.6" />
      <path d="M6.9 6.9l18.2 18.2" />
      <path d="M25.1 6.9L6.9 25.1" />
      <path d="M16 8.2v15.6" opacity="0.35" />
      <path d="M8.2 16h15.6" opacity="0.35" />
    </svg>
  );
}

export function BrandMark({ size = 20 }: { size?: number }) {
  return <Starburst size={size} strokeWidth={3} />;
}

export function LogoMark({ size = 22 }: { size?: number }) {
  return (
    <span
      aria-hidden="true"
      className="pg-logo-mark"
      style={{
        width: size,
        height: size,
        WebkitMaskImage: `url(${markUrl})`,
        maskImage: `url(${markUrl})`,
      }}
    />
  );
}
