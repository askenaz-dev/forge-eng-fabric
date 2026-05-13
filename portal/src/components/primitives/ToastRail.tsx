"use client";

import { useToast } from "../providers/ToastProvider";
import { Check, X } from "../icons";
import { cx } from "./cx";
import Link from "next/link";

export function ToastRail() {
  const { toasts, remove } = useToast();
  if (toasts.length === 0) return null;
  return (
    <div className="toast-wrap" role="status" aria-live="polite">
      {toasts.map((t) => (
        <div
          key={t.id}
          className={cx(
            "toast",
            t.tone === "err" && "toast--err",
            t.tone === "warn" && "toast--warn",
          )}
        >
          {t.tone === "err" ? <X /> : <Check />}
          <span>{t.message}</span>
          {t.link && (
            <Link href={t.link.href} style={{ color: "var(--primary)", marginLeft: 6 }}>
              {t.link.label}
            </Link>
          )}
          <button
            type="button"
            aria-label="dismiss"
            onClick={() => remove(t.id)}
            style={{
              background: "transparent",
              border: 0,
              color: "var(--paper)",
              opacity: 0.6,
              cursor: "pointer",
              marginLeft: 4,
            }}
          >
            ×
          </button>
        </div>
      ))}
    </div>
  );
}
