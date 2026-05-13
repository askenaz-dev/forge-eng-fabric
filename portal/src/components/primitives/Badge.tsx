import { HTMLAttributes, ReactNode } from "react";
import { cx } from "./cx";

export type BadgeTone = "default" | "ok" | "warn" | "err" | "ember" | "info" | "steel";

export type BadgeProps = HTMLAttributes<HTMLSpanElement> & {
  tone?: BadgeTone;
  dot?: boolean;
  leading?: ReactNode;
};

const TONE_CLASS: Record<BadgeTone, string | null> = {
  default: null,
  ok:    "badge--ok",
  warn:  "badge--warn",
  err:   "badge--err",
  ember: "badge--ember",
  info:  "badge--info",
  steel: "badge--steel",
};

export function Badge({ tone = "default", dot = false, leading, className, children, ...rest }: BadgeProps) {
  return (
    <span className={cx("badge", TONE_CLASS[tone], className)} {...rest}>
      {dot && <span className="dot" />}
      {leading}
      {children}
    </span>
  );
}
