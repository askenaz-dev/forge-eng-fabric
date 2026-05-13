// Forge Portal icon set — 1.5px stroke, square caps, currentColor.
// Ported from design/fabric-unzipped/portal-icons.jsx; named exports are
// tree-shakable so unused icons drop out of the bundle.

import type { SVGProps } from "react";

type IconProps = SVGProps<SVGSVGElement>;

const stroke: IconProps = {
  fill: "none",
  stroke: "currentColor",
  strokeWidth: 1.5,
  strokeLinecap: "square",
  strokeLinejoin: "miter",
};

const round: IconProps = { ...stroke, strokeLinecap: "round" };
const fill: IconProps = { fill: "currentColor" };

export const Home = (p: IconProps) => (
  <svg viewBox="0 0 24 24" {...stroke} {...p}>
    <path d="M4 11 L12 4 L20 11 V20 H14 V14 H10 V20 H4 Z" />
  </svg>
);

export const Agents = (p: IconProps) => (
  <svg viewBox="0 0 24 24" {...stroke} {...p}>
    <rect x="4" y="7" width="16" height="12" rx="2" />
    <path d="M9 12 V14 M15 12 V14" />
    <path d="M12 4 V7" />
    <circle cx="12" cy="4" r="1" fill="currentColor" />
  </svg>
);

export const Skills = (p: IconProps) => (
  <svg viewBox="0 0 24 24" {...stroke} {...p}>
    <path d="M12 3 L13.6 9.4 L20 11 L13.6 12.6 L12 19 L10.4 12.6 L4 11 L10.4 9.4 Z" />
  </svg>
);

export const Mcp = (p: IconProps) => (
  <svg viewBox="0 0 24 24" {...stroke} {...p}>
    <path d="M9 4 V8 M15 4 V8" />
    <rect x="6" y="8" width="12" height="6" />
    <path d="M12 14 V19" />
    <path d="M9 19 H15" />
  </svg>
);

export const Workflows = (p: IconProps) => (
  <svg viewBox="0 0 24 24" {...stroke} {...p}>
    <rect x="3" y="4" width="6" height="6" />
    <rect x="15" y="4" width="6" height="6" />
    <rect x="9" y="14" width="6" height="6" />
    <path d="M6 10 V14 H12 M18 10 V14 H12" />
  </svg>
);

export const Approvals = (p: IconProps) => (
  <svg viewBox="0 0 24 24" {...stroke} {...p}>
    <path d="M4 6 L11 13 L20 4" />
    <path d="M20 12 V18 H4 V8" />
  </svg>
);

export const Specs = (p: IconProps) => (
  <svg viewBox="0 0 24 24" {...stroke} {...p}>
    <path d="M6 3 H15 L19 7 V21 H6 Z" />
    <path d="M15 3 V7 H19" />
    <path d="M9 12 H16 M9 16 H14" />
  </svg>
);

export const Policy = (p: IconProps) => (
  <svg viewBox="0 0 24 24" {...stroke} {...p}>
    <path d="M12 3 L19 6 V12 C19 17 12 21 12 21 C12 21 5 17 5 12 V6 Z" />
    <path d="M9 12 L11 14 L15 10" />
  </svg>
);

export const Audit = (p: IconProps) => (
  <svg viewBox="0 0 24 24" {...stroke} {...p}>
    <circle cx="11" cy="11" r="6" />
    <path d="M15.5 15.5 L20 20" />
    <path d="M8 11 L11 14 L15 9" />
  </svg>
);

export const Obs = (p: IconProps) => (
  <svg viewBox="0 0 24 24" {...stroke} {...p}>
    <path d="M3 18 L8 12 L12 16 L16 8 L21 14" />
    <path d="M3 21 H21" />
  </svg>
);

export const Settings = (p: IconProps) => (
  <svg viewBox="0 0 24 24" {...stroke} {...p}>
    <circle cx="12" cy="12" r="3" />
    <path d="M12 3 V5 M12 19 V21 M3 12 H5 M19 12 H21 M5.6 5.6 L7 7 M17 17 L18.4 18.4 M5.6 18.4 L7 17 M17 7 L18.4 5.6" />
  </svg>
);

export const Search = (p: IconProps) => (
  <svg viewBox="0 0 24 24" {...stroke} {...p}>
    <circle cx="11" cy="11" r="6" />
    <path d="M16 16 L21 21" />
  </svg>
);

