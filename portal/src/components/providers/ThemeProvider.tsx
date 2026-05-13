"use client";

import {
  createContext,
  ReactNode,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import type { ThemePref } from "@/lib/prefs";

type ThemeContextValue = {
  pref: ThemePref;
  effective: "light" | "dark";
  setPref: (next: ThemePref) => void;
};

const ThemeContext = createContext<ThemeContextValue | null>(null);

function readSystemDark(): boolean {
  if (typeof window === "undefined") return false;
  return window.matchMedia("(prefers-color-scheme: dark)").matches;
}

export function ThemeProvider({
  initialPref,
  children,
}: {
  initialPref: ThemePref;
  children: ReactNode;
}) {
  const [pref, setPrefState] = useState<ThemePref>(initialPref);
  const [systemDark, setSystemDark] = useState<boolean>(false);
  const firstRunRef = useRef(true);

  // Track OS preference whenever pref is "system".
  useEffect(() => {
    const mq = window.matchMedia("(prefers-color-scheme: dark)");
    const handler = (e: MediaQueryListEvent) => setSystemDark(e.matches);
    setSystemDark(mq.matches);
    mq.addEventListener?.("change", handler);
    return () => mq.removeEventListener?.("change", handler);
  }, []);

  const effective: "light" | "dark" = pref === "system" ? (systemDark ? "dark" : "light") : pref;

  // Apply data-theme + transition guard.
  useEffect(() => {
    const root = document.documentElement;
    if (!firstRunRef.current) {
      root.setAttribute("data-theme-changing", "");
    }
    root.setAttribute("data-theme", effective);
    try {
      localStorage.setItem("forge_theme", pref);
    } catch {
      // localStorage may be unavailable (e.g. private mode); we still set data-theme.
    }
    if (!firstRunRef.current) {
      requestAnimationFrame(() =>
        requestAnimationFrame(() => {
          root.removeAttribute("data-theme-changing");
        }),
      );
    }
    firstRunRef.current = false;
  }, [pref, effective]);

  const setPref = useCallback((next: ThemePref) => {
    setPrefState(next);
    // Persist server-side; failures are tolerated — local state still drives UI.
    fetch("/api/theme/preference", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ theme: next }),
      keepalive: true,
    }).catch(() => undefined);
  }, []);

  const value = useMemo<ThemeContextValue>(
    () => ({ pref, effective, setPref }),
    [pref, effective, setPref],
  );

  return <ThemeContext.Provider value={value}>{children}</ThemeContext.Provider>;
}

export function useTheme(): ThemeContextValue {
  const ctx = useContext(ThemeContext);
  if (!ctx) throw new Error("useTheme must be used inside a <ThemeProvider>");
  return ctx;
}
