import { fetchOnboardingRequests, requirePortalIdentity } from "@/lib/onboarding";
import type { OnboardingRequest } from "@/lib/onboarding-types";
import { PageHead } from "@/components/page/PageHead";
import { Button, Card } from "@/components/primitives";
import Link from "next/link";

type SearchParams = { workspace_id?: string; status?: string };

export default async function OnboardingHistoryPage({ searchParams }: { searchParams: SearchParams }) {
  const identity = await requirePortalIdentity();
  let requests: OnboardingRequest[] = [];
  let error: string | null = null;
  try {
    requests = await fetchOnboardingRequests({ workspace_id: searchParams.workspace_id, status: searchParams.status }, identity.token);
  } catch (e) {
    error = e instanceof Error ? e.message : "failed to load onboarding history";
  }

  return (
    <>
      <PageHead
        eyebrow="Platform · Apps"
        title="Onboarding"
        titleEm="history"
        sub="Live onboarding state, stage timeline and emitted events per request."
        actions={
          <form method="get" style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
            <input
              name="workspace_id"
              defaultValue={searchParams.workspace_id ?? ""}
              placeholder="Workspace ID"
              className="top-search"
              style={{ height: 32, width: 200 }}
            />
            <select
              name="status"
              defaultValue={searchParams.status ?? ""}
              style={{
                height: 32,
                background: "var(--bg-card)",
                border: "1px solid var(--border)",
                borderRadius: "var(--r-2)",
                padding: "0 10px",
                color: "var(--fg)",
                fontFamily: "var(--f-sans)",
                fontSize: 13,
              }}
            >
              <option value="">any status</option>
              <option value="running">running</option>
              <option value="completed">completed</option>
              <option value="failed">failed</option>
              <option value="pending_approval">pending approval</option>
            </select>
            <Button variant="primary" type="submit">Load</Button>
            <Link href="/apps/new">
              <Button variant="secondary">New app</Button>
            </Link>
          </form>
        }
      />
      {error && (
        <Card style={{ marginBottom: 16 }}>
          <div style={{ padding: 14, color: "var(--rust)" }}>{error}</div>
        </Card>
      )}
      <div className="stack">
        {requests.map((request) => <RequestRow key={request.id} request={request} />)}
        {requests.length === 0 && !error && (
          <Card>
            <div className="note" style={{ padding: 24, textAlign: "center" }}>
              No onboarding requests match this filter.
            </div>
          </Card>
        )}
      </div>
    </>
  );
}

function RequestRow({ request }: { request: OnboardingRequest }) {
  return (
    <article className="rounded-3xl border border-neutral-200 bg-white p-5 dark:border-neutral-800 dark:bg-neutral-900">
      <div className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
        <div>
          <h3 className="text-lg font-semibold">{request.repo_org}/{request.repo_name}</h3>
          <p className="mt-1 text-sm opacity-70">{request.template_id}@{request.template_version} · Workspace <code>{request.workspace_id}</code></p>
          <p className="mt-2 text-xs opacity-60">correlation <code>{request.correlation_id}</code></p>
        </div>
        <div className="flex flex-wrap gap-2 text-xs font-semibold">
          <span className="rounded bg-neutral-100 px-2 py-1 dark:bg-neutral-800">{request.status}</span>
          <span className="rounded bg-neutral-100 px-2 py-1 dark:bg-neutral-800">{request.criticality}</span>
          {request.asset_id && <span className="rounded bg-green-100 px-2 py-1 text-green-800 dark:bg-green-950 dark:text-green-200">asset registered</span>}
        </div>
      </div>
      {request.status_reason && <p className="mt-3 rounded border border-yellow-300 bg-yellow-50 p-3 text-sm text-yellow-900 dark:border-yellow-800 dark:bg-yellow-950 dark:text-yellow-100">{request.status_reason}</p>}
      <div className="mt-4 flex items-center justify-between text-sm">
        <span className="opacity-60">Created {new Date(request.created_at).toLocaleString()}</span>
        <a className="font-medium underline" href={`/onboarding/${request.id}`}>View live events</a>
      </div>
    </article>
  );
}
