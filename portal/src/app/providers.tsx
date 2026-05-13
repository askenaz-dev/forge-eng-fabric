"use client";

import { SessionProvider } from "next-auth/react";
import type { ReactNode } from "react";
import { ThemeProvider } from "@/components/providers/ThemeProvider";
import { DensityProvider } from "@/components/providers/DensityProvider";
import { LangProvider } from "@/components/providers/LangProvider";
import { ToastProvider } from "@/components/providers/ToastProvider";
import { CommandPaletteProvider } from "@/components/providers/CommandPaletteProvider";
import type { Density, Lang, ThemePref } from "@/lib/prefs";

export function Providers({
  initialTheme,
  initialDensity,
  initialLang,
  children,
}: {
  initialTheme: ThemePref;
  initialDensity: Density;
  initialLang: Lang;
  children: ReactNode;
}) {
  return (
    <SessionProvider>
      <ThemeProvider initialPref={initialTheme}>
        <DensityProvider initialDensity={initialDensity}>
          <LangProvider initialLang={initialLang}>
            <ToastProvider>
              <CommandPaletteProvider>{children}</CommandPaletteProvider>
            </ToastProvider>
          </LangProvider>
        </DensityProvider>
      </ThemeProvider>
    </SessionProvider>
  );
}
