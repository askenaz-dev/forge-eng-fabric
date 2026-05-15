/**
 * Alfred Console entry point (alfred-console-redesign §1.1).
 *
 * Routes to Friendly or Advanced view based on:
 *   1. ?view=friendly|advanced query param (session override)
 *   2. user.console_view_preference (persisted)
 *   3. Role-based default on first sign-in (friendly for workspace.member, advanced for workspace.developer+)
 *
 * Hosts the MatchDialog so it can be triggered from either view.
 */

"use client";

import { useEffect, useState, useCallback } from "react";
import { useSearchParams, useRouter } from "next/navigation";
import { FriendlyView } from "@/components/alfred/FriendlyView";
import { AdvancedView } from "@/components/alfred/AdvancedView";
import { MatchDialog, type SpecMatch } from "@/components/alfred/MatchDialog";
import type { AppEntry } from "@/components/alfred/AppSwitcher";

type ConsoleView = "friendly" | "advanced";

export default function AlfredPage() {
  const searchParams = useSearchParams();
  const router = useRouter();
  const [view, setView] = useState<ConsoleView | null>(null);
  const [apps, setApps] = useState<AppEntry[]>([]);
  const [activeAppId, setActiveAppId] = useState<string | null>(null);
  const [workspaceId, setWorkspaceId] = useState<string>("");
  const [specMatch, setSpecMatch] = useState<SpecMatch | null>(null);

  // Resolve view: query param → persisted preference → role-based default.
  useEffect(() => {
    const qp = searchParams.get("view") as ConsoleView | null;
    if (qp === "friendly" || qp === "advanced") {
      setView(qp);
      return;
    }
    void fetch("/api/user/preferences", { cache: "no-store" })
      .then((r) => r.ok ? r.json() : { console_view_preference: null })
      .then((data: { console_view_preference: ConsoleView | null }) => {
        if (data.console_view_preference) {
          setView(data.console_view_preference);
        } else {
          // First sign-in: persist role-based default (§3.3).
          void resolveAndPersistDefault();
        }
      })
      .catch(() => setView("friendly"));
  }, [searchParams]);

  async function resolveAndPersistDefault() {
    try {
      // Fetch the user's workspace roles to determine the default.
      const r = await fetch("/api/permissions/me", { cache: "no-store" });
      const perms = r.ok ? await r.json() : {};
      const roles: string[] = perms.roles ?? perms.permissions ?? [];
      const isDev = roles.some((r) =>
        ["workspace.developer", "workspace.admin", "platform-admin"].includes(r),
      );
      const resolved: ConsoleView = isDev ? "advanced" : "friendly";
      setView(resolved);
      // Persist (§3.3).
      await fetch("/api/user/preferences", {
        method: "PUT",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({ console_view_preference: resolved }),
      });
    } catch {
      setView("friendly");
    }
  }

  // Load workspace context.
  useEffect(() => {
    void fetch("/api/workspace/active", { cache: "no-store" })
      .then((r) => r.ok ? r.json() : null)
      .then((data: { workspace_id?: string; apps?: AppEntry[] } | null) => {
        if (data?.workspace_id) setWorkspaceId(data.workspace_id);
        if (data?.apps?.length) {
          setApps(data.apps.filter((a) => !a.slug.startsWith("_")));
          setActiveAppId(data.apps[0]?.id ?? null);
        }
      })
      .catch(() => undefined);
  }, []);

  // Listen for spec_match events fired by FriendlyView / AdvancedView.
  useEffect(() => {
    function handleMatch(e: Event) {
      const match = (e as CustomEvent<SpecMatch>).detail;
      setSpecMatch(match);
    }
    window.addEventListener("alfred:spec_match", handleMatch);
    return () => window.removeEventListener("alfred:spec_match", handleMatch);
  }, []);

  const switchToAdvanced = useCallback(() => {
    setView("advanced");
    // Session-level switch — does not persist unless user clicks "Save as default".
    void fetch("/api/user/preferences", {
      method: "PUT",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ console_view_preference: "advanced", session_only: true }),
    });
    router.replace("/alfred?view=advanced");
  }, [router]);

  const switchToFriendly = useCallback(() => {
    setView("friendly");
    void fetch("/api/user/preferences", {
      method: "PUT",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ console_view_preference: "friendly", session_only: true }),
    });
    router.replace("/alfred?view=friendly");
  }, [router]);

  if (!view) {
    return (
      <div className="page-loading" aria-busy="true" aria-label="Loading Alfred console…">
        <div className="h-eyebrow">Alfred</div>
      </div>
    );
  }

  return (
    <>
      {view === "friendly" ? (
        <FriendlyView
          apps={apps}
          activeAppId={activeAppId}
          workspaceId={workspaceId}
          onSwitchView={switchToAdvanced}
        />
      ) : (
        <AdvancedView
          workspaceId={workspaceId}
          apps={apps}
          activeAppId={activeAppId}
          onSwitchView={switchToFriendly}
        />
      )}

      {/* Match dialog rendered at the page level so it's accessible from both views */}
      {specMatch && (
        <MatchDialog
          match={specMatch}
          workspaceId={workspaceId}
          view={view}
          onDismiss={() => setSpecMatch(null)}
        />
      )}
    </>
  );
}
