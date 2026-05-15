"use client";

/**
 * Alfred Advanced View (alfred-console-redesign §2).
 *
 * The existing slash-command console preserved with:
 * - An App picker in the top bar that scopes every subsequent command.
 * - view=advanced threaded into all Alfred dialogue calls.
 * - Updated keyboard shortcuts cheat sheet.
 */

import { useState } from "react";
import { useLang } from "@/components/providers/LangProvider";
import { ScopeSelect } from "@/components/scope/ScopeSelect";
import type { AppEntry } from "./AppSwitcher";

interface AdvancedViewProps {
  workspaceId: string;
  apps: AppEntry[];
  activeAppId: string | null;
  onSwitchView: () => void;
  onResult?: (result: string, sessionId: string) => void;
  onError?: (error: string) => void;
  initialResult?: string;
  initialSessionId?: string;
  initialError?: string;
}

const APP_PICKER_SENTINEL = "__all__";

export function AdvancedView({
  workspaceId,
  apps,
  activeAppId: initialAppId,
  onSwitchView,
  initialResult,
  initialSessionId,
  initialError,
}: AdvancedViewProps) {
  const { t } = useLang();
  const visible = apps.filter((a) => !a.slug.startsWith("_"));
  const [scopedAppId, setScopedAppId] = useState<string | null>(initialAppId);
  const [text, setText] = useState("");
  const [result, setResult] = useState<string | null>(initialResult ?? null);
  const [sessionId, setSessionId] = useState<string | null>(initialSessionId ?? null);
  const [error, setError] = useState<string | null>(initialError ?? null);
  const [loading, setLoading] = useState(false);
  const [showShortcuts, setShowShortcuts] = useState(false);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    if (!text.trim()) return;
    setLoading(true);
    setResult(null);
    setError(null);

    try {
      // Wire view=advanced into every Alfred dialogue call (2.3).
      const r = await fetch("/api/alfred/console", {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({
          workspace_id: workspaceId,
          app_id: scopedAppId && scopedAppId !== APP_PICKER_SENTINEL ? scopedAppId : undefined,
          message: text,
          view: "advanced",
        }),
      });
      if (!r.ok) throw new Error(await r.text());
      const body = await r.json();
      setResult(body.result ?? "ok");
      setSessionId(body.session_id ?? null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "unknown error");
    } finally {
      setLoading(false);
    }
  }

  const shortcuts = [
    { key: "Ctrl K / ⌘K", label: t("alfred_advanced_shortcut_palette") },
    { key: "Alt A",        label: t("alfred_advanced_shortcut_alfred") },
    { key: "Ctrl J",       label: t("alfred_advanced_shortcut_dark") },
    { key: "Ctrl B",       label: t("alfred_advanced_shortcut_sidebar") },
  ];

  return (
    <div style={{ maxWidth: 1024, margin: "0 auto" }}>
      {/* Top bar: App picker + switch-to-friendly link */}
      <div className="alfred-advanced-topbar">
        <div className="alfred-advanced-app-picker">
          <span className="text-sm font-medium" style={{ color: "var(--fg-2)" }}>
            {t("alfred_advanced_app_label")}
          </span>
          {/* App picker scopes subsequent slash commands (2.2). */}
          {visible.length > 0 ? (
            <select
              value={scopedAppId ?? APP_PICKER_SENTINEL}
              onChange={(e) =>
                setScopedAppId(e.target.value === APP_PICKER_SENTINEL ? null : e.target.value)
              }
              className="rounded border border-neutral-300 bg-transparent px-2 py-1 text-sm dark:border-neutral-700"
            >
              <option value={APP_PICKER_SENTINEL}>All apps</option>
              {visible.map((a) => (
                <option key={a.id} value={a.id}>
                  {a.name}
                </option>
              ))}
            </select>
          ) : (
            <ScopeSelect
              kind="app"
              name="app_id"
              className="rounded border border-neutral-300 bg-transparent px-2 py-1 text-sm dark:border-neutral-700"
            />
          )}
        </div>
        <button
          type="button"
          className="text-sm"
          onClick={onSwitchView}
          style={{ color: "var(--fg-3)" }}
        >
          ← {t("alfred_friendly_back")}
        </button>
      </div>

      <div className="page-head" style={{ marginBottom: 16 }}>
        <div className="h-eyebrow">Platform · Alfred</div>
        <h1 className="page-title">
          {t("alfred_advanced_title")} <em>{t("alfred_advanced_subtitle")}</em>
        </h1>
      </div>

      {result && (
        <div className="rounded border border-neutral-200 bg-white p-3 text-sm dark:border-neutral-800 dark:bg-neutral-900 mb-4">
          <span style={{ color: "var(--thread)" }}>{result}: </span>
          <code>{sessionId}</code>
        </div>
      )}
      {error && (
        <div className="rounded border border-red-300 bg-red-50 p-3 text-sm text-red-800 dark:border-red-800 dark:bg-red-950 dark:text-red-200 mb-4">
          {error}
        </div>
      )}

      <form
        onSubmit={submit}
        className="space-y-4 rounded border border-neutral-200 bg-white p-5 dark:border-neutral-800 dark:bg-neutral-900"
      >
        <label className="grid gap-1 text-sm">
          <span className="font-medium">Intent or slash command</span>
          <textarea
            name="message"
            value={text}
            onChange={(e) => setText(e.target.value)}
            required
            rows={8}
            placeholder={t("alfred_advanced_placeholder")}
            className="rounded border border-neutral-300 bg-transparent px-3 py-2 font-mono text-sm dark:border-neutral-700"
          />
        </label>
        <button
          type="submit"
          disabled={loading}
          className="rounded bg-neutral-900 px-4 py-2 text-sm font-medium text-white dark:bg-neutral-100 dark:text-neutral-900"
        >
          {loading ? "…" : "Run Alfred"}
        </button>
      </form>

      {/* Keyboard shortcuts cheat sheet (task 2.4) */}
      <div className="mt-4">
        <button
          type="button"
          className="text-sm"
          style={{ color: "var(--fg-3)" }}
          onClick={() => setShowShortcuts((s) => !s)}
        >
          {t("alfred_advanced_shortcuts_title")} {showShortcuts ? "▲" : "▼"}
        </button>
        {showShortcuts && (
          <div className="mt-2 grid gap-2 rounded border border-neutral-200 bg-white p-4 dark:border-neutral-800 dark:bg-neutral-900 md:grid-cols-2 text-sm">
            {shortcuts.map((s) => (
              <div key={s.key} className="flex items-center justify-between">
                <span style={{ color: "var(--fg-2)" }}>{s.label}</span>
                <kbd className="rounded bg-neutral-100 px-2 py-0.5 font-mono text-xs dark:bg-neutral-800">
                  {s.key}
                </kbd>
              </div>
            ))}
          </div>
        )}
      </div>

      {/* Command reference */}
      <div className="grid gap-3 rounded border border-dashed border-neutral-300 bg-white p-5 text-sm dark:border-neutral-800 dark:bg-neutral-900 md:grid-cols-2 mt-4">
        <div>
          <h3 className="font-medium">Create specification</h3>
          <code className="mt-2 block rounded bg-neutral-100 p-2 text-xs dark:bg-neutral-800">
            {`/forge create title="Payments" intent="Reduce failures" requirement="Retry failed payments" jira=PAY-123`}
          </code>
        </div>
        <div>
          <h3 className="font-medium">Edit specification</h3>
          <code className="mt-2 block rounded bg-neutral-100 p-2 text-xs dark:bg-neutral-800">
            {`/forge edit id=payments title="Payments v2" problem="Retries are inconsistent"`}
          </code>
        </div>
      </div>
    </div>
  );
}
