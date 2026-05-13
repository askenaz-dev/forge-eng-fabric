import { cookies } from "next/headers";

export type ThemePref = "light" | "dark" | "system";
export type Density = "compact" | "comfortable" | "spacious";
export type Lang = "es" | "en";

export type Preferences = {
  theme: ThemePref;
  density: Density;
  lang: Lang;
  sidebarCollapsed: boolean;
};

export const DEFAULT_PREFS: Preferences = {
  theme: "system",
  density: "comfortable",
  lang: "es",
  sidebarCollapsed: false,
};

const COOKIE_NAME = "forge_prefs";

function parse(raw: string | undefined): Preferences {
  if (!raw) return DEFAULT_PREFS;
  try {
    const parsed = JSON.parse(raw);
    const theme: ThemePref = ["light", "dark", "system"].includes(parsed.theme) ? parsed.theme : DEFAULT_PREFS.theme;
    const density: Density = ["compact", "comfortable", "spacious"].includes(parsed.density) ? parsed.density : DEFAULT_PREFS.density;
    const lang: Lang = parsed.lang === "en" ? "en" : "es";
    const sidebarCollapsed = parsed.sidebarCollapsed === true;
    return { theme, density, lang, sidebarCollapsed };
  } catch {
    return DEFAULT_PREFS;
  }
}

export function readPreferences(): Preferences {
  return parse(cookies().get(COOKIE_NAME)?.value);
}

export function preferencesCookieName(): string {
  return COOKIE_NAME;
}

// Initial-theme resolution used by layout.tsx to set the data-theme attribute
// on <html> before client hydration. If theme is "system", we cannot read
// prefers-color-scheme on the server, so we render the document with no
// data-theme attribute and let the client provider apply the resolved theme
// in an effect (`useTheme()`), guarded by data-theme-changing.
export function initialDataTheme(prefs: Preferences): "light" | "dark" | undefined {
  if (prefs.theme === "light") return "light";
  if (prefs.theme === "dark") return "dark";
  return undefined;
}
