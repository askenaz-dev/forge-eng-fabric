// Self-hosted Forge brand families.
//
// Mechanics:
//   - Instrument Serif & JetBrains Mono come from `next/font/google`, which
//     fetches WOFF2 at *build time* and inlines them into `.next/static/media/`.
//     End-user browsers never reach `fonts.googleapis.com` or `fonts.gstatic.com`.
//   - Geist comes from the official `geist` npm package by Vercel — same
//     next/font integration, self-hosted out of the box.
//
// This satisfies the design-system spec's "self-hosted typography" requirement:
// zero CDN traffic at runtime.

import { GeistSans } from "geist/font/sans";
import { Instrument_Serif, JetBrains_Mono } from "next/font/google";

export const instrumentSerif = Instrument_Serif({
  subsets: ["latin"],
  weight: ["400"],
  style: ["normal", "italic"],
  variable: "--f-display-loaded",
  display: "swap",
  preload: true,
  fallback: ["Cormorant Garamond", "Georgia", "serif"],
});

export const geist = GeistSans;

export const jetbrainsMono = JetBrains_Mono({
  subsets: ["latin"],
  weight: ["400"],
  style: ["normal", "italic"],
  variable: "--f-mono-loaded",
  display: "swap",
  preload: false,
  fallback: ["SF Mono", "ui-monospace", "Menlo", "monospace"],
});

export const fontClassNames = `${instrumentSerif.variable} ${geist.variable} ${jetbrainsMono.variable}`;