export const Bell = (p: IconProps) => (
  <svg viewBox="0 0 24 24" {...stroke} {...p}>
    <path d="M6 16 H18 L17 14 V10 C17 7 15 5 12 5 C9 5 7 7 7 10 V14 Z" />
    <path d="M10 19 C10 20 11 20.5 12 20.5 C13 20.5 14 20 14 19" />
  </svg>
);

export const Sun = (p: IconProps) => (
  <svg viewBox="0 0 24 24" {...round} {...p}>
    <circle cx="12" cy="12" r="4" />
    <path d="M12 2 V4 M12 20 V22 M4 12 H2 M22 12 H20 M5 5 L6.5 6.5 M17.5 17.5 L19 19 M5 19 L6.5 17.5 M17.5 6.5 L19 5" />
  </svg>
);

export const Moon = (p: IconProps) => (
  <svg viewBox="0 0 24 24" {...round} strokeLinejoin="round" {...p}>
    <path d="M20 14.5 A8 8 0 0 1 9.5 4 A8 8 0 1 0 20 14.5 Z" />
  </svg>
);

export const Monitor = (p: IconProps) => (
  <svg viewBox="0 0 24 24" {...stroke} {...p}>
    <rect x="3" y="4" width="18" height="13" rx="1.5" />
    <path d="M8 21 H16 M12 17 V21" />
  </svg>
);

export const Check = (p: IconProps) => (
  <svg viewBox="0 0 24 24" {...stroke} strokeWidth={2} {...p}>
    <path d="M4 12 L10 18 L20 6" />
  </svg>
);

export const X = (p: IconProps) => (
  <svg viewBox="0 0 24 24" {...stroke} {...p}>
    <path d="M6 6 L18 18 M18 6 L6 18" />
  </svg>
);

export const Chev = (p: IconProps) => (
  <svg viewBox="0 0 24 24" {...stroke} {...p}>
    <path d="M9 6 L15 12 L9 18" />
  </svg>
);

export const ChevDown = (p: IconProps) => (
  <svg viewBox="0 0 24 24" {...stroke} {...p}>
    <path d="M6 9 L12 15 L18 9" />
  </svg>
);

export const Plus = (p: IconProps) => (
  <svg viewBox="0 0 24 24" {...stroke} {...p}>
    <path d="M12 5 V19 M5 12 H19" />
  </svg>
);

export const Filter = (p: IconProps) => (
  <svg viewBox="0 0 24 24" {...stroke} {...p}>
    <path d="M4 5 H20 L14 12 V19 L10 17 V12 Z" />
  </svg>
);

export const Refresh = (p: IconProps) => (
  <svg viewBox="0 0 24 24" {...stroke} {...p}>
    <path d="M4 12 A8 8 0 0 1 18 7" />
    <path d="M14 7 H18 V3" />
    <path d="M20 12 A8 8 0 0 1 6 17" />
    <path d="M10 17 H6 V21" />
  </svg>
);

export const Clock = (p: IconProps) => (
  <svg viewBox="0 0 24 24" {...stroke} {...p}>
    <circle cx="12" cy="12" r="8" />
    <path d="M12 7 V12 L15 14" />
  </svg>
);

export const Branch = (p: IconProps) => (
  <svg viewBox="0 0 24 24" {...stroke} {...p}>
    <circle cx="6" cy="6" r="2" />
    <circle cx="18" cy="6" r="2" />
    <circle cx="6" cy="18" r="2" />
    <path d="M6 8 V16" />
    <path d="M18 8 C18 12 14 12 6 12" />
  </svg>
);

export const Play = (p: IconProps) => (
  <svg viewBox="0 0 24 24" {...fill} {...p}>
    <path d="M7 5 L19 12 L7 19 Z" />
  </svg>
);

export const External = (p: IconProps) => (
  <svg viewBox="0 0 24 24" {...stroke} {...p}>
    <path d="M14 4 H20 V10 M20 4 L11 13 M16 14 V20 H4 V8 H10" />
  </svg>
);

export const Copy = (p: IconProps) => (
  <svg viewBox="0 0 24 24" {...stroke} {...p}>
    <rect x="8" y="4" width="12" height="14" rx="1" />
    <path d="M16 18 V20 H4 V8 H6" />
  </svg>
);

