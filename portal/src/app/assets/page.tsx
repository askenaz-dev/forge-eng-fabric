import { authOptions } from "@/auth";
import { getServerSession } from "next-auth";
import { redirect } from "next/navigation";

type Asset = {
  id: string;
  version: string;
  type: string;
  name: string;
  description?: string;
  owner_team: string;
  workspace_id: string;
  lifecycle_state: string;
  trust_level: string;
  eval_scores?: Record<string, number>;
  metadata?: Record<string, unknown>;
};

type SearchParams = { workspace_id?: string; asset_id?: string; range?: string };

type AssetMetrics = {
  asset_id: string;
  range: string;
  granularity: string;
  drift_alert?: boolean;
  top_failing_steps?: string[];
  totals: {
    invocations: number;
    successes: number;
    failures: number;
    success_rate: number;
    latency_p50_ms: number;
    latency_p95_ms: number;
    latency_p99_ms: number;
    cost_total_usd: number;
    cost_llm_usd: number;
    cost_compute_usd: number;
    eval_score_avg?: number | null;
    business_metric_avg?: number | null;
    business_metric_key?: string;
  };
};

const registryUrl = () => process.env.REGISTRY_URL ?? "http://localhost:8082";
const observabilityUrl = () => process.env.ASSET_OBSERVABILITY_URL ?? "http://localhost:8096";

async function getToken() {
  const session = await getServerSession(authOptions);
  if (!session) redirect("/api/auth/signin");
  return (session as { accessToken?: string }).accessToken;
}

async function fetchAssets(workspaceId: string, token?: string) {
  const response = await fetch(`${registryUrl()}/v1/workspaces/${workspaceId}/assets`, {
    headers: token ? { authorization: `Bearer ${token}` } : {},
    cache: "no-store",
  });
  if (!response.ok) throw new Error(`registry ${response.status}: ${await response.text()}`);
  return (await response.json()) as Asset[];
}

async function fetchAssetMetrics(assetId: string, range: string, token?: string): Promise<AssetMetrics | null> {
  try {
    const response = await fetch(
      `${observabilityUrl()}/v1/assets/${encodeURIComponent(assetId)}/metrics?range=${encodeURIComponent(range)}&granularity=hour`,
      { headers: token ? { authorization: `Bearer ${token}` } : {}, cache: "no-store" },
    );
    if (!response.ok) return null;
    return (await response.json()) as AssetMetrics;
  } catch {
    return null;
  }
}

export default async function AssetsPage({ searchParams }: { searchParams: SearchParams }) {
  const token = await getToken();
  const workspaceId = searchParams.workspace_id?.trim() ?? "";
  const range = searchParams.range ?? "24h";
  let assets: Asset[] = [];
  let error: string | null = null;
  if (workspaceId) {
    try {
      assets = await fetchAssets(workspaceId, token);
    } catch (e) {
      error = e instanceof Error ? e.message : "failed to load assets";
    }
  }
  const selected = assets.find((asset) => asset.id === searchParams.asset_id) ?? assets[0] ?? null;
  const metrics = selected ? await fetchAssetMetrics(selected.id, range, token) : null;

  return (
    <section className="space-y-5">
      <div className="flex flex-col gap-3 md:flex-row md:items-end md:justify-between">
        <div>
          <h2 className="text-2xl font-semibold">Asset Registry</h2>
          <p className="mt-1 text-sm opacity-70">Lifecycle, trust level, eval trend and production invocation readiness.</p>
        </div>
        <form className="flex gap-2" method="get">
          <input name="workspace_id" defaultValue={workspaceId} placeholder="Workspace ID" className="rounded border border-neutral-300 bg-transparent px-3 py-2 text-sm dark:border-neutral-700" />
          <button className="rounded bg-neutral-900 px-4 py-2 text-sm text-white dark:bg-neutral-100 dark:text-neutral-900">Load</button>
        </form>
      </div>
      {error && <p className="rounded border border-red-300 bg-red-50 p-3 text-sm text-red-800 dark:border-red-800 dark:bg-red-950 dark:text-red-200">{error}</p>}
      <div className="grid gap-5 lg:grid-cols-[340px_1fr]">
        <aside className="space-y-2 rounded border border-neutral-200 bg-white p-4 dark:border-neutral-800 dark:bg-neutral-900">
          {assets.map((asset) => (
            <a key={`${asset.id}@${asset.version}`} href={`/assets?workspace_id=${workspaceId}&asset_id=${encodeURIComponent(asset.id)}`} className="block rounded border border-neutral-200 px-3 py-2 text-sm hover:bg-neutral-50 dark:border-neutral-800 dark:hover:bg-neutral-800">
              <span className="block font-medium">{asset.name}</span>
              <span className="text-xs opacity-60">{asset.type} · {asset.version} · {asset.lifecycle_state}</span>
            </a>
          ))}
          {assets.length === 0 && <p className="text-sm opacity-70">No assets loaded.</p>}
        </aside>
        {selected ? (
          <div className="space-y-5">
            <AssetDetail asset={selected} />
            <ObservabilityTab asset={selected} metrics={metrics} workspaceId={workspaceId} range={range} />
          </div>
        ) : (
          <div className="rounded border border-dashed border-neutral-300 p-6 text-sm opacity-70 dark:border-neutral-800">Select an asset to inspect lifecycle and evals.</div>
        )}
      </div>
    </section>
  );
}

