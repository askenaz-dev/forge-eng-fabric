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
import type { Lang } from "@/lib/prefs";
import { DictKey, getDictionary, tFmt } from "@/i18n/dictionary";

type Translator = (key: DictKey, vars?: Record<string, string | number>) => string;

type LangContextValue = {
  lang: Lang;
  setLang: (next: Lang) => void;
  t: Translator;
};

const LangContext = createContext<LangContextValue | null>(null);

export function LangProvider({
  initialLang,
  children,
}: {
  initialLang: Lang;
  children: ReactNode;
}) {
  const [lang, setLangState] = useState<Lang>(initialLang);

  useEffect(() => {
    document.documentElement.lang = lang;
    try {
      localStorage.setItem("forge_lang", lang);
    } catch {
      // ignored
    }
  }, [lang]);

  const setLang = useCallback((next: Lang) => {
    setLangState(next);
    fetch("/api/i18n/preference", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ lang: next }),
      keepalive: true,
    }).catch(() => undefined);
  }, []);

  const t = useMemo<Translator>(() => {
    const dict = getDictionary(lang);
    return (key, vars) => {
      const raw = dict[key] ?? key;
      return vars ? tFmt(raw, vars) : raw;
    };
  }, [lang]);

  const value = useMemo<LangContextValue>(() => ({ lang, setLang, t }), [lang, setLang, t]);

  return <LangContext.Provider value={value}>{children}</LangContext.Provider>;
}

export function useLang(): LangContextValue {
  const ctx = useContext(LangContext);
  if (!ctx) throw new Error("useLang must be used inside a <LangProvider>");
  return ctx;
}
