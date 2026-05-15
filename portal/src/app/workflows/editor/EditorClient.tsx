"use client";

// Client shell for the workflow editor. The Flowise embed is loaded
// dynamically once the npm dependency lands (see ADR-0001). Until then,
// this shell renders a simplified in-place editor that exercises the same
// adapter contract — good enough for round-trip testing and persistence
// validation.

import { useEffect, useMemo, useState } from "react";
import { astToFlowise, flowiseToAST, type CanonicalStep, type CanonicalWorkflow } from "@/lib/flowise-adapter";

type CatalogEntry = {
  asset_id: string;
  provenance?: "internal" | "external";
  active_surface?: { family: "mcp" | "a2a" | "skill"; endpoint?: string; artifact_pointer?: string };
};

type PaletteCatalog = {
  mcp: CatalogEntry[];
  a2a: CatalogEntry[];
  skill: CatalogEntry[];
};

type Props = {
  readOnly: boolean;
  initialAst: CanonicalWorkflow | null;
  workspaceId: string;
  workflowId?: string;
  /**
   * Selected-assets pinned set saved by the wizard (active-registry-gateways
   * §6.3). When non-empty, the palette marks unpinned entries with a small
   * indicator and prompts on add.
   */
  selectedAssets?: { skills?: string[]; mcps?: string[]; agents?: string[] };
};

const WORKFLOW_REGISTRY_URL = "/api/workflows";

const SAMPLE_WORKFLOW: CanonicalWorkflow = {
  apiVersion: "forge.workflows/v1",
  kind: "Workflow",
  metadata: {
    id: "untitled-workflow",
    name: "Untitled workflow",
    version: "0.1.0",
    visibility: "workspace",
    criticality: "medium",
  },
  spec: {
    inputs: [],
    steps: [
      {
        id: "step-1",
        type: "skill",
        ref: "registry:skill/example/sample@1.0.0",
      },
    ],
  },
};

