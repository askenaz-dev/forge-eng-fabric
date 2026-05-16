import { authOptions } from "@/auth";
import { getServerSession } from "next-auth";
import { redirect } from "next/navigation";
// WorkflowEditor inline rendering removed per ai-flow-authoring §9.3.
// Authoring lives at /workflows/editor (visual canvas) and the Code view
// tab inside it. /workflows is now the library + version history.
import { CreateWorkflowForm } from "./CreateWorkflowForm";
import { ConsolidationBanner } from "./ConsolidationBanner";
import { PageHead } from "@/components/page/PageHead";
import { Button, Card, CardHeader } from "@/components/primitives";

type Workflow = {
  id: string;
  tenant_id: string;
  workspace_id: string;
  name: string;
  visibility: string;
  latest_version?: string;
  description?: string;
  tags?: string[];
};

type Version = {
  workflow_id: string;
  version: string;
  ast: any;
  lifecycle_state: string;
  published_at?: string;
  diff_prev?: { bump: string; reasons: string[]; major: boolean; minor: boolean };
};

type SearchParams = {
  tenant_id?: string;
  workspace_id?: string;
  workflow_id?: string;
  base_version?: string;
  compare_version?: string;
  saved?: string;
  error?: string;
};

const registryUrl = () => process.env.WORKFLOW_REGISTRY_URL ?? "http://localhost:8094";

async function getToken() {
  const session = await getServerSession(authOptions);
  if (!session) redirect("/api/auth/signin");
  return (session as { accessToken?: string }).accessToken;
}

async function fetchWorkflows(tenantId: string, workspaceId: string, token?: string) {
  const url = `${registryUrl()}/v1/workflows?tenant_id=${encodeURIComponent(tenantId)}&workspace_id=${encodeURIComponent(workspaceId)}`;
  const response = await fetch(url, {
    headers: token ? { authorization: `Bearer ${token}` } : {},
    cache: "no-store",
  });
  if (!response.ok) throw new Error(`workflow-registry ${response.status}: ${await response.text()}`);
  return ((await response.json()) as { workflows: Workflow[] }).workflows;
}

async function fetchVersions(workflowId: string, token?: string) {
  const response = await fetch(
    `${registryUrl()}/v1/workflows/${encodeURIComponent(workflowId)}/versions`,
    { headers: token ? { authorization: `Bearer ${token}` } : {}, cache: "no-store" },
  );
  if (!response.ok) return [];
  return ((await response.json()) as { versions: Version[] }).versions;
}

async function createWorkflow(formData: FormData) {
  "use server";
  const token = await getToken();
  const payload = {
    id: required(formData, "id"),
    tenant_id: required(formData, "tenant_id"),
    workspace_id: required(formData, "workspace_id"),
    name: required(formData, "name"),
    description: optional(formData, "description"),
    visibility: optional(formData, "visibility") || "workspace",
  };
  const response = await fetch(`${registryUrl()}/v1/workflows`, {
    method: "POST",
    headers: { "content-type": "application/json", ...(token ? { authorization: `Bearer ${token}` } : {}) },
    body: JSON.stringify(payload),
  });
  if (!response.ok) {
    redirect(`/workflows?tenant_id=${payload.tenant_id}&workspace_id=${payload.workspace_id}&error=${encodeURIComponent(await response.text())}`);
  }
  redirect(`/workflows?tenant_id=${payload.tenant_id}&workspace_id=${payload.workspace_id}&workflow_id=${payload.id}&saved=1`);
}

// publishVersion / dryRun server actions removed per ai-flow-authoring §9.3.
// Publish lives on the canvas's Save button (POSTing the canonical AST via
// /api/workflows). Dry-run lives on the canvas's Dry-run drawer. Both call
// workflow-registry / workflow-runtime through the same paths formerly used
// by these actions.

