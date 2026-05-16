"use client";

// Client shell for the AI-Flow editor.
//
// Built atop the React Flow canvas (`@xyflow/react`) per ADR-0002. The
// canvas is rendered behind the AI_FLOWS_CANVAS_ENABLED feature flag;
// when the flag is OFF the page falls back to a thin status panel so
// rollout can flip cleanly.
//
// Catalogs (MCPs, A2A agents, skills, prompt templates, models) load
// from the gateway/registry endpoints and are passed down to the
// CanvasShell as `catalogs`.

import { useEffect, useMemo, useState } from "react";
import {
  astToCanvas as _astToCanvasUnused,
  type CanonicalWorkflow,
} from "@/lib/ast-canvas-adapter";
import CanvasShell from "@/components/flow/CanvasShell";
import { isCanvasEnabledFromEnv } from "@/components/flow/featureFlag";
import type { PropertyPanelCatalogs } from "@/components/flow/PropertyPanel";
import type { DryRunStepTrace } from "@/components/flow/types";

type CatalogEntry = {
  asset_id: string;
  provenance?: "internal" | "external";
  active_surface?: { family: "mcp" | "a2a" | "skill"; endpoint?: string; artifact_pointer?: string };
};

type Props = {
  readOnly: boolean;
  initialAst: CanonicalWorkflow | null;
  workspaceId: string;
  workflowId?: string;
  selectedAssets?: { skills?: string[]; mcps?: string[]; agents?: string[] };
};

const WORKFLOW_REGISTRY_URL = "/api/workflows";

const SAMPLE_WORKFLOW: CanonicalWorkflow = {
  apiVersion: "forge.workflows/v1",
  kind: "Workflow",
  metadata: {
    id: "untitled-workflow",
    name: "Untitled AI Flow",
    version: "0.1.0",
    visibility: "workspace",
    criticality: "medium",
  },
  spec: {
    inputs: [],
    steps: [],
  },
};

export default function EditorClient({ readOnly, initialAst, workspaceId, workflowId }: Props) {
  const ast = initialAst ?? SAMPLE_WORKFLOW;
  const canvasEnabled = isCanvasEnabledFromEnv();

  const [catalogs, setCatalogs] = useState<PropertyPanelCatalogs>({
    promptTemplates: [],
    models: [],
    mcps: [],
  });
  const [catalogError, setCatalogError] = useState<string | null>(null);
  const [savedVersion, setSavedVersion] = useState<string | null>(null);
  const [saveError, setSaveError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const [mcp, prompts, models] = await Promise.all([
          fetch("/api/gateway/mcp/catalog", { cache: "no-store" }).then(safeJSON).catch(() => null),
          fetch(`/api/prompt-templates?workspace_id=${encodeURIComponent(workspaceId)}`, {
            cache: "no-store",
          }).then(safeJSON).catch(() => null),
          fetch(`/api/models?workspace_id=${encodeURIComponent(workspaceId)}`, {
            cache: "no-store",
          }).then(safeJSON).catch(() => null),
        ]);
        if (cancelled) return;
        setCatalogs({
          mcps: extractMCPs(mcp),
          promptTemplates: extractPromptTemplates(prompts),
          models: extractModels(models),
        });
      } catch (e: any) {
        if (!cancelled) setCatalogError(e?.message ?? "catalog load failed");
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [workspaceId]);

  const persist = useMemo(
    () =>
      async (next: CanonicalWorkflow): Promise<void> => {
        const r = await fetch(WORKFLOW_REGISTRY_URL, {
          method: "POST",
          headers: { "content-type": "application/json" },
          body: JSON.stringify({
            workspace_id: workspaceId,
            workflow_id: workflowId ?? next.metadata.id,
            workflow_yaml: JSON.stringify(next, null, 2),
          }),
        });
        if (!r.ok) {
          const text = await r.text();
          throw new Error(`workflow-registry returned ${r.status}: ${text}`);
        }
        const body = await r.json();
        setSavedVersion(body.version ?? "saved");
        setSaveError(null);
      },
    [workspaceId, workflowId],
  );

  const dryRun = useMemo(
    () =>
      async (next: CanonicalWorkflow): Promise<DryRunStepTrace[]> => {
        const r = await fetch(`/api/workflows/${encodeURIComponent(next.metadata.id)}/dry-run`, {
          method: "POST",
          headers: { "content-type": "application/json" },
          body: JSON.stringify({
            workspace_id: workspaceId,
            workflow_yaml: JSON.stringify(next, null, 2),
            inputs: {},
          }),
        });
        if (!r.ok) {
          throw new Error(`dry-run failed: ${await r.text()}`);
        }
        const body = await r.json();
        return (body.steps as DryRunStepTrace[]) ?? [];
      },
    [workspaceId],
  );

  if (!canvasEnabled) {
    return (
      <div className="rounded border border-amber-300 bg-amber-50 p-4 text-sm text-amber-900 dark:border-amber-700 dark:bg-amber-950 dark:text-amber-200">
        <p className="font-medium">AI Flows canvas is not enabled in this environment.</p>
        <p className="mt-1">
          Set <code>AI_FLOWS_CANVAS_ENABLED=true</code> in the portal env to render the visual editor. The fallback YAML
          view at <code>/workflows</code> remains usable for editing.
        </p>
      </div>
    );
  }

  return (
    <div className="space-y-3">
      {catalogError && (
        <p className="rounded border border-yellow-300 bg-yellow-50 p-2 text-xs text-yellow-800 dark:border-yellow-700 dark:bg-yellow-950 dark:text-yellow-200">
          Some catalogs could not be loaded ({catalogError}). The canvas still works — save will resolve missing endpoints
          via the registry.
        </p>
      )}
      {savedVersion && !saveError && (
        <p className="rounded border border-emerald-300 bg-emerald-50 p-2 text-xs text-emerald-800 dark:border-emerald-700 dark:bg-emerald-950 dark:text-emerald-200">
          Saved as version {savedVersion}.
        </p>
      )}
      <CanvasShell
        workspaceId={workspaceId}
        workflowId={workflowId}
        initialAst={ast}
        readOnly={readOnly}
        catalogs={catalogs}
        onSave={persist}
        onDryRun={dryRun}
      />
    </div>
  );
}

async function safeJSON(r: Response): Promise<any> {
  if (!r.ok) throw new Error(`status ${r.status}`);
  return r.json();
}

function extractMCPs(body: any): { ref: string; label: string }[] {
  if (!body) return [];
  const items = Array.isArray(body) ? body : body.items;
  if (!Array.isArray(items)) return [];
  return items.map((it: any) => ({
    ref: it.asset_id ?? it.id ?? "",
    label: it.asset_id ?? it.id ?? "",
  }));
}

function extractPromptTemplates(body: any): { ref: string; label: string }[] {
  if (!body) return [];
  const items = Array.isArray(body) ? body : body.items;
  if (!Array.isArray(items)) return [];
  return items.map((it: any) => ({
    ref: it.ref ?? it.asset_id ?? "",
    label: it.label ?? it.ref ?? it.asset_id ?? "",
  }));
}

function extractModels(body: any): { ref: string; label: string; provider?: string; pricingPerToken?: number }[] {
  if (!body) return [];
  const items = Array.isArray(body) ? body : body.items;
  if (!Array.isArray(items)) return [];
  return items.map((it: any) => ({
    ref: it.ref ?? `gateway:model/${it.model_id}@latest-stable`,
    label: it.label ?? it.model_id ?? "",
    provider: it.provider,
    pricingPerToken: it.pricing_per_token,
  }));
}
