import type { Lang } from "@/lib/prefs";

const numberCache = new Map<string, Intl.NumberFormat>();
const dateCache = new Map<string, Intl.DateTimeFormat>();
const relativeCache = new Map<string, Intl.RelativeTimeFormat>();

function getNumberFormatter(lang: Lang, opts?: Intl.NumberFormatOptions): Intl.NumberFormat {
  const key = `${lang}::${JSON.stringify(opts ?? {})}`;
  let fmt = numberCache.get(key);
  if (!fmt) {
    fmt = new Intl.NumberFormat(lang, opts);
    numberCache.set(key, fmt);
  }
  return fmt;
}

function getDateFormatter(lang: Lang, opts: Intl.DateTimeFormatOptions): Intl.DateTimeFormat {
  const key = `${lang}::${JSON.stringify(opts)}`;
  let fmt = dateCache.get(key);
  if (!fmt) {
    fmt = new Intl.DateTimeFormat(lang, opts);
    dateCache.set(key, fmt);
  }
  return fmt;
}

function getRelativeFormatter(lang: Lang): Intl.RelativeTimeFormat {
  let fmt = relativeCache.get(lang);
  if (!fmt) {
    fmt = new Intl.RelativeTimeFormat(lang, { numeric: "auto", style: "short" });
    relativeCache.set(lang, fmt);
  }
  return fmt;
}

export function formatNumber(lang: Lang, value: number, opts?: Intl.NumberFormatOptions): string {
  return getNumberFormatter(lang, opts).format(value);
}

export function formatDate(
  lang: Lang,
  ts: number | string | Date,
  opts: Intl.DateTimeFormatOptions = { dateStyle: "medium", timeStyle: "short" },
): string {
  const date = ts instanceof Date ? ts : new Date(ts);
  return getDateFormatter(lang, opts).format(date);
}

const RELATIVE_THRESHOLDS: Array<{ unit: Intl.RelativeTimeFormatUnit; ms: number }> = [
  { unit: "year",   ms: 365 * 24 * 60 * 60 * 1000 },
  { unit: "month",  ms: 30 * 24 * 60 * 60 * 1000 },
  { unit: "week",   ms: 7 * 24 * 60 * 60 * 1000 },
  { unit: "day",    ms: 24 * 60 * 60 * 1000 },
  { unit: "hour",   ms: 60 * 60 * 1000 },
  { unit: "minute", ms: 60 * 1000 },
  { unit: "second", ms: 1000 },
];

export function formatRelativeTime(lang: Lang, ts: number | string | Date, now: number = Date.now()): string {
  const date = ts instanceof Date ? ts : new Date(ts);
  const diff = date.getTime() - now;
  const abs = Math.abs(diff);
  for (const { unit, ms } of RELATIVE_THRESHOLDS) {
    if (abs >= ms || unit === "second") {
      const value = Math.round(diff / ms);
      return getRelativeFormatter(lang).format(value, unit);
    }
  }
  return "";
}

export function formatDuration(totalSeconds: number): string {
  const safe = Math.max(0, Math.floor(totalSeconds));
  const mm = Math.floor(safe / 60);
  const ss = safe % 60;
  return `${String(mm).padStart(2, "0")}:${String(ss).padStart(2, "0")}`;
}
