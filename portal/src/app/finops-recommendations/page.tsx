import { authOptions } from "@/auth";
import { getServerSession } from "next-auth";
import { redirect } from "next/navigation";
import { PageHead } from "@/components/page/PageHead";
import { Button } from "@/components/primitives";

type Recommendation = {
  id: string;
  tenant_id: string;
  asset_id?: string;
  kind: string;
  title: string;
  detail: string;
  expected_savings_usd_monthly: number;
  affected_resources: string[];
  pr_url?: string;
  pr_status: string;
  severity: string;
  synthetic: boolean;
  created_at: string;
};

type RecommendationsResponse = {
  recommendations: Recommendation[];
  total_savings_usd_monthly: number;
  by_kind: Record<string, number>;
};

const finopsUrl = () => process.env.FINOPS_ADVISOR_URL ?? "http://localhost:8107";

async function getToken() {
  const session = await getServerSession(authOptions);
  if (!session) redirect("/api/auth/signin");
  return (session as { accessToken?: string }).accessToken;
}

async function fetchRecommendations(tenantId: string, token?: string): Promise<RecommendationsResponse> {
  const params = tenantId ? `?tenant_id=${encodeURIComponent(tenantId)}` : "";
  const response = await fetch(`${finopsUrl()}/v1/finops/recommendations${params}`, {
    headers: token ? { authorization: `Bearer ${token}` } : {},
    cache: "no-store",
  });
  if (!response.ok) return { recommendations: [], total_savings_usd_monthly: 0, by_kind: {} };
  return (await response.json()) as RecommendationsResponse;
}

export default async function FinOpsRecommendationsPage({ searchParams }: { searchParams: { tenant_id?: string } }) {
  const token = await getToken();
  const tenantId = searchParams.tenant_id ?? "";
  const data = await fetchRecommendations(tenantId, token);

  return (
    <>
      <PageHead
        eyebrow="Observability · FinOps"
        title="FinOps"
        titleEm="recommendations"
        sub="Cost-reduction proposals from the autonomous FinOps advisor. Each comes with a draft PR."
        actions={
          <form method="get" style={{ display: "flex", gap: 8 }}>
            <input name="tenant_id" defaultValue={tenantId} placeholder="Tenant ID" className="top-search" style={{ height: 32, width: 200 }} />
            <Button variant="primary" type="submit">Filter</Button>
          </form>
        }
      />

      <div className="grid gap-3 sm:grid-cols-3" data-testid="finops-summary">
        <Stat label="Total recs" value={data.recommendations.length} />
        <Stat label="Expected savings ($/mo)" value={`$${data.total_savings_usd_monthly.toFixed(2)}`} />
        <Stat label="Patterns" value={Object.keys(data.by_kind).length} />
      </div>

      <ul className="space-y-3" data-testid="finops-list">
        {data.recommendations.map((rec) => (
          <li key={rec.id} className="rounded border border-neutral-200 bg-white p-4 dark:border-neutral-800 dark:bg-neutral-900" data-testid={`rec-${rec.id}`}>
            <div className="flex items-start justify-between gap-3">
              <div>
                <h3 className="font-medium">{rec.title}</h3>
                <p className="text-xs opacity-60">{rec.kind} · severity {rec.severity}</p>
              </div>
              <div className="text-right">
                <p className="text-lg font-semibold">${rec.expected_savings_usd_monthly.toFixed(2)}/mo</p>
                {rec.pr_url ? (
                  <a href={rec.pr_url} className="text-xs text-blue-600 underline" target="_blank" rel="noreferrer">PR ({rec.pr_status})</a>
                ) : (
                  <span className="text-xs opacity-60">no PR</span>
                )}
              </div>
            </div>
            <p className="mt-2 text-sm">{rec.detail}</p>
            <div className="mt-2 text-xs opacity-70">
              Affected: {rec.affected_resources.map((r) => <code key={r} className="mr-2 rounded bg-neutral-100 px-1 py-0.5 dark:bg-neutral-800">{r}</code>)}
            </div>
          </li>
        ))}
        {data.recommendations.length === 0 && <p className="rounded border border-dashed border-neutral-300 p-6 text-sm opacity-70 dark:border-neutral-800">No recommendations yet — the advisor runs daily.</p>}
      </ul>
    </>
  );
}

function Stat({ label, value }: { label: string; value: number | string }) {
  return (
    <div className="rounded border border-neutral-200 bg-white p-4 dark:border-neutral-800 dark:bg-neutral-900">
      <p className="text-xs uppercase opacity-60">{label}</p>
      <p className="text-2xl font-semibold">{value}</p>
    </div>
  );
}
