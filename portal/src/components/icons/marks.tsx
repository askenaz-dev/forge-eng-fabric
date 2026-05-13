// Shared brand-mark registry for the Forge portal.
// Marks have stable `id` attributes so they can be referenced by external
// assets (notebook, screenshots). Each mark is provided as both a React
// component (inline SVG) and a stable url (under /public/).

import type { SVGProps } from "react";

export type MarkId = "alfred-mark" | "alfred-mark-mono" | "alfred-mark-working";

export const MARK_URLS: Record<MarkId, string> = {
  "alfred-mark": "/alfred-mark.svg",
  "alfred-mark-mono": "/alfred-mark-mono.svg",
  "alfred-mark-working": "/alfred-mark-working.svg",
};

type MarkProps = SVGProps<SVGSVGElement> & { size?: number };

export function AlfredMark({ size = 24, ...rest }: MarkProps) {
  return (
    <svg
      id="alfred-mark"
      viewBox="0 0 24 24"
      width={size}
      height={size}
      role="img"
      aria-label="Alfred"
      {...rest}
    >
      <defs>
        <linearGradient id="alfred-ember-grad-inline" x1="0" y1="0" x2="0" y2="1">
          <stop offset="0%" stopColor="#F4A77E" />
          <stop offset="60%" stopColor="#DC4318" />
          <stop offset="100%" stopColor="#8A2509" />
        </linearGradient>
      </defs>
      <path d="M4 10 L12 13 L20 10 L20 14 L12 11 L4 14 Z" fill="#1A1614" />
      <circle cx="12" cy="12" r="2.4" fill="url(#alfred-ember-grad-inline)" />
      <rect x="11.2" y="3.5" width="1.6" height="4.5" rx="0.6" fill="#1A1614" />
      <rect x="11.2" y="3.5" width="3.2" height="1.4" rx="0.4" fill="#1A1614" />
    </svg>
  );
}

export function AlfredMarkMono({ size = 24, ...rest }: MarkProps) {
  return (
    <svg
      id="alfred-mark-mono"
      viewBox="0 0 24 24"
      width={size}
      height={size}
      role="img"
      aria-label="Alfred"
      {...rest}
    >
      <path d="M4 10 L12 13 L20 10 L20 14 L12 11 L4 14 Z" fill="currentColor" />
      <circle cx="12" cy="12" r="2.4" fill="currentColor" />
      <rect x="11.2" y="3.5" width="1.6" height="4.5" rx="0.6" fill="currentColor" />
      <rect x="11.2" y="3.5" width="3.2" height="1.4" rx="0.4" fill="currentColor" />
    </svg>
  );
}
