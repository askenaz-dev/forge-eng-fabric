"use client";

import {
  createContext,
  ReactNode,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
} from "react";

type CommandPaletteContextValue = {
  open: boolean;
  show: () => void;
  hide: () => void;
  toggle: () => void;
};

const CommandPaletteContext = createContext<CommandPaletteContextValue | null>(null);

function isTypingInElement(el: EventTarget | null): boolean {
  if (!(el instanceof HTMLElement)) return false;
  const tag = el.tagName.toLowerCase();
  if (tag === "input" || tag === "textarea" || tag === "select") return true;
  if (el.isContentEditable) return true;
  return false;
}

export function CommandPaletteProvider({ children }: { children: ReactNode }) {
  const [open, setOpen] = useState(false);

  const show = useCallback(() => setOpen(true), []);
  const hide = useCallback(() => setOpen(false), []);
  const toggle = useCallback(() => setOpen((o) => !o), []);

  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      const isPaletteShortcut =
        (e.key === "k" || e.key === "K") && (e.metaKey || e.ctrlKey);
      const isSlashShortcut = e.key === "/" && !isTypingInElement(e.target);
      if (!isPaletteShortcut && !isSlashShortcut) return;
      // Allow ⌘K to fire from inputs only if the target is the top-bar search.
      if (isPaletteShortcut && isTypingInElement(e.target)) {
        const target = e.target as HTMLElement;
        if (!target.closest(".top-search")) return;
      }
      e.preventDefault();
      setOpen((o) => !o);
    }
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, []);

  const value = useMemo<CommandPaletteContextValue>(
    () => ({ open, show, hide, toggle }),
    [open, show, hide, toggle],
  );

  return <CommandPaletteContext.Provider value={value}>{children}</CommandPaletteContext.Provider>;
}

export function useCommandPalette(): CommandPaletteContextValue {
  const ctx = useContext(CommandPaletteContext);
  if (!ctx) throw new Error("useCommandPalette must be used inside a <CommandPaletteProvider>");
  return ctx;
}
