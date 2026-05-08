import { fetchOnboardingRequest, fetchOnboardingTimeline, requirePortalIdentity } from "@/lib/onboarding";
import { notFound } from "next/navigation";
import { LiveEvents } from "./events-client";

export default async function OnboardingDetailPage({ params }: { params: { id: string } }) {
  const identity = await requirePortalIdentity();
  const request = await fetchOnboardingRequest(params.id, identity.token);
  if (!request) notFound();
  const events = await fetchOnboardingTimeline(params.id, identity.token);

  return (
    <section className="space-y-5">
      <div className="flex flex-col gap-3 rounded-3xl border border-neutral-200 bg-white p-5 dark:border-neutral-800 dark:bg-neutral-900 md:flex-row md:items-start md:justify-between">
        <div>
          <a href="/onboarding" className="text-sm underline opacity-70">Back to history</a>
          <h2 className="mt-2 text-2xl font-semibold">{request.repo_org}/{request.repo_name}</h2>
          <p className="mt-1 text-sm opacity-70">{request.template_id}@{request.template_version} · requested by {request.requested_by}</p>
        </div>
        <div className="flex flex-wrap gap-2 text-xs font-semibold">
          <span className="rounded bg-neutral-100 px-2 py-1 dark:bg-neutral-800">{request.status}</span>
          <span className="rounded bg-neutral-100 px-2 py-1 dark:bg-neutral-800">{request.criticality}</span>
          <span className="rounded bg-neutral-100 px-2 py-1 dark:bg-neutral-800">{request.data_classification}</span>
        </div>
      </div>
      <div className="grid gap-4 md:grid-cols-4">
        <Fact label="Workspace" value={request.workspace_id} />
        <Fact label="Asset" value={request.asset_id ?? "pending"} />
        <Fact label="Correlation" value={request.correlation_id} />
        <Fact label="Owners" value={request.owners.join(", ")} />
      </div>
      <LiveEvents requestId={request.id} initialEvents={events} initialStatus={request.status} />
    </section>
  );
}

function Fact({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-2xl border border-neutral-200 bg-white p-4 dark:border-neutral-800 dark:bg-neutral-900">
      <p className="text-xs uppercase tracking-wide opacity-60">{label}</p>
      <p className="mt-2 break-all text-sm font-medium">{value}</p>
    </div>
  );
}
