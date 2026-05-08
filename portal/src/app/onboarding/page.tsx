import { fetchOnboardingRequests, requirePortalIdentity } from "@/lib/onboarding";
import type { OnboardingRequest } from "@/lib/onboarding-types";

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
    <section className="space-y-5">
      <div className="flex flex-col gap-3 md:flex-row md:items-end md:justify-between">
        <div>
          <h2 className="text-2xl font-semibold">Onboarding history</h2>
          <p className="mt-1 text-sm opacity-70">Live onboarding state, stage timeline and emitted events per request.</p>
        </div>
        <form className="flex flex-wrap gap-2 text-sm" method="get">
          <input name="workspace_id" defaultValue={searchParams.workspace_id ?? ""} placeholder="Workspace ID" className="rounded border border-neutral-300 bg-transparent px-3 py-2 dark:border-neutral-700" />
          <select name="status" defaultValue={searchParams.status ?? ""} className="rounded border border-neutral-300 bg-transparent px-3 py-2 dark:border-neutral-700">
            <option value="">any status</option>
            <option value="running">running</option>
            <option value="completed">completed</option>
            <option value="failed">failed</option>
            <option value="pending_approval">pending approval</option>
          </select>
          <button className="rounded bg-neutral-900 px-4 py-2 text-white dark:bg-neutral-100 dark:text-neutral-900">Load</button>
          <a href="/apps/new" className="rounded border border-neutral-300 px-4 py-2 dark:border-neutral-700">New app</a>
        </form>
      </div>
      {error && <p className="rounded border border-red-300 bg-red-50 p-3 text-sm text-red-800 dark:border-red-800 dark:bg-red-950 dark:text-red-200">{error}</p>}
      <div className="grid gap-4">
        {requests.map((request) => <RequestRow key={request.id} request={request} />)}
        {requests.length === 0 && !error && <p className="rounded border border-dashed border-neutral-300 p-6 text-sm opacity-70 dark:border-neutral-800">No onboarding requests match this filter.</p>}
      </div>
    </section>
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
