import { authOptions } from "@/auth";
import { getServerSession } from "next-auth";
import { redirect } from "next/navigation";
import { WorkflowEditor } from "./editor";

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
const runtimeUrl = () => process.env.WORKFLOW_RUNTIME_URL ?? "http://localhost:8093";

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

async function publishVersion(formData: FormData) {
  "use server";
  const token = await getToken();
  const tenantId = required(formData, "tenant_id");
  const workspaceId = required(formData, "workspace_id");
  const workflowId = required(formData, "workflow_id");
  const yaml = required(formData, "workflow_yaml");
  const autoBump = optional(formData, "auto_bump") === "1";
  const response = await fetch(`${registryUrl()}/v1/workflows/${encodeURIComponent(workflowId)}/versions`, {
    method: "POST",
    headers: { "content-type": "application/json", ...(token ? { authorization: `Bearer ${token}` } : {}) },
    body: JSON.stringify({ workflow_id: workflowId, workflow_yaml: yaml, auto_bump: autoBump, actor: "portal" }),
  });
  if (!response.ok) {
    const text = await response.text();
    redirect(`/workflows?tenant_id=${tenantId}&workspace_id=${workspaceId}&workflow_id=${workflowId}&error=${encodeURIComponent(text)}`);
  }
  redirect(`/workflows?tenant_id=${tenantId}&workspace_id=${workspaceId}&workflow_id=${workflowId}&saved=1`);
}

async function dryRun(formData: FormData) {
  "use server";
  const token = await getToken();
  const tenantId = required(formData, "tenant_id");
  const workspaceId = required(formData, "workspace_id");
  const yaml = required(formData, "workflow_yaml");
  const inputsRaw = optional(formData, "inputs_json") || "{}";
  let inputs: Record<string, unknown> = {};
  try {
    inputs = JSON.parse(inputsRaw);
  } catch {
    redirect(`/workflows?tenant_id=${tenantId}&workspace_id=${workspaceId}&error=${encodeURIComponent("invalid_inputs_json")}`);
  }
  const response = await fetch(`${runtimeUrl()}/v1/executions`, {
    method: "POST",
    headers: { "content-type": "application/json", ...(token ? { authorization: `Bearer ${token}` } : {}) },
    body: JSON.stringify({
      tenant_id: tenantId,
      workspace_id: workspaceId,
      workflow_yaml: yaml,
      inputs,
      dry_run: true,
      correlation_id: `portal-dryrun-${Date.now()}`,
    }),
  });
  if (!response.ok) {
    const text = await response.text();
    redirect(`/workflows?tenant_id=${tenantId}&workspace_id=${workspaceId}&error=${encodeURIComponent(text)}`);
  }
  redirect(`/workflows?tenant_id=${tenantId}&workspace_id=${workspaceId}&saved=1`);
}

export default async function WorkflowsPage({ searchParams }: { searchParams: SearchParams }) {
  const token = await getToken();
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
    <section className="space-y-6">
      <div className="flex flex-col gap-3 md:flex-row md:items-end md:justify-between">
        <div>
          <h2 className="text-2xl font-semibold">Workflows</h2>
          <p className="mt-1 text-sm opacity-70">
            Compose Skills, MCPs, Prompts, gates, branches and human-in-the-loop steps. Versioned with
            SemVer + immutability; dry-run before publish.
          </p>
        </div>
        <form className="flex gap-2" method="get">
          <input
            name="tenant_id"
            defaultValue={tenantId}
            placeholder="Tenant ID"
            className="min-w-0 rounded border border-neutral-300 bg-transparent px-3 py-2 text-sm dark:border-neutral-700"
          />
          <input
            name="workspace_id"
            defaultValue={workspaceId}
            placeholder="Workspace ID"
            className="min-w-0 rounded border border-neutral-300 bg-transparent px-3 py-2 text-sm dark:border-neutral-700"
          />
          <button className="rounded bg-neutral-900 px-4 py-2 text-sm text-white dark:bg-neutral-100 dark:text-neutral-900">Load</button>
        </form>
      </div>

      {searchParams.saved && (
        <p className="rounded border border-green-300 bg-green-50 p-3 text-sm text-green-800 dark:border-green-800 dark:bg-green-950 dark:text-green-200">
          Saved.
        </p>
      )}
      {error && (
        <p className="rounded border border-red-300 bg-red-50 p-3 text-sm text-red-800 dark:border-red-800 dark:bg-red-950 dark:text-red-200">
          {error}
        </p>
      )}

      <div className="grid gap-5 xl:grid-cols-[300px_1fr]">
        <aside className="space-y-4 rounded border border-neutral-200 bg-white p-4 dark:border-neutral-800 dark:bg-neutral-900">
          <h3 className="font-medium">Workspace workflows</h3>
          <div className="grid gap-2 text-sm">
            {workflows.map((wf) => (
              <a
                key={wf.id}
                href={`/workflows?tenant_id=${tenantId}&workspace_id=${workspaceId}&workflow_id=${wf.id}`}
                className={`rounded border px-3 py-2 ${
                  selected?.id === wf.id ? "border-blue-400 bg-blue-50 dark:bg-blue-950" : "border-neutral-200 hover:bg-neutral-50 dark:border-neutral-800 dark:hover:bg-neutral-800"
                }`}
              >
                <span className="block font-medium">{wf.name}</span>
                <span className="text-xs opacity-60">
                  {wf.id} · {wf.visibility} · v{wf.latest_version ?? "-"}
                </span>
              </a>
            ))}
            {tenantId && workspaceId && workflows.length === 0 && !error && (
              <p className="opacity-70">No workflows yet. Create one →</p>
            )}
          </div>
          <CreateWorkflowForm tenantId={tenantId} workspaceId={workspaceId} />
        </aside>

        <div className="space-y-5">
          {selected ? (
            <>
              <WorkflowEditor
                tenantId={tenantId}
                workspaceId={workspaceId}
                workflow={selected}
                versions={versions}
                publishAction={publishVersion}
                dryRunAction={dryRun}
              />
              <DiffViewer base={baseVersion} compare={compareVersion} versions={versions} workflow={selected} tenantId={tenantId} workspaceId={workspaceId} />
            </>
          ) : (
            <div className="rounded border border-dashed border-neutral-300 p-6 text-sm opacity-70 dark:border-neutral-800">
              Set a Tenant + Workspace and select a workflow.
            </div>
          )}
        </div>
      </div>
    </section>
  );
}

