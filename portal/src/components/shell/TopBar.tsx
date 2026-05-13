"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useEffect, useState } from "react";
import { Chev, Github, Search } from "../icons";
import { useLang } from "../providers/LangProvider";
import { useCommandPalette } from "../providers/CommandPaletteProvider";
import { findNavItem } from "./nav";
import { ThemeMenu } from "./ThemeMenu";
import { LangPill } from "./LangPill";
import { NotificationsButton } from "./NotificationsButton";

export function TopBar({
  workspaceLabel,
  githubHref,
}: {
  workspaceLabel: string;
  githubHref?: string;
}) {
  const { t } = useLang();
  const pathname = usePathname() ?? "/";
  const active = findNavItem(pathname);
  const cmd = useCommandPalette();
  const [kbd, setKbd] = useState("Ctrl K");

  useEffect(() => {
    if (typeof navigator !== "undefined" && /mac/i.test(navigator.platform)) {
      setKbd("⌘K");
    }
  }, []);

  const sectionHref = active ? active.href.split(/[?#]/)[0].replace(/\/$/, "") || "/" : "/";
  const currentPath = pathname.replace(/\/$/, "") || "/";
  const onSection = active ? currentPath === sectionHref : false;

  return (
    <header className="top" role="banner">
      <nav className="top-crumb" aria-label="Breadcrumb">
        <Link href="/" className="crumb-link">{workspaceLabel}</Link>
        <Chev className="sep" style={{ width: 12, height: 12 }} />
        {active ? (
          onSection ? (
            <b>{t(active.labelKey)}</b>
          ) : (
            <Link href={sectionHref} className="crumb-link">{t(active.labelKey)}</Link>
          )
        ) : (
          <b>—</b>
        )}
      </nav>

      <button
        type="button"
        className="top-search"
        onClick={() => cmd.show()}
        aria-label={t("tb_search")}
      >
        <Search />
        <span className="placeholder">{t("tb_search")}</span>
        <span className="kbd">{kbd}</span>
      </button>

      <div className="top-actions">
        <LangPill />
        <ThemeMenu />
        <NotificationsButton />
        {githubHref && (
          <a className="icon-btn" aria-label="GitHub" href={githubHref} target="_blank" rel="noreferrer">
            <Github />
          </a>
        )}
      </div>
    </header>
  );
}
