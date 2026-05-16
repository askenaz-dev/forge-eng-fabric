"use client";

// DryRunDrawer is the right-side panel that pops up after Dry-run is
// triggered. It shows the workflow-runtime execution trace (mock I/O)
// per step. The actual API call lives in the editor shell; this
// component is the presentational surface.

import type { DryRunStepTrace } from "./types";

export function DryRunDrawer({
  open,
  onClose,
  steps,
  estimatedCostUSD,
  startedAt,
  error,
}: {
  open: boolean;
  onClose: () => void;
  steps: DryRunStepTrace[];
  estimatedCostUSD?: number;
  startedAt?: string;
  error?: string;
}) {
  if (!open) return null;
  return (
    <div
      role="dialog"
      aria-label="Dry-run trace"
      className="absolute right-0 top-0 h-full w-[420px] bg-white dark:bg-neutral-950 border-l border-neutral-200 dark:border-neutral-800 shadow-xl z-20 overflow-y-auto"
    >
      <header className="flex items-center justify-between p-3 border-b border-neutral-200 dark:border-neutral-800">
        <h3 className="font-medium">Dry run</h3>
        <button
          type="button"
          onClick={onClose}
          aria-label="Close dry-run drawer"
          className="text-sm opacity-60 hover:opacity-100"
        >
          ✕
        </button>
      </header>
      <div className="p-3 space-y-3 text-xs">
        {startedAt && <p className="opacity-60">Started {startedAt}</p>}
        {typeof estimatedCostUSD === "number" && (
          <p>
            Estimated total cost: <strong>${estimatedCostUSD.toFixed(6)}</strong>
          </p>
        )}
        {error && (
          <p className="rounded border border-rose-300 bg-rose-50 p-2 text-rose-800 dark:border-rose-700 dark:bg-rose-950 dark:text-rose-200">
            {error}
          </p>
        )}
        {steps.length === 0 && !error && <p className="opacity-60">No steps executed yet.</p>}
        <ol className="space-y-2">
          {steps.map((s, i) => (
            <li
              key={i}
              className={`rounded border p-2 ${
                s.status === "failed"
                  ? "border-rose-300 bg-rose-50 dark:border-rose-700 dark:bg-rose-950"
                  : "border-neutral-200 dark:border-neutral-800"
              }`}
            >
              <p className="font-medium">
                {s.stepId} <span className="opacity-60">({s.type})</span>
              </p>
              <p className="opacity-60">{s.status}{s.durationMs ? ` · ${s.durationMs}ms` : ""}</p>
              {s.inputs && (
                <details className="mt-1">
                  <summary className="cursor-pointer">inputs</summary>
                  <pre className="mt-1 overflow-x-auto">{JSON.stringify(s.inputs, null, 2)}</pre>
                </details>
              )}
              {s.outputs && (
                <details className="mt-1">
                  <summary className="cursor-pointer">outputs</summary>
                  <pre className="mt-1 overflow-x-auto">{JSON.stringify(s.outputs, null, 2)}</pre>
                </details>
              )}
              {s.failureReason && (
                <p className="mt-1 text-rose-800 dark:text-rose-200">{s.failureReason}</p>
              )}
            </li>
          ))}
        </ol>
      </div>
    </div>
  );
}
