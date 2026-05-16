// Version-history surface for an AI Flow.
//
// Hosts the diff viewer that previously lived on /workflows. The full
// migration of the existing diff component lands when the
// `WorkflowEditor` inline editing on /workflows is removed (cutover
// §13.3); for the rollout window this page links back to /workflows
// with the legacy params so the existing diff UX keeps working.

import Link from "next/link";

type Props = {
  params: { id: string };
  searchParams: { tenant_id?: string; workspace_id?: string };
};

export default function WorkflowHistoryPage({ params, searchParams }: Props) {
  const { id } = params;
  const { tenant_id: tenantID = "", workspace_id: workspaceID = "" } = searchParams;
  const legacyHref =
    `/workflows?` +
    new URLSearchParams({
      tenant_id: tenantID,
      workspace_id: workspaceID,
      workflow_id: id,
    }).toString();
  return (
    <section className="space-y-4">
      <header>
        <p className="text-sm uppercase tracking-wide opacity-60">AI Flow · Version history</p>
        <h2 className="text-2xl font-semibold">{id}</h2>
      </header>
      <p className="text-sm">
        Version diff and bump classification live in the consolidated library at{" "}
        <Link className="underline" href={legacyHref}>
          /workflows
        </Link>{" "}
        during the rollout window. The standalone history surface (with no inline editor) lands at cutover.
      </p>
      <p className="text-sm">
        To author or edit visually, open the flow in the canvas:{" "}
        <Link
          className="underline"
          href={`/workflows/editor?workspace_id=${encodeURIComponent(workspaceID)}&workflow_id=${encodeURIComponent(id)}`}
        >
          /workflows/editor
        </Link>
        .
      </p>
    </section>
  );
}
