"use client";

import { ReactNode } from "react";

export type SegOption<T extends string> = {
  value: T;
  label: ReactNode;
  ariaLabel?: string;
};

export type SegProps<T extends string> = {
  value: T;
  options: SegOption<T>[];
  onChange: (next: T) => void;
  className?: string;
};

export function Seg<T extends string>({ value, options, onChange, className }: SegProps<T>) {
  return (
    <div className={`seg ${className ?? ""}`} role="group">
      {options.map((o) => (
        <button
          key={o.value}
          type="button"
          aria-pressed={value === o.value}
          aria-label={o.ariaLabel}
          onClick={() => onChange(o.value)}
        >
          {o.label}
        </button>
      ))}
    </div>
  );
}