function CreateWorkflowForm({ tenantId, workspaceId }: { tenantId: string; workspaceId: string }) {
  return (
    <form action={createWorkflow} className="space-y-2 rounded border border-neutral-200 p-3 text-sm dark:border-neutral-800">
      <p className="font-medium">New workflow</p>
      <input type="hidden" name="tenant_id" value={tenantId} />
      <input type="hidden" name="workspace_id" value={workspaceId} />
      <input name="id" required placeholder="id (kebab-case)" className="w-full rounded border border-neutral-300 bg-transparent px-2 py-1 dark:border-neutral-700" />
      <input name="name" required placeholder="display name" className="w-full rounded border border-neutral-300 bg-transparent px-2 py-1 dark:border-neutral-700" />
      <select name="visibility" defaultValue="workspace" className="w-full rounded border border-neutral-300 bg-transparent px-2 py-1 dark:border-neutral-700">
        <option value="private">private</option>
        <option value="workspace">workspace</option>
        <option value="tenant">tenant</option>
      </select>
      <textarea name="description" rows={2} placeholder="description" className="w-full rounded border border-neutral-300 bg-transparent px-2 py-1 dark:border-neutral-700" />
      <button disabled={!tenantId || !workspaceId} className="w-full rounded bg-neutral-900 px-3 py-1.5 text-white disabled:opacity-40 dark:bg-neutral-100 dark:text-neutral-900">
        Create
      </button>
    </form>
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
    <div className="rounded border border-neutral-200 bg-white p-5 dark:border-neutral-800 dark:bg-neutral-900">
      <h3 className="font-medium">Version diff</h3>
      <form className="mt-3 flex flex-wrap gap-2 text-sm" method="get">
        <input type="hidden" name="tenant_id" value={tenantId} />
        <input type="hidden" name="workspace_id" value={workspaceId} />
        <input type="hidden" name="workflow_id" value={workflow.id} />
        <select name="base_version" defaultValue={base?.version} className="rounded border border-neutral-300 bg-transparent px-2 py-1 dark:border-neutral-700">
          {versions.map((v) => (
            <option key={`b-${v.version}`} value={v.version}>
              base v{v.version}
            </option>
          ))}
        </select>
        <select name="compare_version" defaultValue={compare?.version} className="rounded border border-neutral-300 bg-transparent px-2 py-1 dark:border-neutral-700">
          {versions.map((v) => (
            <option key={`c-${v.version}`} value={v.version}>
              compare v{v.version}
            </option>
          ))}
        </select>
        <button className="rounded border border-neutral-300 px-3 py-1 dark:border-neutral-700">Diff</button>
      </form>
      {compare?.diff_prev && (
        <div className="mt-3 rounded bg-neutral-100 p-3 text-sm dark:bg-neutral-800">
          <p>
            Bump: <strong>{compare.diff_prev.bump}</strong>
          </p>
          {compare.diff_prev.reasons.length > 0 && (
            <ul className="mt-1 list-inside list-disc text-xs opacity-80">
              {compare.diff_prev.reasons.map((r, i) => (
                <li key={i}>{r}</li>
              ))}
            </ul>
          )}
        </div>
      )}
      <div className="mt-3 grid gap-3 md:grid-cols-2">
        <div className="rounded border border-red-300 p-3 text-sm dark:border-red-800">
          <p className="font-medium text-red-700 dark:text-red-300">Removed steps</p>
          {removed.length === 0 ? (
            <p className="opacity-60">none</p>
          ) : (
            removed.map((s: any) => (
              <code key={s.id} className="block">
                {s.id} ({s.type})
              </code>
            ))
          )}
        </div>
        <div className="rounded border border-green-300 p-3 text-sm dark:border-green-800">
          <p className="font-medium text-green-700 dark:text-green-300">Added steps</p>
          {added.length === 0 ? (
            <p className="opacity-60">none</p>
          ) : (
            added.map((s: any) => (
              <code key={s.id} className="block">
                {s.id} ({s.type})
              </code>
            ))
          )}
        </div>
      </div>
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
