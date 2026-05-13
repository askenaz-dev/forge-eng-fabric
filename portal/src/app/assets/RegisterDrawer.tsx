"use client";

import { useEffect, useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import { Button, Sheet } from "@/components/primitives";
import { ScopeSelect } from "@/components/scope/ScopeSelect";
import { useToast } from "@/components/providers/ToastProvider";

export type AssetKind = "mcp" | "skill" | "agent" | "workflow" | "prompt_template";

const KIND_OPTIONS: { value: AssetKind; label: string }[] = [
  { value: "skill", label: "Skill" },
  { value: "agent", label: "Agent" },
  { value: "mcp", label: "MCP server" },
  { value: "workflow", label: "Workflow" },
  { value: "prompt_template", label: "Prompt template" },
];

const TRUST_LEVELS = ["T0", "T1", "T2", "T3", "T4", "T5"] as const;

const SEMVER_RE = /^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$/;

type Props = {
  open: boolean;
  onOpenChange: (next: boolean) => void;
  workspaceId: string;
  lockedKind?: AssetKind;
};

type FormState = {
  type: AssetKind;
  name: string;
  version: string;
  description: string;
  owner_team: string;
  visibility: "workspace" | "tenant";
  trust_level: (typeof TRUST_LEVELS)[number];
  owners: string;
  inputs_schema: string;
  outputs_schema: string;
  metadata: string;
  workspace_id: string;
};

const EMPTY: FormState = {
  type: "skill",
  name: "",
  version: "0.1.0",
  description: "",
  owner_team: "",
  visibility: "workspace",
  trust_level: "T0",
  owners: "",
  inputs_schema: '{ "type": "object" }',
  outputs_schema: '{ "type": "object" }',
  metadata: "",
  workspace_id: "",
};

export function RegisterDrawer({ open, onOpenChange, workspaceId, lockedKind }: Props) {
  const router = useRouter();
  const toast = useToast();
  const [form, setForm] = useState<FormState>(() => ({ ...EMPTY, type: lockedKind ?? EMPTY.type, workspace_id: workspaceId }));
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (open) {
      setForm({ ...EMPTY, type: lockedKind ?? EMPTY.type, workspace_id: workspaceId });
      setError(null);
    }
  }, [open, lockedKind, workspaceId]);

  const canSubmit = useMemo(() => {
    if (!form.workspace_id.trim()) return false;
    if (!form.name.trim()) return false;
    if (!SEMVER_RE.test(form.version.trim())) return false;
    if (!form.owner_team.trim()) return false;
    return true;
  }, [form]);

  function set<K extends keyof FormState>(key: K, value: FormState[K]) {
    setForm((current) => ({ ...current, [key]: value }));
  }

  function parseJsonField(label: string, raw: string): Record<string, unknown> {
    if (!raw.trim()) return {};
    try {
      const parsed = JSON.parse(raw);
      if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) {
        throw new Error(`${label} must be a JSON object`);
      }
      return parsed as Record<string, unknown>;
    } catch (e) {
      throw new Error(`${label}: ${e instanceof Error ? e.message : "invalid JSON"}`);
    }
  }

  async function submit() {
    setSubmitting(true);
    setError(null);
    try {
      const inputs_schema = parseJsonField("inputs_schema", form.inputs_schema);
      const outputs_schema = parseJsonField("outputs_schema", form.outputs_schema);
      const metadata = form.metadata.trim() ? parseJsonField("metadata", form.metadata) : undefined;
      const owners = form.owners
        .split(",")
        .map((s) => s.trim())
        .filter(Boolean);
      const response = await fetch("/api/assets", {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({
          workspace_id: form.workspace_id.trim(),
          type: form.type,
          name: form.name.trim(),
          version: form.version.trim(),
          description: form.description.trim() || undefined,
          owner_team: form.owner_team.trim(),
          visibility: form.visibility,
          trust_level: form.trust_level,
          owners,
          inputs_schema,
          outputs_schema,
          metadata,
        }),
      });
      const payload = (await response.json().catch(() => ({}))) as {
        id?: string;
        error?: string;
        message?: string;
        code?: string;
      };
      if (!response.ok) {
        throw new Error(payload.error || payload.message || `registry ${response.status}`);
      }
      toast.success(`Registered ${form.name} ${form.version}`);
      onOpenChange(false);
      router.refresh();
    } catch (e) {
      setError(e instanceof Error ? e.message : "failed to register asset");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <Sheet
      open={open}
      onOpenChange={onOpenChange}
      title={<>Register <em>asset</em></>}
      subtitle={`workspace ${form.workspace_id || "—"} · lifecycle_state=proposed`}
      footer={
        <div style={{ display: "flex", gap: 8, justifyContent: "flex-end" }}>
          <Button variant="ghost" onClick={() => onOpenChange(false)} disabled={submitting}>
            Cancel
          </Button>
          <Button variant="primary" onClick={submit} disabled={!canSubmit || submitting}>
            {submitting ? "Registering…" : "Register"}
          </Button>
        </div>
      }
    >
      {error && (
        <p className="mb-3 rounded border border-red-300 bg-red-50 p-3 text-sm text-red-800 dark:border-red-800 dark:bg-red-950 dark:text-red-200">
          {error}
        </p>
      )}
      <div className="grid gap-4">
        <Row label="Workspace" required hint="Where the asset will live">
          <ScopeSelect
            kind="workspace"
            name="workspace_id"
            value={form.workspace_id}
            onChange={(next) => set("workspace_id", next)}
            required
            className={selectCls}
          />
        </Row>
        <Row label="Type" hint={lockedKind ? "Locked by sidebar filter" : "Select what you are publishing"}>
          <select
            value={form.type}
            disabled={Boolean(lockedKind)}
            onChange={(e) => set("type", e.target.value as AssetKind)}
            className={selectCls}
          >
            {KIND_OPTIONS.map((opt) => (
              <option key={opt.value} value={opt.value}>{opt.label}</option>
            ))}
          </select>
        </Row>
        <Row label="Name" required>
          <input value={form.name} onChange={(e) => set("name", e.target.value)} placeholder="my-skill" className={inputCls} />
        </Row>
        <Row label="Version" required hint="SemVer MAJOR.MINOR.PATCH">
          <input value={form.version} onChange={(e) => set("version", e.target.value)} placeholder="0.1.0" className={inputCls} />
        </Row>
        <Row label="Description">
          <textarea value={form.description} onChange={(e) => set("description", e.target.value)} rows={2} className={inputCls} />
        </Row>
        <Row label="Owner team" required>
          <input value={form.owner_team} onChange={(e) => set("owner_team", e.target.value)} placeholder="engineering" className={inputCls} />
        </Row>
        <Row label="Owners" hint="Comma-separated emails (optional)">
          <input value={form.owners} onChange={(e) => set("owners", e.target.value)} placeholder="ana@acme.io, ben@acme.io" className={inputCls} />
        </Row>
        <div className="grid gap-4 md:grid-cols-2">
          <Row label="Visibility">
            <select value={form.visibility} onChange={(e) => set("visibility", e.target.value as FormState["visibility"])} className={selectCls}>
              <option value="workspace">workspace</option>
              <option value="tenant">tenant</option>
            </select>
          </Row>
          <Row label="Trust level" hint="T0 minimum; promotion sets the rest">
            <select value={form.trust_level} onChange={(e) => set("trust_level", e.target.value as FormState["trust_level"])} className={selectCls}>
              {TRUST_LEVELS.map((tl) => (
                <option key={tl} value={tl}>{tl}</option>
              ))}
            </select>
          </Row>
        </div>
        <Row label="Inputs schema (JSON)" required hint="JSON Schema describing the asset's inputs">
          <textarea value={form.inputs_schema} onChange={(e) => set("inputs_schema", e.target.value)} rows={4} className={monoCls} spellCheck={false} />
        </Row>
        <Row label="Outputs schema (JSON)" required hint="JSON Schema describing the asset's outputs">
          <textarea value={form.outputs_schema} onChange={(e) => set("outputs_schema", e.target.value)} rows={4} className={monoCls} spellCheck={false} />
        </Row>
        <Row label="Metadata (JSON)" hint="Optional free-form metadata object">
          <textarea value={form.metadata} onChange={(e) => set("metadata", e.target.value)} rows={3} className={monoCls} spellCheck={false} placeholder="{}" />
        </Row>
      </div>
    </Sheet>
  );
}

function Row({ label, hint, required, children }: { label: string; hint?: string; required?: boolean; children: React.ReactNode }) {
  return (
    <label className="block">
      <span className="mb-1 flex items-baseline justify-between gap-3 text-xs uppercase tracking-wide opacity-70">
        <span>{label}{required && <span className="ml-1 text-red-600 dark:text-red-400">*</span>}</span>
        {hint && <span className="text-[10px] normal-case opacity-60">{hint}</span>}
      </span>
      {children}
    </label>
  );
}

const inputCls =
  "w-full rounded border border-neutral-300 bg-transparent px-2 py-1.5 text-sm outline-none focus:border-neutral-500 dark:border-neutral-700 dark:focus:border-neutral-400";
const selectCls = inputCls;
const monoCls =
  "w-full rounded border border-neutral-300 bg-transparent px-2 py-1.5 font-mono text-xs outline-none focus:border-neutral-500 dark:border-neutral-700 dark:focus:border-neutral-400";
