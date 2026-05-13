"use client";

import { ReactNode, useEffect, useState } from "react";
import { useSession } from "next-auth/react";
import { Sidebar, useStickyCollapse } from "./Sidebar";
import { TopBar } from "./TopBar";
import { NavigationProgress } from "./NavigationProgress";
import { ToastRail } from "../primitives/ToastRail";
import { CommandPalette } from "../palette/CommandPalette";
import { useLang } from "../providers/LangProvider";
import { cx } from "../primitives/cx";

type Counts = { agents: number; skills: number; mcp: number; approvals: number };

const EMPTY_COUNTS: Counts = { agents: 0, skills: 0, mcp: 0, approvals: 0 };

export function PortalShell({
  tenantSlug,
  workspaceLabel,
  githubHref,
  initialPermissions,
  initialCounts = EMPTY_COUNTS,
  children,
}: {
  tenantSlug: string;
  workspaceLabel: string;
  githubHref?: string;
  initialPermissions: string[];
  initialCounts?: Counts;
  children: ReactNode;
}) {
  const { t } = useLang();
  const [counts, setCounts] = useState<Counts>(initialCounts);
  const [permissions, setPermissions] = useState<Set<string>>(() => new Set(initialPermissions));
  const { status } = useSession();
  const { collapsed, toggle } = useStickyCollapse();

  useEffect(() => {
    if (status !== "authenticated") return;
    let cancelled = false;

    fetch("/api/sidebar/counts", { cache: "no-store" })
      .then((r) => (r.ok ? r.json() : null))
      .then((data: Counts | null) => {
        if (!cancelled && data) setCounts(data);
      })
      .catch(() => undefined);

    fetch("/api/permissions/me", { cache: "no-store" })
      .then((r) => (r.ok ? r.json() : null))
      .then((data: { permissions?: string[] } | null) => {
        if (!cancelled && data?.permissions) setPermissions(new Set(data.permissions));
      })
      .catch(() => undefined);

    return () => {
      cancelled = true;
    };
  }, [status]);

  return (
    <>
      <NavigationProgress />
      <div className={cx("app", collapsed && "app--collapsed")}>
        <Sidebar
          tenantSlug={tenantSlug}
          counts={counts}
          permissions={permissions}
          collapsed={collapsed}
          onToggleCollapse={toggle}
        />
        <TopBar workspaceLabel={workspaceLabel} githubHref={githubHref} />
        <main className="main">
          <div className="main-inner">{children}</div>
        </main>
      </div>
      <ToastRail />
      <CommandPalette />
      <noscript>
        <div style={{ padding: 16, fontFamily: "var(--f-mono)", fontSize: 12 }}>
          {t("min_width_notice")}
        </div>
      </noscript>
    </>
  );
}
