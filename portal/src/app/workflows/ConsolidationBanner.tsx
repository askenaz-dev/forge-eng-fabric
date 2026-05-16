"use client";

// One-time banner introduced by the ai-flow-authoring change. Tells
// existing /workflows users that authoring has moved to the visual
// canvas at /workflows/editor. Dismissed per-user via localStorage.

import { useEffect, useState } from "react";

const STORAGE_KEY = "forge.ai-flows.consolidation-banner.dismissed";

export function ConsolidationBanner() {
  const [dismissed, setDismissed] = useState(true);

  useEffect(() => {
    try {
      setDismissed(localStorage.getItem(STORAGE_KEY) === "1");
    } catch {
      setDismissed(false);
    }
  }, []);

  if (dismissed) return null;

  return (
    <div
      role="status"
      className="rounded border border-indigo-300 bg-indigo-50 p-3 mb-4 text-sm text-indigo-900 dark:border-indigo-700 dark:bg-indigo-950 dark:text-indigo-200 flex items-start gap-3"
    >
      <div className="flex-1">
        <p className="font-medium">Authoring moved to the visual canvas.</p>
        <p className="mt-1 text-xs opacity-90">
          This page is now the AI Flow library + version history. To create or edit a flow, click{" "}
          <strong>Open in canvas</strong> on the selected flow (or go to{" "}
          <a className="underline" href="/workflows/editor">
            /workflows/editor
          </a>
          ). The YAML view is a tab inside the canvas.
        </p>
      </div>
      <button
        type="button"
        onClick={() => {
          try {
            localStorage.setItem(STORAGE_KEY, "1");
          } catch {
            /* ignore */
          }
          setDismissed(true);
        }}
        className="text-xs underline opacity-80 hover:opacity-100"
        aria-label="Dismiss banner"
      >
        Got it
      </button>
    </div>
  );
}