export default async function WorkflowsPage({ searchParams }: { searchParams: SearchParams }) {
  const token = await getToken();
  // Tenant/workspace come from the URL when navigating from a workflow link;
  // otherwise the inline Create form picks them from a dropdown (sourced
  // from /api/me/tenants and /api/me/workspaces).
  const tenantId = searchParams.tenant_id?.trim() ?? "";
  const workspaceId = searchParams.workspace_id?.trim() ?? "";

  let workflows: Workflow[] = [];
  let versions: Version[] = [];
  let selected: Workflow | null = null;
  let error: string | null = searchParams.error ?? null;

  if (tenantId && workspaceId) {
    try {
      workflows = await fetchWorkflows(tenantId, workspaceId, token);
      const selectedId = searchParams.workflow_id ?? workflows[0]?.id;
      if (selectedId) {
        selected = workflows.find((w) => w.id === selectedId) ?? null;
        versions = await fetchVersions(selectedId, token);
      }
    } catch (e) {
      error = e instanceof Error ? e.message : "failed to load workflows";
    }
  }

  const baseVersion = searchParams.base_version
    ? versions.find((v) => v.version === searchParams.base_version)
    : versions[1];
  const compareVersion = searchParams.compare_version
    ? versions.find((v) => v.version === searchParams.compare_version)
    : versions[0];

  return (
    <>
      <PageHead
        eyebrow="Platform · AI Flows"
        title="AI Flows &"
        titleEm="versions"
        sub="Author visually. Triggered by webhooks, cron, email, event bus, or manual. LLM steps orchestrate skills + MCPs + agents with full audit. Versioned with SemVer + immutability; dry-run before publish."
        actions={
          <form method="get" style={{ display: "flex", gap: 8 }}>
            <input
              name="tenant_id"
              defaultValue={tenantId}
              placeholder="Tenant ID"
              className="top-search"
              style={{ height: 32, width: 180 }}
            />
            <input
              name="workspace_id"
              defaultValue={workspaceId}
              placeholder="Workspace ID"
              className="top-search"
              style={{ height: 32, width: 200 }}
            />
            <Button variant="primary" type="submit">Load</Button>
          </form>
        }
      />

      <ConsolidationBanner />
      {searchParams.saved && (
        <div className="notice notice--ok" style={{ marginBottom: 16 }} role="status">
          <span className="body">Saved.</span>
        </div>
      )}
      {error && (
        <div className="notice notice--err" style={{ marginBottom: 16 }} role="alert">
          <span className="body">{error}</span>
        </div>
      )}

      <div className="grid gap-5 xl:grid-cols-[320px_1fr]">
        <aside
          style={{
            background: "var(--bg-card)",
            border: "1px solid var(--border)",
            borderRadius: "var(--r-4)",
            padding: 16,
            display: "grid",
            gap: 14,
            alignContent: "start",
          }}
        >
          <div>
            <h3 style={{ margin: 0, fontSize: 13, fontWeight: 500 }}>Workspace workflows</h3>
            <p className="h-eyebrow" style={{ margin: "6px 0 0" }}>
              {tenantId && workspaceId ? `${tenantId} · ${workspaceId}` : "Set a tenant + workspace to load"}
            </p>
          </div>
          {tenantId && workspaceId && (
            <div className="grid gap-2 text-sm">
              {workflows.map((wf) => {
                const isActive = selected?.id === wf.id;
                return (
                  <a
                    key={wf.id}
                    href={`/workflows?tenant_id=${encodeURIComponent(tenantId)}&workspace_id=${encodeURIComponent(workspaceId)}&workflow_id=${encodeURIComponent(wf.id)}`}
                    style={{
                      display: "block",
                      padding: "8px 10px",
                      borderRadius: "var(--r-2)",
                      border: "1px solid",
                      borderColor: isActive ? "color-mix(in oklch, var(--primary), transparent 60%)" : "var(--border)",
                      background: isActive ? "color-mix(in oklch, var(--primary), transparent 90%)" : "transparent",
                      color: "var(--fg)",
                      transition: "background var(--t-fast), border-color var(--t-fast)",
                    }}
                  >
                    <span style={{ display: "block", fontWeight: 500, fontSize: 13 }}>{wf.name}</span>
                    <span style={{ fontFamily: "var(--f-mono)", fontSize: 11, color: "var(--fg-3)" }}>
                      {wf.id} · {wf.visibility} · v{wf.latest_version ?? "-"}
                    </span>
                  </a>
                );
              })}
              {workflows.length === 0 && !error && (
                <p style={{ color: "var(--fg-3)", fontSize: 12, margin: 0 }}>No workflows yet. Create one below.</p>
              )}
            </div>
          )}
          <CreateWorkflowForm tenantId={tenantId} workspaceId={workspaceId} action={createWorkflow} />
        </aside>

        <div style={{ display: "grid", gap: 20 }}>
          {selected ? (
            <>
              <FlowSummary workflow={selected} versions={versions} tenantId={tenantId} workspaceId={workspaceId} />
              <DiffViewer base={baseVersion} compare={compareVersion} versions={versions} workflow={selected} tenantId={tenantId} workspaceId={workspaceId} />
            </>
          ) : (
            <div className="empty-state">
              <span className="title">No AI Flow selected</span>
              <span className="sub">
                {tenantId && workspaceId
                  ? "Pick a flow from the list, or create a new one. Authoring happens in the visual canvas at /workflows/editor."
                  : "Set a Tenant + Workspace using the Load form above. You can also type IDs directly in the New flow form to create one without loading first."}
              </span>
            </div>
          )}
        </div>
      </div>
    </>
  );
}

