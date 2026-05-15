"use client";

// AppPicker — global topbar control for picking the active App in the
// current workspace. Renders the workspace's Apps with the system-managed
// `_unassigned` bucket pinned to the bottom (see app-first-class-entity 10.3).
// When the workspace has only `_unassigned`, the picker shows an empty-state
// CTA pointing at /workspaces/{ws}/apps/new.

import { useState } from "react";
import Link from "next/link";

export type AppSummary = {
  id: string;
  slug: string;
  name: string;
  system_managed: boolean;
  lifecycle_state: "active" | "archived" | "deleted";
};

export function AppPicker({
  workspaceId,
  workspaceSlug,
  apps,
  activeAppId,
}: {
  workspaceId: string;
  workspaceSlug: string;
  apps: AppSummary[];
  activeAppId?: string;
}) {
  const [open, setOpen] = useState(false);
  // System-managed `_unassigned` always renders last with a distinct accent.
  const sorted = [...apps].sort((a, b) => {
    if (a.system_managed !== b.system_managed) return a.system_managed ? 1 : -1;
    return a.name.localeCompare(b.name);
  });
  const realApps = sorted.filter((app) => !app.system_managed);
  const active = sorted.find((app) => app.id === activeAppId);
  const emptyState = realApps.length === 0;

  return (
    <div style={{ position: "relative" }}>
      <button
        type="button"
        onClick={() => setOpen((value) => !value)}
        className="app-picker-trigger"
        aria-haspopup="listbox"
        aria-expanded={open}
        style={{
          display: "inline-flex",
          alignItems: "center",
          gap: 6,
          padding: "6px 10px",
          borderRadius: 6,
          border: "1px solid var(--bd-2)",
          background: "var(--bg-1)",
          fontSize: 13,
          fontFamily: "var(--f-mono)",
        }}
      >
        <span style={{ opacity: 0.6 }}>App:</span>
        <span>{active ? active.name : emptyState ? "(none)" : "Pick…"}</span>
      </button>
      {open && (
        <div
          role="listbox"
          className="app-picker-menu"
          style={{
            position: "absolute",
            top: "100%",
            right: 0,
            marginTop: 4,
            minWidth: 260,
            background: "var(--bg-1)",
            border: "1px solid var(--bd-2)",
            borderRadius: 6,
            padding: 4,
            boxShadow: "0 4px 12px rgba(0,0,0,0.08)",
            zIndex: 50,
          }}
        >
          {emptyState && (
            <div style={{ padding: "10px 12px", fontSize: 12, color: "var(--fg-3)" }}>
              No Apps in this workspace yet. The platform created the
              <code style={{ margin: "0 4px" }}>_unassigned</code> bucket for
              you;{" "}
              <Link href={`/workspaces/${workspaceSlug}/apps/new`} style={{ color: "var(--primary)" }}>
                create a real one
              </Link>{" "}
              to anchor specs.
            </div>
          )}
          <ul style={{ listStyle: "none", margin: 0, padding: 0 }}>
            {sorted.map((app) => (
              <li key={app.id}>
                <Link
                  href={`/workspaces/${workspaceSlug}/apps/${app.slug}`}
                  className="app-picker-row"
                  style={{
                    display: "flex",
                    alignItems: "center",
                    justifyContent: "space-between",
                    padding: "6px 10px",
                    borderRadius: 4,
                    color: app.system_managed ? "var(--fg-3)" : "var(--fg-1)",
                    fontStyle: app.system_managed ? "italic" : "normal",
                    background: app.id === activeAppId ? "var(--bg-hover)" : "transparent",
                  }}
                >
                  <span>{app.name}</span>
                  <code style={{ fontSize: 10, opacity: 0.6 }}>{app.slug}</code>
                </Link>
              </li>
            ))}
          </ul>
          <Link
            href={`/workspaces/${workspaceSlug}/apps/new`}
            style={{
              display: "block",
              marginTop: 4,
              padding: "8px 10px",
              fontSize: 13,
              color: "var(--primary)",
              borderTop: "1px solid var(--bd-3)",
            }}
          >
            + New App…
          </Link>
        </div>
      )}
    </div>
  );
}
