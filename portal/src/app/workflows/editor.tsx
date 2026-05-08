"use client";

import { useMemo, useState } from "react";

type Workflow = {
  id: string;
  name: string;
  visibility: string;
  latest_version?: string;
};

type Version = {
  workflow_id: string;
  version: string;
  ast: any;
  lifecycle_state: string;
  published_at?: string;
};

const SAMPLE_DSL = `apiVersion: forge.workflows/v1
kind: Workflow
metadata:
  id: my-workflow
  name: My Workflow
  version: 1.0.0
  visibility: workspace
  criticality: medium
spec:
  inputs:
    - name: story
      type: string
      required: true
  steps:
    - id: refine
      type: skill
      ref: registry:skill/sdlc-product/refine-user-story@1.2.0
      inputs:
        story: $inputs.story
      retries:
        max: 3
        backoff: exponential
      timeout: 60s
    - id: human-approval
      type: human-in-the-loop
      approver_role: product-owner
      depends_on: [refine]
      timeout: 24h
      on_timeout: escalate
      escalation_role: engineering-manager
    - id: open-pr
      type: mcp
      ref: registry:mcp/github@write
      tool: create_pr
      depends_on: [human-approval]
      inputs:
        title: $steps.refine.outputs.refined
`;

export function WorkflowEditor({
  tenantId,
  workspaceId,
  workflow,
  versions,
  publishAction,
  dryRunAction,
}: {
  tenantId: string;
  workspaceId: string;
  workflow: Workflow;
  versions: Version[];
  publishAction: (formData: FormData) => Promise<void>;
  dryRunAction: (formData: FormData) => Promise<void>;
}) {
  const latest = versions[0];
  const initial = latest ? toYAML(latest.ast) : SAMPLE_DSL.replace("my-workflow", workflow.id).replace("My Workflow", workflow.name);
  const [yaml, setYaml] = useState(initial);

  const parsed = useMemo(() => parseSimpleYaml(yaml), [yaml]);
  const lintFindings = useMemo(() => lintLocally(parsed), [parsed]);

  return (
    <div className="space-y-4 rounded border border-neutral-200 bg-white p-5 dark:border-neutral-800 dark:bg-neutral-900">
      <header className="flex items-start justify-between">
        <div>
          <h3 className="font-medium">{workflow.name}</h3>
          <p className="text-xs opacity-60">
            id <code>{workflow.id}</code> · visibility <strong>{workflow.visibility}</strong> · latest{" "}
            {workflow.latest_version ?? "—"}
          </p>
        </div>
        <span className="rounded-full bg-neutral-100 px-3 py-1 text-xs dark:bg-neutral-800">
          {versions.length} version{versions.length === 1 ? "" : "s"}
        </span>
      </header>

      <div className="grid gap-4 md:grid-cols-2">
        <form action={publishAction} className="grid gap-3">
          <input type="hidden" name="tenant_id" value={tenantId} />
          <input type="hidden" name="workspace_id" value={workspaceId} />
          <input type="hidden" name="workflow_id" value={workflow.id} />
          <label className="grid gap-1 text-sm">
            <span className="font-medium">DSL (YAML)</span>
            <textarea
              name="workflow_yaml"
              value={yaml}
              onChange={(event) => setYaml(event.target.value)}
              rows={28}
              spellCheck={false}
              className="rounded border border-neutral-300 bg-neutral-50 px-3 py-2 font-mono text-xs dark:border-neutral-700 dark:bg-neutral-950"
            />
          </label>
          <div className="flex flex-wrap gap-2 text-sm">
            <label className="inline-flex items-center gap-2">
              <input type="checkbox" name="auto_bump" value="1" /> Auto-bump version
            </label>
            <button className="rounded bg-neutral-900 px-3 py-1.5 text-sm text-white dark:bg-neutral-100 dark:text-neutral-900">
              Publish version
            </button>
            <ImportExport yaml={yaml} setYaml={setYaml} />
          </div>
        </form>
        <div className="space-y-3 text-sm">
          <GraphPreview steps={parsed.steps} />
          <LintPanel findings={lintFindings} />
          <form action={dryRunAction} className="space-y-2 rounded border border-neutral-200 p-3 dark:border-neutral-800">
            <p className="font-medium">Dry-run (mock I/O)</p>
            <input type="hidden" name="tenant_id" value={tenantId} />
            <input type="hidden" name="workspace_id" value={workspaceId} />
            <input type="hidden" name="workflow_yaml" value={yaml} />
            <textarea
              name="inputs_json"
              defaultValue={'{"story": "as a user I want..."}'}
              rows={3}
              className="w-full rounded border border-neutral-300 bg-transparent px-2 py-1 font-mono text-xs dark:border-neutral-700"
            />
            <button className="rounded border border-neutral-300 px-3 py-1.5 dark:border-neutral-700">Dry run</button>
          </form>
        </div>
      </div>
    </div>
  );
}

