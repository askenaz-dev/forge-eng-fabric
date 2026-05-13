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
import type { Density } from "@/lib/prefs";

type DensityContextValue = {
  density: Density;
  setDensity: (next: Density) => void;
};

const DensityContext = createContext<DensityContextValue | null>(null);

export function DensityProvider({
  initialDensity,
  children,
}: {
  initialDensity: Density;
  children: ReactNode;
}) {
  const [density, setDensityState] = useState<Density>(initialDensity);

  useEffect(() => {
    document.documentElement.setAttribute("data-density", density);
    try {
      localStorage.setItem("forge_density", density);
    } catch {
      // ignored — see ThemeProvider for rationale
    }
  }, [density]);

  const setDensity = useCallback((next: Density) => {
    setDensityState(next);
    fetch("/api/density/preference", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ density: next }),
      keepalive: true,
    }).catch(() => undefined);
  }, []);

  const value = useMemo<DensityContextValue>(
    () => ({ density, setDensity }),
    [density, setDensity],
  );

  return <DensityContext.Provider value={value}>{children}</DensityContext.Provider>;
}

export function useDensity(): DensityContextValue {
  const ctx = useContext(DensityContext);
  if (!ctx) throw new Error("useDensity must be used inside a <DensityProvider>");
  return ctx;
}
