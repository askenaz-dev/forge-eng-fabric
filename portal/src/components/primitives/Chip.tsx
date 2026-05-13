"use client";

import { ButtonHTMLAttributes, ReactNode } from "react";
import { cx } from "./cx";

export type ChipProps = Omit<ButtonHTMLAttributes<HTMLButtonElement>, "aria-pressed"> & {
  pressed?: boolean;
  count?: number | string;
};

export function Chip({ pressed = false, count, className, children, type, ...rest }: ChipProps) {
  return (
    <button
      type={type ?? "button"}
      aria-pressed={pressed}
      className={cx("chip", className)}
      {...rest}
    >
      {children}
      {count != null && (
        <span style={{ fontFamily: "var(--f-mono)", color: "var(--fg-3)" }}>{count}</span>
      )}
    </button>
  );
}

export function ChipRow({
  children,
  className,
  ...rest
}: React.HTMLAttributes<HTMLDivElement>) {
  return (
    <div className={cx("chips", className)} {...rest}>
      {children}
    </div>
  );
}
