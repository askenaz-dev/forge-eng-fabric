import { cx } from "./cx";

export type PulseTone = "ok" | "warn" | "err" | "pending" | "queued";

const TONE_CLASS: Record<PulseTone, string> = {
  ok:      "st--ok",
  warn:    "st--warn",
  err:     "st--err",
  pending: "st--pending",
  queued:  "st--queued",
};

export function PulseDot({ tone, className }: { tone: PulseTone; className?: string }) {
  return <span className={cx("st", TONE_CLASS[tone], className)} aria-hidden="true" />;
}