function ImportExport({ yaml, setYaml }: { yaml: string; setYaml: (v: string) => void }) {
  return (
    <span className="inline-flex gap-2">
      <button
        type="button"
        onClick={() => {
          const blob = new Blob([yaml], { type: "text/yaml" });
          const url = URL.createObjectURL(blob);
          const a = document.createElement("a");
          a.href = url;
          a.download = "workflow.yaml";
          a.click();
          URL.revokeObjectURL(url);
        }}
        className="rounded border border-neutral-300 px-3 py-1.5 dark:border-neutral-700"
      >
        Export YAML
      </button>
      <label className="cursor-pointer rounded border border-neutral-300 px-3 py-1.5 dark:border-neutral-700">
        Import YAML
        <input
          type="file"
          accept=".yaml,.yml,text/yaml"
          className="hidden"
          onChange={async (event) => {
            const file = event.target.files?.[0];
            if (!file) return;
            const text = await file.text();
            setYaml(text);
          }}
        />
      </label>
    </span>
  );
}

function GraphPreview({ steps }: { steps: { id: string; type: string; depends_on?: string[]; ref?: string }[] }) {
  if (!steps?.length) {
    return <div className="rounded border border-dashed border-neutral-300 p-4 text-xs opacity-60 dark:border-neutral-700">No steps yet.</div>;
  }
  return (
    <div className="rounded border border-neutral-200 p-3 dark:border-neutral-800">
      <p className="mb-2 font-medium">Graph preview</p>
      <ol className="space-y-1 text-xs">
        {steps.map((step) => (
          <li key={step.id} className="flex items-start gap-2">
            <span className="rounded bg-neutral-100 px-2 py-0.5 font-mono dark:bg-neutral-800">{step.type}</span>
            <span className="font-mono">{step.id}</span>
            {step.depends_on && step.depends_on.length > 0 && (
              <span className="opacity-60">← {step.depends_on.join(", ")}</span>
            )}
            {step.ref && <span className="opacity-60">{step.ref}</span>}
          </li>
        ))}
      </ol>
      <p className="mt-2 text-xs opacity-60">
        For the full visual editor, install <code>reactflow</code> in the portal package.
      </p>
    </div>
  );
}

type Finding = { code: string; severity: "error" | "warning"; step_id?: string; message: string };

function LintPanel({ findings }: { findings: Finding[] }) {
  if (findings.length === 0) {
    return <div className="rounded border border-green-300 p-3 text-xs text-green-800 dark:border-green-800 dark:text-green-200">Lint clean ✓</div>;
  }
  return (
    <div className="rounded border border-red-300 p-3 text-xs text-red-800 dark:border-red-800 dark:text-red-200">
      <p className="mb-2 font-medium">Lint findings</p>
      <ul className="space-y-1">
        {findings.map((f, i) => (
          <li key={i}>
            <code>{f.code}</code> {f.step_id ? `[${f.step_id}]` : ""} {f.message}
          </li>
        ))}
      </ul>
    </div>
  );
}

// Lightweight client-side YAML parsing — sufficient for graph preview and
// local lint feedback. Server-side validation is authoritative.
function parseSimpleYaml(yaml: string): { steps: { id: string; type: string; depends_on?: string[]; ref?: string }[] } {
  const steps: { id: string; type: string; depends_on?: string[]; ref?: string }[] = [];
  const lines = yaml.split(/\r?\n/);
  let inSteps = false;
  let current: any = null;
  for (const raw of lines) {
    const line = raw.replace(/\t/g, "  ");
    if (/^\s*steps:\s*$/.test(line)) {
      inSteps = true;
      continue;
    }
    if (!inSteps) continue;
    if (/^[A-Za-z]/.test(line)) {
      inSteps = false;
      if (current) steps.push(current);
      current = null;
      continue;
    }
    const startMatch = line.match(/^\s*-\s+id:\s*(.+)$/);
    if (startMatch) {
      if (current) steps.push(current);
      current = { id: startMatch[1].trim() };
      continue;
    }
    if (!current) continue;
    const kv = line.match(/^\s+(type|ref|tool|approver_role):\s*(.+)$/);
    if (kv) {
      current[kv[1]] = kv[2].trim();
    }
    const deps = line.match(/^\s+depends_on:\s*\[(.+)\]\s*$/);
    if (deps) {
      current.depends_on = deps[1].split(",").map((s) => s.trim().replace(/^"|"$/g, ""));
    }
  }
  if (current) steps.push(current);
  return { steps };
}

function lintLocally(parsed: { steps: { id: string; type?: string; depends_on?: string[]; ref?: string }[] }): Finding[] {
  const findings: Finding[] = [];
  const seen = new Map<string, number>();
  for (const s of parsed.steps) {
    seen.set(s.id, (seen.get(s.id) ?? 0) + 1);
    if (s.ref && /@(latest|main|master|stable|current)$/i.test(s.ref)) {
      findings.push({
        code: "floating_reference_not_allowed",
        severity: "error",
        step_id: s.id,
        message: `ref ${s.ref} is not pinned`,
      });
    }
  }
  for (const [id, count] of seen) {
    if (count > 1) {
      findings.push({ code: "duplicate_step_id", severity: "error", step_id: id, message: `${id} appears ${count} times` });
    }
  }
  const ids = new Set(parsed.steps.map((s) => s.id));
  for (const s of parsed.steps) {
    for (const dep of s.depends_on ?? []) {
      if (!ids.has(dep)) {
        findings.push({ code: "dangling_dep", severity: "error", step_id: s.id, message: `unknown step ${dep}` });
      }
    }
  }
  return findings;
}

function toYAML(astValue: any): string {
  if (!astValue || typeof astValue !== "object") return "";
  return JSON.stringify(astValue, null, 2);
}
