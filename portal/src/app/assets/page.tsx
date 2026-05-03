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

type SearchParams = { workspace_id?: string; asset_id?: string };

const registryUrl = () => process.env.REGISTRY_URL ?? "http://localhost:8082";

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

export default async function AssetsPage({ searchParams }: { searchParams: SearchParams }) {
  const token = await getToken();
  const workspaceId = searchParams.workspace_id?.trim() ?? "";
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
        {selected ? <AssetDetail asset={selected} /> : <div className="rounded border border-dashed border-neutral-300 p-6 text-sm opacity-70 dark:border-neutral-800">Select an asset to inspect lifecycle and evals.</div>}
      </div>
    </section>
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
