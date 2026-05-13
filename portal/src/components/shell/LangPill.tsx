"use client";

import { useLang } from "../providers/LangProvider";
import { useToast } from "../providers/ToastProvider";
import { DICTIONARY } from "@/i18n/dictionary";

export function LangPill() {
  const { lang, setLang, t } = useLang();
  const toast = useToast();

  function pick(next: "es" | "en") {
    if (next === lang) return;
    setLang(next);
    toast.success(next === "es" ? DICTIONARY.es.toast_lang : DICTIONARY.en.toast_lang_en);
  }

  return (
    <div className="lang-pill" role="group" aria-label={t("tb_lang")}>
      <button aria-pressed={lang === "es"} onClick={() => pick("es")}>
        ES
      </button>
      <button aria-pressed={lang === "en"} onClick={() => pick("en")}>
        EN
      </button>
    </div>
  );
}