function ObservabilityTab({
  asset,
  metrics,
  workspaceId,
  range,
}: {
  asset: Asset;
  metrics: AssetMetrics | null;
  workspaceId: string;
  range: string;
}) {
  const totals = metrics?.totals;
  return (
    <article className="rounded border border-neutral-200 bg-white p-5 dark:border-neutral-800 dark:bg-neutral-900">
      <header className="flex flex-wrap items-center justify-between gap-2">
        <h3 className="font-medium">Observability</h3>
        <form className="flex gap-2 text-sm" method="get">
          <input type="hidden" name="workspace_id" value={workspaceId} />
          <input type="hidden" name="asset_id" value={asset.id} />
          <select name="range" defaultValue={range} className="rounded border border-neutral-300 bg-transparent px-2 py-1 dark:border-neutral-700">
            {["1h", "24h", "7d", "30d"].map((r) => (
              <option key={r} value={r}>
                {r}
              </option>
            ))}
          </select>
          <button className="rounded border border-neutral-300 px-3 py-1 dark:border-neutral-700">Apply</button>
        </form>
      </header>
      {!metrics ? (
        <p className="mt-3 text-sm opacity-60">No metrics yet — invocations will appear here once the runtime emits them.</p>
      ) : (
        <div className="mt-3 space-y-3">
          {metrics.drift_alert && (
            <p className="rounded border border-orange-300 bg-orange-50 p-3 text-sm text-orange-800 dark:border-orange-800 dark:bg-orange-950 dark:text-orange-200">
              Eval drift detected — recent runs trending below baseline.
            </p>
          )}
          <div className="grid gap-3 md:grid-cols-4">
            <Metric label="Invocations" value={String(totals?.invocations ?? 0)} />
            <Metric label="Success rate" value={`${Math.round((totals?.success_rate ?? 0) * 100)}%`} />
            <Metric label="p95 latency" value={`${Math.round(totals?.latency_p95_ms ?? 0)}ms`} />
            <Metric label="Cost / run" value={`$${(totals && totals.invocations > 0 ? totals.cost_total_usd / totals.invocations : 0).toFixed(4)}`} />
          </div>
          {metrics.top_failing_steps && metrics.top_failing_steps.length > 0 && (
            <div className="rounded border border-red-300 p-3 text-sm dark:border-red-800">
              <p className="mb-1 font-medium">Top failing steps</p>
              <ul className="list-inside list-disc">
                {metrics.top_failing_steps.map((step) => (
                  <li key={step}>
                    <code>{step}</code>
                  </li>
                ))}
              </ul>
            </div>
          )}
        </div>
      )}
    </article>
  );
}

function Metric({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded border border-neutral-200 p-3 dark:border-neutral-800">
      <p className="text-xs uppercase tracking-wide opacity-60">{label}</p>
      <p className="mt-1 text-2xl font-semibold">{value}</p>
    </div>
  );
}

function AssetDetail({ asset }: { asset: Asset }) {
  const scores = asset.eval_scores ?? {};
  return (
    <article className="space-y-5 rounded border border-neutral-200 bg-white p-5 dark:border-neutral-800 dark:bg-neutral-900">
      <div className="flex flex-col gap-2 md:flex-row md:justify-between">
        <div>
          <h3 className="text-xl font-semibold">{asset.name}</h3>
          <p className="text-sm opacity-70">{asset.id}@{asset.version}</p>
        </div>
        <div className="flex gap-2 text-xs uppercase tracking-wide">
          <span className="rounded bg-neutral-100 px-2 py-1 dark:bg-neutral-800">{asset.lifecycle_state}</span>
          <span className="rounded bg-neutral-100 px-2 py-1 dark:bg-neutral-800">{asset.trust_level}</span>
        </div>
      </div>
      {asset.lifecycle_state === "deprecated" && <p className="rounded border border-yellow-300 bg-yellow-50 p-3 text-sm text-yellow-900 dark:border-yellow-800 dark:bg-yellow-950 dark:text-yellow-100">Deprecated asset. Prefer the recommended replacement in metadata before new adoption.</p>}
      <div className="grid gap-3 md:grid-cols-4">
        {["quality", "safety", "cost", "latency"].map((key) => <Score key={key} label={key} value={Number(scores[key] ?? 0)} />)}
      </div>
      <div className="rounded bg-neutral-950 p-4 text-xs text-neutral-100">
        <p className="font-medium">Production readiness</p>
        <p className="mt-1 opacity-80">{asset.lifecycle_state === "approved" ? "Invocable in production-relevant flows." : "Blocked in production-relevant flows until approved."}</p>
      </div>
    </article>
  );
}

function Score({ label, value }: { label: string; value: number }) {
  return (
    <div className="rounded border border-neutral-200 p-3 dark:border-neutral-800">
      <p className="text-xs uppercase tracking-wide opacity-60">{label}</p>
      <p className="mt-1 text-2xl font-semibold">{Math.round(value * 100)}%</p>
      <div className="mt-2 h-2 rounded bg-neutral-100 dark:bg-neutral-800"><div className="h-2 rounded bg-neutral-900 dark:bg-neutral-100" style={{ width: `${Math.max(0, Math.min(100, value * 100))}%` }} /></div>
    </div>
  );
}