function FlowSummary({
  workflow,
  versions,
  tenantId,
  workspaceId,
}: {
  workflow: Workflow;
  versions: Version[];
  tenantId: string;
  workspaceId: string;
}) {
  const latest = versions[0];
  const editorHref =
    `/workflows/editor?` +
    new URLSearchParams({
      workspace_id: workspaceId,
      workflow_id: workflow.id,
    }).toString();
  const historyHref =
    `/workflows/${encodeURIComponent(workflow.id)}/history?` +
    new URLSearchParams({ tenant_id: tenantId, workspace_id: workspaceId }).toString();
  return (
    <Card>
      <CardHeader
        title={workflow.name}
        sub={`${workflow.id} · ${workflow.visibility}`}
      />
      <div style={{ padding: 16, display: "grid", gap: 12 }}>
        <p className="body" style={{ margin: 0 }}>
          Latest version: <strong>{latest?.version ?? "—"}</strong>
          {latest?.published_at ? ` · published ${new Date(latest.published_at).toLocaleString()}` : ""}
        </p>
        <p className="body" style={{ margin: 0, color: "var(--fg-3)" }}>
          {versions.length} version{versions.length === 1 ? "" : "s"} on file.
        </p>
        <div style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
          <a
            className="btn btn-primary"
            href={editorHref}
            style={{
              display: "inline-flex",
              alignItems: "center",
              gap: 6,
              padding: "8px 14px",
              borderRadius: "var(--r-2)",
              background: "var(--primary)",
              color: "var(--bg)",
              textDecoration: "none",
              fontSize: 13,
              fontWeight: 500,
            }}
          >
            Open in canvas →
          </a>
          <a
            href={historyHref}
            style={{
              display: "inline-flex",
              alignItems: "center",
              gap: 6,
              padding: "8px 14px",
              borderRadius: "var(--r-2)",
              border: "1px solid var(--border)",
              color: "var(--fg)",
              textDecoration: "none",
              fontSize: 13,
            }}
          >
            Version history
          </a>
        </div>
        <p className="body" style={{ margin: 0, fontSize: 11, color: "var(--fg-3)" }}>
          Authoring moved to <code>/workflows/editor</code>. Use the canvas tab for visual editing or the Code view tab for YAML.
        </p>
      </div>
    </Card>
  );
}