export const Bolt = (p: IconProps) => (
  <svg viewBox="0 0 24 24" {...stroke} {...p}>
    <path d="M13 3 L5 14 H11 L10 21 L18 10 H12 Z" />
  </svg>
);

export const Globe = (p: IconProps) => (
  <svg viewBox="0 0 24 24" {...stroke} {...p}>
    <circle cx="12" cy="12" r="8" />
    <path d="M4 12 H20" />
    <path d="M12 4 C15 7 15 17 12 20 C9 17 9 7 12 4 Z" />
  </svg>
);

export const More = (p: IconProps) => (
  <svg viewBox="0 0 24 24" {...fill} {...p}>
    <circle cx="6" cy="12" r="1.5" />
    <circle cx="12" cy="12" r="1.5" />
    <circle cx="18" cy="12" r="1.5" />
  </svg>
);

export const Github = (p: IconProps) => (
  <svg viewBox="0 0 24 24" {...fill} {...p}>
    <path d="M12 2 C6.5 2 2 6.5 2 12 c0 4.4 2.9 8.2 6.8 9.5 c.5 .1 .7 -.2 .7 -.5 v-1.8 c-2.8 .6 -3.4 -1.3 -3.4 -1.3 c-.5 -1.2 -1.1 -1.5 -1.1 -1.5 c-.9 -.6 .1 -.6 .1 -.6 c1 .1 1.5 1 1.5 1 c.9 1.5 2.3 1.1 2.9 .8 c.1 -.6 .3 -1.1 .6 -1.3 c-2.2 -.3 -4.6 -1.1 -4.6 -5 c0 -1.1 .4 -2 1 -2.7 c-.1 -.3 -.4 -1.3 .1 -2.7 c0 0 .8 -.3 2.8 1 c.8 -.2 1.7 -.3 2.5 -.3 c.8 0 1.7 .1 2.5 .3 c1.9 -1.3 2.8 -1 2.8 -1 c.5 1.4 .2 2.4 .1 2.7 c.6 .7 1 1.6 1 2.7 c0 3.9 -2.3 4.7 -4.6 5 c.4 .3 .7 .9 .7 1.9 v2.8 c0 .3 .2 .6 .7 .5 C19.1 20.2 22 16.4 22 12 C22 6.5 17.5 2 12 2 z" />
  </svg>
);

export const Spark = (p: IconProps) => (
  <svg viewBox="0 0 24 24" {...stroke} {...p}>
    <path d="M12 3 L13.6 9.4 L20 11 L13.6 12.6 L12 19 L10.4 12.6 L4 11 L10.4 9.4 Z" />
  </svg>
);

export const Shield = (p: IconProps) => (
  <svg viewBox="0 0 24 24" {...stroke} {...p}>
    <path d="M12 3 L19 6 V12 C19 17 12 21 12 21 C12 21 5 17 5 12 V6 Z" />
  </svg>
);

export const Terminal = (p: IconProps) => (
  <svg viewBox="0 0 24 24" {...stroke} {...p}>
    <rect x="3" y="4" width="18" height="16" rx="1.5" />
    <path d="M7 9 L10 12 L7 15 M12 15 H16" />
  </svg>
);

export const Diamond = (p: IconProps) => (
  <svg viewBox="0 0 24 24" {...stroke} {...p}>
    <path d="M12 3 L21 12 L12 21 L3 12 Z" />
  </svg>
);

export const User = (p: IconProps) => (
  <svg viewBox="0 0 24 24" {...stroke} {...p}>
    <circle cx="12" cy="8" r="4" />
    <path d="M4 21 C4 16 8 14 12 14 C16 14 20 16 20 21" />
  </svg>
);

export const Arrow = (p: IconProps) => (
  <svg viewBox="0 0 24 24" {...stroke} {...p}>
    <path d="M5 12 H19 M13 6 L19 12 L13 18" />
  </svg>
);

// The Forge mark used in the sidebar brand block.
export const ForgeMark = (p: IconProps) => (
  <svg viewBox="0 0 32 32" fill="none" {...p}>
    <path d="M6 4 H24 L26 6 V10 H22 V8 H10 V14 H20 V18 H10 V28 H6 Z" fill="currentColor" />
    <circle cx="25" cy="25" r="2.4" fill="var(--primary)" />
  </svg>
);
