"use client";

import { useEffect, useState } from "react";
import { useLang } from "@/components/providers/LangProvider";

type AlfredAdminState = {
  workspace_id: string;
  dock_enabled: boolean;
};

// Admin surface for the per-workspace `alfred.dock_enabled` flag (10.3).
// Reuses the autonomy-preset admin page surface so workspace owners can
// flip the dock on without leaving the admin section.
export default function AlfredAdminPage() {
  const { t } = useLang();
  const [state, setState] = useState<AlfredAdminState | null>(null);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    fetch("/api/admin/alfred/settings", { cache: "no-store" })
      .then((r) => r.json())
      .then((data: AlfredAdminState) => setState(data))
      .catch((exc: Error) => setError(exc.message));
  }, []);

  async function toggle() {
    if (!state) return;
    setSaving(true);
    setError(null);
    try {
      const r = await fetch("/api/admin/alfred/settings", {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({ dock_enabled: !state.dock_enabled }),
      });
      if (!r.ok) throw new Error(`http ${r.status}`);
      const next = (await r.json()) as AlfredAdminState;
      setState(next);
    } catch (exc) {
      setError((exc as Error).message);
    } finally {
      setSaving(false);
    }
  }

  return (
    <main className="page">
      <header className="page-header">
        <h1 className="page-title">Alfred — administration</h1>
        <p className="page-lede">
          {t("alfred_admin_lede")}
        </p>
      </header>

      <section className="card" style={{ padding: "var(--s-4)" }}>
        <h2 style={{ fontFamily: "var(--f-display)", marginTop: 0 }}>Dock</h2>
        <p style={{ color: "var(--fg-2)" }}>
          {t("alfred_admin_dock_help")}
        </p>
        <label style={{ display: "inline-flex", gap: "var(--s-2)", alignItems: "center" }}>
          <input
            type="checkbox"
            checked={state?.dock_enabled ?? false}
            disabled={!state || saving}
            onChange={toggle}
          />
          <span>{t("alfred_admin_dock_toggle")}</span>
        </label>
        {error && <p style={{ color: "var(--rust)", marginTop: "var(--s-3)" }}>{error}</p>}
      </section>
    </main>
  );
}
