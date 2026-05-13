"use client";

import {
  createContext,
  ReactNode,
  useCallback,
  useContext,
  useMemo,
  useRef,
  useState,
} from "react";

export type ToastTone = "default" | "success" | "info" | "warn" | "err";

export type Toast = {
  id: string;
  tone: ToastTone;
  message: string;
  link?: { href: string; label: string };
};

type ToastContextValue = {
  toasts: Toast[];
  push: (toast: Omit<Toast, "id">) => void;
  remove: (id: string) => void;
  success: (message: string, opts?: Partial<Omit<Toast, "id" | "tone" | "message">>) => void;
  info: (message: string, opts?: Partial<Omit<Toast, "id" | "tone" | "message">>) => void;
  warn: (message: string, opts?: Partial<Omit<Toast, "id" | "tone" | "message">>) => void;
  err: (message: string, opts?: Partial<Omit<Toast, "id" | "tone" | "message">>) => void;
};

const ToastContext = createContext<ToastContextValue | null>(null);

const DEDUPE_WINDOW_MS = 500;
const MAX_TOASTS = 5;
const TTL_MS = 3000;

export function ToastProvider({ children }: { children: ReactNode }) {
  const [toasts, setToasts] = useState<Toast[]>([]);
  const lastDedupeRef = useRef<Map<string, number>>(new Map());

  const remove = useCallback((id: string) => {
    setToasts((current) => current.filter((t) => t.id !== id));
  }, []);

  const push = useCallback(
    (toast: Omit<Toast, "id">) => {
      const dedupeKey = `${toast.tone}::${toast.message}::${toast.link?.href ?? ""}`;
      const now = Date.now();
      const last = lastDedupeRef.current.get(dedupeKey) ?? 0;
      if (now - last < DEDUPE_WINDOW_MS) return;
      lastDedupeRef.current.set(dedupeKey, now);

      const id = `toast_${now}_${Math.random().toString(36).slice(2, 8)}`;
      const entry: Toast = { id, ...toast };
      setToasts((current) => {
        const next = [...current, entry];
        return next.length > MAX_TOASTS ? next.slice(next.length - MAX_TOASTS) : next;
      });
      setTimeout(() => remove(id), TTL_MS);
    },
    [remove],
  );

  const value = useMemo<ToastContextValue>(() => {
    const tone =
      (kind: ToastTone) =>
      (message: string, opts?: Partial<Omit<Toast, "id" | "tone" | "message">>) =>
        push({ tone: kind, message, ...opts });
    return {
      toasts,
      push,
      remove,
      success: tone("success"),
      info: tone("info"),
      warn: tone("warn"),
      err: tone("err"),
    };
  }, [toasts, push, remove]);

  return <ToastContext.Provider value={value}>{children}</ToastContext.Provider>;
}

export function useToast(): ToastContextValue {
  const ctx = useContext(ToastContext);
  if (!ctx) throw new Error("useToast must be used inside a <ToastProvider>");
  return ctx;
}