export default function EditorClient({ readOnly, initialAst, workspaceId, workflowId, selectedAssets }: Props) {
  const [ast, setAst] = useState<CanonicalWorkflow>(initialAst ?? SAMPLE_WORKFLOW);
  const [editing, setEditing] = useState<string>(JSON.stringify(initialAst ?? SAMPLE_WORKFLOW, null, 2));
  const [savedVersion, setSavedVersion] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [catalog, setCatalog] = useState<PaletteCatalog>({ mcp: [], a2a: [], skill: [] });
  const [paletteError, setPaletteError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    // Source palette from the gateway catalogs (§7.4). The portal API
    // routes proxy these to mcp-gateway and a2a-gateway respectively;
    // skills come from the registry list. Failures here are non-fatal
    // and surface as a small banner above the palette.
    (async () => {
      try {
        const [mcp, a2a, skills] = await Promise.all([
          fetch("/api/gateway/mcp/catalog", { cache: "no-store" }).then(safeJSON),
          fetch("/api/gateway/a2a/catalog", { cache: "no-store" }).then(safeJSON),
          fetch(`/api/assets?workspace_id=${encodeURIComponent(workspaceId)}&type=skill&lifecycle_state=approved`, { cache: "no-store" }).then(safeJSON),
        ]);
        if (cancelled) return;
        setCatalog({
          mcp: extractCatalog(mcp),
          a2a: extractCatalog(a2a),
          skill: extractCatalog(skills, "skill"),
        });
      } catch (e: any) {
        if (!cancelled) setPaletteError(e?.message ?? "palette load failed");
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [workspaceId]);

  const pinnedSets = useMemo(
    () => ({
      mcp: new Set(selectedAssets?.mcps ?? []),
      a2a: new Set(selectedAssets?.agents ?? []),
      skill: new Set(selectedAssets?.skills ?? []),
    }),
    [selectedAssets],
  );
  const pinningActive = pinnedSets.mcp.size + pinnedSets.a2a.size + pinnedSets.skill.size > 0;

  function addStep(entry: CatalogEntry, family: "mcp" | "a2a" | "skill") {
    const pinned = pinnedSets[family].has(entry.asset_id);
    if (pinningActive && !pinned) {
      const proceed = window.confirm(
        `${entry.asset_id} is outside the pinned set for this workflow. Add it anyway? The runtime will refuse the invocation at execution time unless the wizard is updated.`,
      );
      if (!proceed) return;
    }
    const stepType: CanonicalStep["type"] = family === "mcp" ? "mcp" : family === "a2a" ? "agent" : "skill";
    const id = `${family}-${(ast.spec.steps.length + 1).toString().padStart(2, "0")}`;
    const newStep: CanonicalStep = {
      id,
      type: stepType,
      ref: entry.asset_id,
      active_surface: entry.active_surface
        ? {
            family: entry.active_surface.family,
            endpoint: entry.active_surface.endpoint,
            artifact_pointer: entry.active_surface.artifact_pointer,
          }
        : undefined,
    };
    const next = { ...ast, spec: { ...ast.spec, steps: [...ast.spec.steps, newStep] } };
    setAst(next);
    setEditing(JSON.stringify(next, null, 2));
  }

  function handleEditorChange(value: string) {
    setEditing(value);
    try {
      const parsed = JSON.parse(value) as CanonicalWorkflow;
      setAst(parsed);
      setError(null);
    } catch (e: any) {
      setError(`Invalid JSON: ${e.message}`);
    }
  }

  async function save() {
    if (readOnly) return;
    setBusy(true);
    setError(null);
    try {
      // Round-trip through the adapter to confirm the JSON survives Flowise format.
      const back = flowiseToAST(astToFlowise(ast));
      const r = await fetch(WORKFLOW_REGISTRY_URL, {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({
          workspace_id: workspaceId,
          workflow_id: workflowId ?? back.metadata.id,
          workflow_yaml: stringifyAsYAML(back),
        }),
      });
      if (!r.ok) throw new Error(`workflow-registry returned ${r.status}: ${await r.text()}`);
      const body = await r.json();
      setSavedVersion(body.version ?? "saved");
    } catch (e: any) {
      setError(e.message);
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="grid gap-3 lg:grid-cols-[280px_1fr]">
      <aside className="space-y-4" data-testid="palette">
        <div>
          <h3 className="mb-2 text-sm font-semibold uppercase tracking-wide opacity-70">Palette · Gateway catalogs</h3>
          {paletteError && (
            <div data-testid="palette-error" className="rounded border border-yellow-300 bg-yellow-50 p-2 text-xs text-yellow-800 dark:border-yellow-800 dark:bg-yellow-950 dark:text-yellow-200">
              Could not load gateway catalogs: {paletteError}. The editor still works — gateway endpoints will be resolved at save time.
            </div>
          )}
          {pinningActive && (
            <p data-testid="palette-pinning-banner" className="rounded border border-indigo-300 bg-indigo-50 p-2 text-xs text-indigo-800 dark:border-indigo-800 dark:bg-indigo-950 dark:text-indigo-200">
              Workflow has a pinned set. Outside-of-pin entries are marked and require confirmation to add.
            </p>
          )}
        </div>
        <PaletteSection title="Skills" family="skill" entries={catalog.skill} pinned={pinnedSets.skill} pinningActive={pinningActive} onAdd={addStep} />
        <PaletteSection title="MCPs" family="mcp" entries={catalog.mcp} pinned={pinnedSets.mcp} pinningActive={pinningActive} onAdd={addStep} />
        <PaletteSection title="Agents" family="a2a" entries={catalog.a2a} pinned={pinnedSets.a2a} pinningActive={pinningActive} onAdd={addStep} />
      </aside>
      <div className="space-y-3">
        {error && (
          <div className="rounded border border-red-300 bg-red-50 p-2 text-sm text-red-800 dark:border-red-800 dark:bg-red-950 dark:text-red-200">
            {error}
          </div>
        )}
        {savedVersion && !error && (
          <div className="rounded border border-emerald-300 bg-emerald-50 p-2 text-sm text-emerald-800 dark:border-emerald-800 dark:bg-emerald-950 dark:text-emerald-200">
            Saved as version {savedVersion}.
          </div>
        )}
        <textarea
          readOnly={readOnly}
          rows={28}
          value={editing}
          onChange={(e) => handleEditorChange(e.target.value)}
          className="w-full rounded border border-neutral-200 bg-white p-3 font-mono text-xs dark:border-neutral-800 dark:bg-neutral-900"
        />
        <div className="flex items-center gap-2">
          <button
            disabled={readOnly || busy}
            onClick={save}
            className="rounded bg-neutral-900 px-4 py-2 text-sm font-medium text-white disabled:opacity-50 dark:bg-neutral-100 dark:text-neutral-900"
          >
            {busy ? "Saving…" : readOnly ? "Read-only" : "Save as new version"}
          </button>
          <p className="text-xs text-neutral-500">
            Each save persists a new immutable version to <code>workflow-registry</code>.
          </p>
        </div>
      </div>
    </div>
  );
}

function PaletteSection({
  title,
  family,
  entries,
  pinned,
  pinningActive,
  onAdd,
}: {
  title: string;
  family: "mcp" | "a2a" | "skill";
  entries: CatalogEntry[];
  pinned: Set<string>;
  pinningActive: boolean;
  onAdd: (entry: CatalogEntry, family: "mcp" | "a2a" | "skill") => void;
}) {
  // Pinned entries sort first when a pinned set is active.
  const sorted = [...entries].sort((a, b) => {
    if (!pinningActive) return a.asset_id.localeCompare(b.asset_id);
    const ap = pinned.has(a.asset_id) ? 0 : 1;
    const bp = pinned.has(b.asset_id) ? 0 : 1;
    if (ap !== bp) return ap - bp;
    return a.asset_id.localeCompare(b.asset_id);
  });
  return (
    <section data-testid={`palette-${family}`}>
      <h4 className="mb-1 text-xs font-medium uppercase tracking-wide opacity-60">{title}</h4>
      {sorted.length === 0 ? (
        <p className="text-xs opacity-60">(empty)</p>
      ) : (
        <ul className="space-y-1">
          {sorted.map((entry) => {
            const isPinned = pinned.has(entry.asset_id);
            const offPin = pinningActive && !isPinned;
            return (
              <li key={entry.asset_id}>
                <button
                  type="button"
                  onClick={() => onAdd(entry, family)}
                  data-testid={`palette-item-${entry.asset_id}`}
                  data-pinned={isPinned ? "true" : "false"}
                  data-off-pin={offPin ? "true" : "false"}
                  className={
                    "flex w-full items-start gap-2 rounded border px-2 py-1 text-left text-xs " +
                    (isPinned
                      ? "border-emerald-300 bg-emerald-50 dark:border-emerald-700 dark:bg-emerald-950"
                      : offPin
                        ? "border-orange-300 bg-orange-50 dark:border-orange-700 dark:bg-orange-950"
                        : "border-neutral-200 hover:bg-neutral-50 dark:border-neutral-800 dark:hover:bg-neutral-900")
                  }
                >
                  <span className="flex-1 font-mono">{entry.asset_id}</span>
                  {entry.provenance === "external" && <span className="rounded bg-indigo-100 px-1 text-[10px] text-indigo-800 dark:bg-indigo-900 dark:text-indigo-200">ext</span>}
                  {isPinned && <span className="rounded bg-emerald-600 px-1 text-[10px] text-white">pin</span>}
                  {offPin && <span className="rounded bg-orange-600 px-1 text-[10px] text-white">off-pin</span>}
                </button>
              </li>
            );
          })}
        </ul>
      )}
    </section>
  );
}

async function safeJSON(r: Response): Promise<any> {
  if (!r.ok) throw new Error(`status ${r.status}`);
  return r.json();
}

function extractCatalog(body: any, fallbackFamily: "mcp" | "a2a" | "skill" | null = null): CatalogEntry[] {
  // Gateway catalog format: { items: [{ asset_id, provenance, active_surface, how_to }] }
  // Registry list format: an array of Asset.
  if (Array.isArray(body)) {
    return body.map((a: any) => ({
      asset_id: a.id ?? a.asset_id,
      provenance: a.provenance,
      active_surface: a.active_surface ?? (fallbackFamily ? { family: fallbackFamily } : undefined),
    }));
  }
  if (body && Array.isArray(body.items)) {
    return body.items.map((it: any) => ({
      asset_id: it.asset_id ?? it.id,
      provenance: it.provenance,
      active_surface: it.active_surface,
    }));
  }
  return [];
}

// Minimal YAML serializer for the canonical AST. Full fidelity is delegated to
// the Go-side `pkg/workflow/dsl.Marshal` once the editor POSTs to the registry —
// this client-side stringify is just for previews and round-trip checks.
function stringifyAsYAML(ast: CanonicalWorkflow): string {
  return JSON.stringify(ast, null, 2);
}
