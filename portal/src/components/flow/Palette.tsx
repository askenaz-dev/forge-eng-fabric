"use client";

// Palette is the four-section sidebar (Triggers / AI / Actions / Logic +
// optional Custom). Items are draggable onto the React Flow canvas;
// keyboard users can Tab through them and press Enter to add the node.
//
// The legacy `event-trigger` step type is intentionally hidden from
// the palette — its place is now the Triggers section. Loaded legacy
// flows surface the `migratedFrom` banner on the in-canvas node.

import { useEffect, useMemo, useRef } from "react";
import {
  CANONICAL_STEP_TYPES,
  CANONICAL_TRIGGER_TYPES,
  type CanonicalStepType,
  type CanonicalTriggerType,
} from "@/lib/ast-canvas-adapter";
import {
  STEP_NODE_META,
  TRIGGER_NODE_META,
  familyOrder,
  familyTitle,
  type NodeFamily,
} from "./nodeMetadata";

export interface PaletteItem {
  kind: "step" | "trigger";
  type: CanonicalStepType | CanonicalTriggerType;
  label: string;
  family: NodeFamily;
}

export function buildPaletteItems(opts: { customNodesRegistered: boolean }): PaletteItem[] {
  const out: PaletteItem[] = [];
  for (const t of CANONICAL_TRIGGER_TYPES) {
    const meta = TRIGGER_NODE_META[t];
    out.push({ kind: "trigger", type: t, label: meta.label, family: "trigger" });
  }
  for (const s of CANONICAL_STEP_TYPES) {
    if (s === "custom" && !opts.customNodesRegistered) continue;
    const meta = STEP_NODE_META[s];
    out.push({ kind: "step", type: s, label: meta.label, family: meta.family });
  }
  return out;
}

export function Palette({
  customNodesRegistered,
  onAdd,
  catalogStatus,
}: {
  customNodesRegistered: boolean;
  onAdd: (item: PaletteItem) => void;
  /** Optional banner for catalog-load failures. */
  catalogStatus?: { error?: string; pinningActive?: boolean };
}) {
  const items = useMemo(() => buildPaletteItems({ customNodesRegistered }), [customNodesRegistered]);
  const liveRef = useRef<HTMLDivElement>(null);

  // Announce additions via ARIA live region for screen readers.
  useEffect(() => {
    if (!liveRef.current) return;
    liveRef.current.textContent = "";
  }, []);

  const grouped = useMemo(() => {
    const m: Record<NodeFamily, PaletteItem[]> = { trigger: [], ai: [], action: [], logic: [], custom: [] };
    for (const it of items) m[it.family].push(it);
    return m;
  }, [items]);

  return (
    <aside
      role="region"
      aria-label="AI Flow node palette"
      className="w-[260px] shrink-0 border-r border-neutral-200 dark:border-neutral-800 overflow-y-auto"
    >
      <h3 className="text-xs font-semibold uppercase tracking-wide px-3 py-2 opacity-60">Palette</h3>
      {catalogStatus?.error && (
        <p className="mx-3 mb-2 rounded border border-yellow-300 bg-yellow-50 p-2 text-xs text-yellow-800 dark:border-yellow-700 dark:bg-yellow-950 dark:text-yellow-200">
          Could not load gateway catalogs: {catalogStatus.error}. The editor still works — gateway endpoints will be resolved at save time.
        </p>
      )}
      {catalogStatus?.pinningActive && (
        <p className="mx-3 mb-2 rounded border border-indigo-300 bg-indigo-50 p-2 text-xs text-indigo-800 dark:border-indigo-700 dark:bg-indigo-950 dark:text-indigo-200">
          This flow has a pinned asset set. Outside-of-pin entries are marked and require confirmation to add.
        </p>
      )}
      {familyOrder().map((fam) => {
        const list = grouped[fam];
        if (list.length === 0) return null;
        return (
          <section key={fam} className="px-3 pb-3" aria-labelledby={`palette-${fam}-heading`}>
            <h4 id={`palette-${fam}-heading`} className="text-[10px] font-medium uppercase tracking-wider opacity-60 my-2">
              {familyTitle(fam)}
            </h4>
            <ul className="space-y-1">
              {list.map((item) => (
                <li key={`${item.kind}-${item.type}`}>
                  <button
                    type="button"
                    draggable
                    onDragStart={(e) => {
                      e.dataTransfer.setData("application/forge-flow-item", JSON.stringify(item));
                      e.dataTransfer.effectAllowed = "move";
                    }}
                    onClick={() => {
                      onAdd(item);
                      if (liveRef.current) {
                        liveRef.current.textContent = `Added ${item.label} to the canvas.`;
                      }
                    }}
                    onKeyDown={(e) => {
                      if (e.key === "Enter") {
                        onAdd(item);
                        if (liveRef.current) {
                          liveRef.current.textContent = `Added ${item.label} to the canvas.`;
                        }
                      }
                    }}
                    aria-label={`Add ${item.label} (${familyTitle(item.family)}) to the canvas`}
                    className="w-full text-left rounded border border-neutral-200 dark:border-neutral-800 px-2 py-1 text-xs hover:bg-neutral-50 dark:hover:bg-neutral-900 focus-visible:outline focus-visible:outline-2 focus-visible:outline-black"
                    data-testid={`palette-item-${item.kind}-${item.type}`}
                  >
                    {item.label}
                  </button>
                </li>
              ))}
            </ul>
          </section>
        );
      })}
      <div ref={liveRef} aria-live="polite" aria-atomic="true" className="sr-only" />
    </aside>
  );
}
