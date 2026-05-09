"use client";

// Client shell for the workflow editor. The Flowise embed is loaded
// dynamically once the npm dependency lands (see ADR-0001). Until then,
// this shell renders a simplified in-place editor that exercises the same
// adapter contract — good enough for round-trip testing and persistence
// validation.

import { useState } from "react";
import { astToFlowise, flowiseToAST, type CanonicalWorkflow } from "@/lib/flowise-adapter";

type Props = {
  readOnly: boolean;
  initialAst: CanonicalWorkflow | null;
  workspaceId: string;
  workflowId?: string;
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

export default function EditorClient({ readOnly, initialAst, workspaceId, workflowId }: Props) {
  const [ast, setAst] = useState<CanonicalWorkflow>(initialAst ?? SAMPLE_WORKFLOW);
  const [editing, setEditing] = useState<string>(JSON.stringify(initialAst ?? SAMPLE_WORKFLOW, null, 2));
  const [savedVersion, setSavedVersion] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

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
  );
}

// Minimal YAML serializer for the canonical AST. Full fidelity is delegated to
// the Go-side `pkg/workflow/dsl.Marshal` once the editor POSTs to the registry —
// this client-side stringify is just for previews and round-trip checks.
function stringifyAsYAML(ast: CanonicalWorkflow): string {
  return JSON.stringify(ast, null, 2);
}
