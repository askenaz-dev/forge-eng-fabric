// Visual workflow editor route — embeds the canonical node-catalog editor
// inside the Portal's auth/workspace context. Reject non-`workflow.author`
// users with a clear permission error (per workflow-visual-editor spec).
//
// Note: the live Flowise embed lands in a follow-up integration once the
// LGPL distribution mechanics are wired in CI. This page provides the
// permission-gated shell, the node catalog, and the version-aware persistence
// surface that the Flowise host will plug into.

import { getServerSession } from "next-auth";
import { redirect } from "next/navigation";
import { authOptions } from "@/auth";
import EditorClient from "./EditorClient";

type SearchParams = { workspace_id?: string; workflow_id?: string; version?: string };

const workflowRegistryUrl = () =>
  process.env.WORKFLOW_REGISTRY_URL ?? "http://localhost:8094";

const NODE_CATALOG = [
  { type: "llm", label: "LLM", color: "purple" },
  { type: "mcp", label: "MCP", color: "blue" },
  { type: "skill", label: "Skill", color: "emerald" },
  { type: "agent", label: "Agent", color: "amber" },
  { type: "prompt-template", label: "Prompt Template", color: "indigo" },
  { type: "human-in-the-loop", label: "HITL Gate", color: "rose" },
  { type: "branch", label: "Branch", color: "sky" },
  { type: "loop", label: "Loop", color: "sky" },
  { type: "retry", label: "Retry", color: "yellow" },
  { type: "eval", label: "Eval", color: "violet" },
  { type: "webhook", label: "Webhook", color: "teal" },
  { type: "github-action", label: "GitHub Action", color: "neutral" },
  { type: "deploy-action", label: "Deploy Action", color: "lime" },
  { type: "approval-action", label: "Approval Action", color: "rose" },
  { type: "notification-action", label: "Notification Action", color: "pink" },
] as const;

async function userHasWorkflowAuthor(token: string | undefined, workspaceId: string | undefined): Promise<boolean> {
  // Best-effort permission check — when the permissions service is unreachable
  // OR no workspace context is provided, the page renders a permission notice.
  if (!workspaceId || !token) return false;
  try {
    const r = await fetch(
      `${process.env.PERMISSIONS_URL ?? "http://localhost:8092"}/v1/check?relation=workflow.author&object=workspace:${workspaceId}`,
      { headers: { authorization: `Bearer ${token}` }, cache: "no-store" },
    );
    if (!r.ok) return false;
    const body = await r.json();
    return Boolean(body.allowed);
  } catch {
    return false;
  }
}

async function fetchWorkflowVersion(workflowId: string, version: string | undefined): Promise<any | null> {
  try {
    const url = version
      ? `${workflowRegistryUrl()}/v1/workflows/${workflowId}/versions/${version}`
      : `${workflowRegistryUrl()}/v1/workflows/${workflowId}/versions/latest`;
    const r = await fetch(url, { cache: "no-store" });
    if (!r.ok) return null;
    return r.json();
  } catch {
    return null;
  }
}

export default async function WorkflowEditorPage({ searchParams }: { searchParams: SearchParams }) {
  const session = await getServerSession(authOptions);
  if (!session) redirect("/api/auth/signin");
  const token = (session as { accessToken?: string }).accessToken;

  const allowed = await userHasWorkflowAuthor(token, searchParams.workspace_id);
  if (!allowed) {
    return (
      <section className="space-y-3">
        <h2 className="text-2xl font-semibold">Workflow Editor</h2>
        <div className="rounded border border-amber-300 bg-amber-50 p-4 text-sm text-amber-900 dark:border-amber-800 dark:bg-amber-950 dark:text-amber-200">
          <p className="font-medium">Permission required</p>
          <p className="mt-1">
            You need the <code className="rounded bg-amber-100 px-1 dark:bg-amber-900">workflow.author</code> permission on
            this Workspace to open the editor. Ask a Workspace admin to grant it via{" "}
            <a className="underline" href="/permissions">Admin & Governance</a>.
          </p>
        </div>
      </section>
    );
  }

  const workflowId = searchParams.workflow_id;
  const versionParam = searchParams.version;
  const versionRecord = workflowId ? await fetchWorkflowVersion(workflowId, versionParam) : null;
  const isLatest = !versionParam || (versionRecord && versionRecord.is_latest !== false);
  const readOnly = !isLatest;

  return (
    <section className="space-y-4">
      <header className="flex items-baseline justify-between">
        <div>
          <p className="text-sm font-medium uppercase tracking-wide text-neutral-500">Workflow Editor</p>
          <h2 className="text-2xl font-semibold">{versionRecord?.ast?.metadata?.name ?? "New workflow"}</h2>
          <p className="text-xs text-neutral-500">
            {workflowId ? `${workflowId} · v${versionRecord?.version ?? "draft"}` : "(unsaved draft)"}
            {readOnly && " · READ-ONLY (older version)"}
          </p>
        </div>
        {readOnly && (
          <a
            className="rounded border border-neutral-300 px-3 py-2 text-sm dark:border-neutral-700"
            href={`/workflows/editor?workspace_id=${searchParams.workspace_id}&workflow_id=${workflowId}&fork=1`}
          >
            Fork as new latest
          </a>
        )}
      </header>

      <div className="grid gap-4 lg:grid-cols-[260px_1fr]">
        <aside className="space-y-2">
          <p className="text-xs font-medium uppercase tracking-wide text-neutral-500">Node Catalog</p>
          <ul className="grid grid-cols-2 gap-2 text-xs">
            {NODE_CATALOG.map((node) => (
              <li
                key={node.type}
                className="cursor-grab rounded border border-neutral-200 bg-white p-2 dark:border-neutral-800 dark:bg-neutral-900"
                draggable
                data-node-type={node.type}
              >
                <span className={`mr-1 inline-block h-2 w-2 rounded-full bg-${node.color}-500`} />
                {node.label}
              </li>
            ))}
          </ul>
          <p className="pt-2 text-xs text-neutral-500">
            Nodes referencing assets in non-<code>approved</code> state are visually marked and rejected on save.
          </p>
        </aside>

        <EditorClient
          readOnly={readOnly}
          initialAst={versionRecord?.ast ?? null}
          workspaceId={searchParams.workspace_id ?? ""}
          workflowId={workflowId}
        />
      </div>
    </section>
  );
}