function DiffViewer({
  base,
  compare,
  versions,
  workflow,
  tenantId,
  workspaceId,
}: {
  base: Version | undefined;
  compare: Version | undefined;
  versions: Version[];
  workflow: Workflow;
  tenantId: string;
  workspaceId: string;
}) {
  const baseSteps = base?.ast?.spec?.steps ?? [];
  const compareSteps = compare?.ast?.spec?.steps ?? [];
  const baseIds = new Set(baseSteps.map((s: any) => s.id));
  const compareIds = new Set(compareSteps.map((s: any) => s.id));
  const added = compareSteps.filter((s: any) => !baseIds.has(s.id));
  const removed = baseSteps.filter((s: any) => !compareIds.has(s.id));
  return (
    <Card>
      <CardHeader title="Version diff" sub={versions.length > 0 ? `${versions.length} versions` : "no versions yet"} />
      <div style={{ padding: 16, display: "grid", gap: 14 }}>
        <form method="get" className="fld-row">
          <input type="hidden" name="tenant_id" value={tenantId} />
          <input type="hidden" name="workspace_id" value={workspaceId} />
          <input type="hidden" name="workflow_id" value={workflow.id} />
          <label className="fld" style={{ minWidth: 160 }}>
            <span className="fld-label">Base</span>
            <select name="base_version" defaultValue={base?.version} className="fld-select">
              {versions.map((v) => (
                <option key={`b-${v.version}`} value={v.version}>
                  v{v.version}
                </option>
              ))}
            </select>
          </label>
          <label className="fld" style={{ minWidth: 160 }}>
            <span className="fld-label">Compare</span>
            <select name="compare_version" defaultValue={compare?.version} className="fld-select">
              {versions.map((v) => (
                <option key={`c-${v.version}`} value={v.version}>
                  v{v.version}
                </option>
              ))}
            </select>
          </label>
          <Button variant="secondary" type="submit">Diff</Button>
        </form>

        {compare?.diff_prev && (
          <div
            style={{
              background: "var(--bg-sunk)",
              border: "1px solid var(--border)",
              borderRadius: "var(--r-3)",
              padding: 12,
              fontSize: 13,
            }}
          >
            <p style={{ margin: 0 }}>
              Bump: <strong>{compare.diff_prev.bump}</strong>
            </p>
            {compare.diff_prev.reasons.length > 0 && (
              <ul style={{ margin: "6px 0 0", paddingLeft: 18, fontSize: 12, color: "var(--fg-2)" }}>
                {compare.diff_prev.reasons.map((r, i) => (
                  <li key={i}>{r}</li>
                ))}
              </ul>
            )}
          </div>
        )}

        <div style={{ display: "grid", gap: 12, gridTemplateColumns: "repeat(auto-fit, minmax(220px, 1fr))" }}>
          <DiffPanel tone="rem" label="Removed steps" items={removed} />
          <DiffPanel tone="add" label="Added steps" items={added} />
        </div>
      </div>
    </Card>
  );
}

function DiffPanel({ tone, label, items }: { tone: "add" | "rem"; label: string; items: { id: string; type: string }[] }) {
  const color = tone === "add" ? "var(--thread)" : "var(--rust)";
  return (
    <div
      style={{
        border: `1px solid color-mix(in oklch, ${color}, transparent 60%)`,
        background: `color-mix(in oklch, ${color}, transparent 92%)`,
        borderRadius: "var(--r-3)",
        padding: 12,
        fontFamily: "var(--f-mono)",
        fontSize: 12,
      }}
    >
      <p style={{ margin: 0, color, fontWeight: 500, fontSize: 11.5, letterSpacing: ".06em", textTransform: "uppercase" }}>
        {label}
      </p>
      {items.length === 0 ? (
        <p style={{ margin: "8px 0 0", color: "var(--fg-3)", fontFamily: "var(--f-sans)", fontSize: 12 }}>none</p>
      ) : (
        <ul style={{ margin: "8px 0 0", padding: 0, listStyle: "none", display: "grid", gap: 4 }}>
          {items.map((s) => (
            <li key={s.id} style={{ color: "var(--fg)" }}>
              {s.id} <span style={{ color: "var(--fg-3)" }}>({s.type})</span>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}

function required(formData: FormData, key: string) {
  const value = optional(formData, key);
  if (!value) throw new Error(`${key} is required`);
  return value;
}

function optional(formData: FormData, key: string) {
  return String(formData.get(key) ?? "").trim();
}
