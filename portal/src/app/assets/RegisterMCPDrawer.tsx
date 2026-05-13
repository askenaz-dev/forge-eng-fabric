"use client";

import { useEffect, useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import { Button, Sheet } from "@/components/primitives";
import { ScopeSelect } from "@/components/scope/ScopeSelect";
import { useToast } from "@/components/providers/ToastProvider";

const TRUST_LEVELS = ["T0", "T1", "T2", "T3", "T4", "T5"] as const;
const TRANSPORTS = [
  { value: "http", label: "Streamable HTTP" },
  { value: "sse", label: "Server-Sent Events" },
] as const;

const SEMVER_RE = /^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$/;

type Props = {
  open: boolean;
  onOpenChange: (next: boolean) => void;
  workspaceId: string;
};

type FormState = {
  workspace_id: string;
  name: string;
  version: string;
  description: string;
  owner_team: string;
  visibility: "workspace" | "tenant";
  trust_level: (typeof TRUST_LEVELS)[number];
  owners: string;
  upstream_url: string;
  transport: (typeof TRANSPORTS)[number]["value"];
};

const EMPTY: FormState = {
  workspace_id: "",
  name: "",
  version: "0.1.0",
  description: "",
  owner_team: "",
  visibility: "workspace",
  trust_level: "T0",
  owners: "",
  upstream_url: "",
  transport: "http",
};

function isValidHttpsUrl(raw: string): boolean {
  try {
    const u = new URL(raw);
    return u.protocol === "https:" || u.protocol === "http:";
  } catch {
    return false;
  }
}

export function RegisterMCPDrawer({ open, onOpenChange, workspaceId }: Props) {
  const router = useRouter();
  const toast = useToast();
  const [form, setForm] = useState<FormState>(() => ({ ...EMPTY, workspace_id: workspaceId }));
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (open) {
      setForm({ ...EMPTY, workspace_id: workspaceId });
      setError(null);
    }
  }, [open, workspaceId]);

  const canSubmit = useMemo(() => {
    if (!form.workspace_id.trim()) return false;
    if (!form.name.trim()) return false;
    if (!SEMVER_RE.test(form.version.trim())) return false;
    if (!form.owner_team.trim()) return false;
    if (!isValidHttpsUrl(form.upstream_url.trim())) return false;
    return true;
  }, [form]);

  function set<K extends keyof FormState>(key: K, value: FormState[K]) {
    setForm((current) => ({ ...current, [key]: value }));
  }

  async function submit() {
    setSubmitting(true);
    setError(null);
    try {
      const owners = form.owners
        .split(",")
        .map((s) => s.trim())
        .filter(Boolean);
      const response = await fetch("/api/assets", {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({
          workspace_id: form.workspace_id.trim(),
          type: "mcp",
          name: form.name.trim(),
          version: form.version.trim(),
          description: form.description.trim() || undefined,
          owner_team: form.owner_team.trim(),
          visibility: form.visibility,
          trust_level: form.trust_level,
          owners,
          inputs_schema: { type: "object" },
          outputs_schema: { type: "object" },
          metadata: {
            remote_transport: {
              [form.transport]: { upstream_url: form.upstream_url.trim() },
            },
          },
        }),
      });
      const payload = (await response.json().catch(() => ({}))) as {
        id?: string;
        error?: string;
        message?: string;
      };
      if (!response.ok) {
        throw new Error(payload.error || payload.message || `registry ${response.status}`);
      }
      toast.success(`Registered MCP ${form.name} ${form.version}`);
      onOpenChange(false);
      router.refresh();
    } catch (e) {
      setError(e instanceof Error ? e.message : "failed to register MCP server");
    } finally {
      setSubmitting(false);
    }
  }

  const urlInvalid = form.upstream_url.trim() !== "" && !isValidHttpsUrl(form.upstream_url.trim());

  return (
    <Sheet
      open={open}
      onOpenChange={onOpenChange}
      title={<>Register <em>MCP server</em></>}
      subtitle={`workspace ${form.workspace_id || "—"} · transport=${form.transport} · lifecycle_state=proposed`}
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
      <p className="mb-4 rounded border border-neutral-200 bg-neutral-50 p-3 text-xs leading-relaxed text-neutral-700 dark:border-neutral-800 dark:bg-neutral-900 dark:text-neutral-300">
        Forge will reverse-proxy this MCP server through the gateway at
        <code className="mx-1 rounded bg-neutral-200 px-1 dark:bg-neutral-800">/v1/gateway/mcp/&lt;asset-id&gt;</code>
        once the asset is approved (T1+) and <code>distribution.gateway_published</code> is set. Identity
        and audit are injected at the gateway — the upstream sees the developer principal, not the gateway.
      </p>
      <div className="grid gap-4">
        <Row label="Workspace" required hint="Where the MCP server lives">
          <ScopeSelect
            kind="workspace"
            name="workspace_id"
            value={form.workspace_id}
            onChange={(next) => set("workspace_id", next)}
            required
            className={inputCls}
          />
        </Row>
        <Row label="Server name" required hint="kebab-case identifier — e.g. github, jira, postgres-readonly">
          <input
            value={form.name}
            onChange={(e) => set("name", e.target.value)}
            placeholder="github"
            className={inputCls}
          />
        </Row>
        <div className="grid gap-4 md:grid-cols-2">
          <Row label="Version" required hint="SemVer MAJOR.MINOR.PATCH">
            <input value={form.version} onChange={(e) => set("version", e.target.value)} placeholder="0.1.0" className={inputCls} />
          </Row>
          <Row label="Transport" required hint="stdio servers cannot be proxied">
            <select
              value={form.transport}
              onChange={(e) => set("transport", e.target.value as FormState["transport"])}
              className={inputCls}
            >
              {TRANSPORTS.map((t) => (
                <option key={t.value} value={t.value}>{t.label}</option>
              ))}
            </select>
          </Row>
        </div>
        <Row
          label="Upstream URL"
          required
          hint={urlInvalid ? "Must be an http(s) URL" : "Where the MCP server runs — gateway proxies here"}
        >
          <input
            value={form.upstream_url}
            onChange={(e) => set("upstream_url", e.target.value)}
            placeholder="https://mcp.internal/github"
            className={monoCls}
            spellCheck={false}
            aria-invalid={urlInvalid}
          />
        </Row>
        <Row label="Description">
          <textarea
            value={form.description}
            onChange={(e) => set("description", e.target.value)}
            rows={2}
            className={inputCls}
            placeholder="What does this MCP server expose? (Resources, tools, prompts.)"
          />
        </Row>
        <Row label="Owner team" required>
          <input value={form.owner_team} onChange={(e) => set("owner_team", e.target.value)} placeholder="engineering" className={inputCls} />
        </Row>
        <Row label="Owners" hint="Comma-separated emails (optional)">
          <input value={form.owners} onChange={(e) => set("owners", e.target.value)} placeholder="ana@acme.io, ben@acme.io" className={inputCls} />
        </Row>
        <div className="grid gap-4 md:grid-cols-2">
          <Row label="Visibility">
            <select
              value={form.visibility}
              onChange={(e) => set("visibility", e.target.value as FormState["visibility"])}
              className={inputCls}
            >
              <option value="workspace">workspace</option>
              <option value="tenant">tenant</option>
            </select>
          </Row>
          <Row label="Trust level" hint="T0 minimum; T1+ unlocks gateway exposure">
            <select
              value={form.trust_level}
              onChange={(e) => set("trust_level", e.target.value as FormState["trust_level"])}
              className={inputCls}
            >
              {TRUST_LEVELS.map((tl) => (
                <option key={tl} value={tl}>{tl}</option>
              ))}
            </select>
          </Row>
        </div>
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
  "w-full rounded border border-neutral-300 bg-transparent px-2 py-1.5 text-sm outline-none focus:border-neutral-500 dark:border-neutral-700 dark:focus:border-neutral-400 aria-invalid:border-red-500";
const monoCls =
  "w-full rounded border border-neutral-300 bg-transparent px-2 py-1.5 font-mono text-xs outline-none focus:border-neutral-500 dark:border-neutral-700 dark:focus:border-neutral-400 aria-invalid:border-red-500";
