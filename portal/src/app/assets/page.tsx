import { authOptions } from "@/auth";
import { getServerSession } from "next-auth";
import { redirect } from "next/navigation";
import { PageHead } from "@/components/page/PageHead";
import { Button, Card } from "@/components/primitives";
import { ScopeSelect } from "@/components/scope/ScopeSelect";
import { RegisterButton } from "./RegisterButton";
import { LifecycleActions } from "./LifecycleActions";
import { MirroredVersionCard } from "@/components/assets/MirroredVersionCard";
import { PublicOriginBadge } from "@/components/assets/PublicOriginBadge";
import type { AssetKind } from "./RegisterDrawer";

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
  how_to?: HowToBlock | null;
  active_surface?: ActiveSurfaceBlock | null;
  provenance?: "internal" | "external";
  distribution?: {
    gateway_published?: boolean;
    gateway_channel?: string;
    package_digest?: string | null;
    package_signed_at?: string | null;
  };
  // Public-origin fields (Task 13.8 / 13.9)
  origin_ref?: string | null;
  is_public_origin?: boolean;
  last_synced_at?: string | null;
  auto_promote_policy?: "none" | "patch" | "minor" | "major";
};

type HowToBlock = {
  install?: Record<string, string>;
  usage?: Record<string, string>;
  env?: string[];
};

type ActiveSurfaceBlock = {
  family: "mcp" | "a2a" | "skill";
  endpoint?: string;
  artifact_pointer?: string;
  digest?: string;
  signature_id?: string;
};

type SearchParams = { workspace_id?: string; asset_id?: string; range?: string; kind?: string; tab?: string; filter?: string };

const VALID_KINDS: AssetKind[] = ["mcp", "skill", "agent", "workflow", "prompt_template"];

const KIND_HEADING: Record<AssetKind, { title: string; titleEm: string; sub: string }> = {
  skill: { title: "Skill", titleEm: "registry", sub: "Reusable capabilities your agents and workflows can call." },
  agent: { title: "Agent", titleEm: "registry", sub: "Autonomous agents with their inputs, evals and lifecycle." },
  mcp: { title: "MCP", titleEm: "registry", sub: "MCP tool servers exposed to agents and workflows." },
  workflow: { title: "Workflow", titleEm: "registry", sub: "Versioned workflows orchestrating agents, skills and tools." },
  prompt_template: { title: "Prompt", titleEm: "templates", sub: "Reusable prompt templates with parameter schemas." },
};

function parseKind(raw: string | undefined): AssetKind | undefined {
  if (!raw) return undefined;
  return (VALID_KINDS as string[]).includes(raw) ? (raw as AssetKind) : undefined;
}

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

async function fetchAssets(workspaceId: string, kind: AssetKind | undefined, token?: string) {
  const query = kind ? `?type=${encodeURIComponent(kind)}` : "";
  const response = await fetch(`${registryUrl()}/v1/workspaces/${workspaceId}/assets${query}`, {
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
  const kind = parseKind(searchParams.kind);
  let assets: Asset[] = [];
  let error: string | null = null;
  if (workspaceId) {
    try {
      assets = await fetchAssets(workspaceId, kind, token);
    } catch (e) {
      error = e instanceof Error ? e.message : "failed to load assets";
    }
  }
  const selected = assets.find((asset) => asset.id === searchParams.asset_id) ?? assets[0] ?? null;
  const metrics = selected ? await fetchAssetMetrics(selected.id, range, token) : null;
  const heading = kind ? KIND_HEADING[kind] : { title: "Asset", titleEm: "registry", sub: "Lifecycle, trust level, eval trend and production invocation readiness." };
  const linkParams = (extra: Record<string, string>) => {
    const params = new URLSearchParams();
    params.set("workspace_id", workspaceId);
    if (kind) params.set("kind", kind);
    for (const [k, v] of Object.entries(extra)) params.set(k, v);
    return params.toString();
  };

  return (
    <>
      <PageHead
        eyebrow={kind ? `Platform · ${heading.title} registry` : "Platform · Asset Registry"}
        title={heading.title}
        titleEm={heading.titleEm}
        sub={heading.sub}
        actions={
          <div style={{ display: "flex", gap: 8, alignItems: "center" }}>
            <form method="get" style={{ display: "flex", gap: 8 }}>
              {kind && <input type="hidden" name="kind" value={kind} />}
              <ScopeSelect kind="workspace" name="workspace_id" defaultValue={workspaceId} className="top-search" style={{ height: 32, width: 200 }} />
              <Button variant="secondary" type="submit">Load</Button>
            </form>
            <RegisterButton workspaceId={workspaceId} lockedKind={kind} />
          </div>
        }
      />
      {error && <Card style={{ marginBottom: 16 }}><div style={{ padding: 14, color: "var(--rust)" }}>{error}</div></Card>}
      <AssetListFilters filter={searchParams.filter ?? "all"} workspaceId={workspaceId} kind={kind} />
      <div className="grid gap-5 lg:grid-cols-[340px_1fr]">
        <aside className="space-y-2 rounded border border-neutral-200 bg-white p-4 dark:border-neutral-800 dark:bg-neutral-900">
          {applyListFilter(assets, searchParams.filter).map((asset) => (
            <a key={`${asset.id}@${asset.version}`} href={`/assets?${linkParams({ asset_id: asset.id })}`} className="block rounded border border-neutral-200 px-3 py-2 text-sm hover:bg-neutral-50 dark:border-neutral-800 dark:hover:bg-neutral-800">
              <span className="block font-medium">{asset.name}</span>
              <span className="text-xs opacity-60">{asset.type} · {asset.version} · {asset.lifecycle_state}</span>
              <AssetRowBadges asset={asset} />
            </a>
          ))}
          {assets.length === 0 && (
            <p className="text-sm opacity-70">
              {workspaceId ? (kind ? `No ${kind} assets in this workspace yet.` : "No assets loaded.") : "Paste a Workspace ID and press Load."}
            </p>
          )}
        </aside>
        {selected ? (
          <div className="space-y-5">
            <AssetDetail asset={selected} workspaceId={workspaceId} activeTab={searchParams.tab ?? "overview"} />
            {selected.lifecycle_state === "mirrored" && selected.is_public_origin && selected.origin_ref && (
              <MirroredVersionCard
                assetId={selected.id}
                version={selected.version}
                originRef={selected.origin_ref}
                lastSyncedAt={selected.last_synced_at ?? null}
                autoPromotePolicy={selected.auto_promote_policy ?? "none"}
              />
            )}
            <LifecycleActions
              assetId={selected.id}
              version={selected.version}
              lifecycleState={selected.lifecycle_state as Parameters<typeof LifecycleActions>[0]["lifecycleState"]}
              trustLevel={(selected.trust_level || "T0") as Parameters<typeof LifecycleActions>[0]["trustLevel"]}
              evalScores={selected.eval_scores ?? {}}
            />
            <ObservabilityTab asset={selected} metrics={metrics} workspaceId={workspaceId} range={range} />
          </div>
        ) : (
          <div className="rounded border border-dashed border-neutral-300 p-6 text-sm opacity-70 dark:border-neutral-800">
            {workspaceId ? "Select an asset to inspect lifecycle and evals." : "Load a workspace to see its assets."}
          </div>
        )}
      </div>
    </>
  );
}

// applyListFilter narrows the visible asset list per the user's chosen
// filter. Supported values mirror the FilterBar buttons.
function applyListFilter(assets: Asset[], filter: string | undefined): Asset[] {
  switch (filter) {
    case "approved":
      return assets.filter((a) => a.lifecycle_state === "approved");
    case "external":
      return assets.filter((a) => a.provenance === "external");
    case "published":
      return assets.filter((a) => a.distribution?.gateway_published === true);
    case "drift":
      return assets.filter((a) => assetHasDrift(a));
    case "missing-active-surface":
      return assets.filter((a) => !a.active_surface);
    case "all":
    default:
      return assets;
  }
}

function assetHasDrift(asset: Asset): boolean {
  const m = (asset.metadata ?? {}) as Record<string, any>;
  return Boolean(m.drift_detected) || Boolean(m.external_drift);
}

function AssetListFilters({ filter, workspaceId, kind }: { filter: string; workspaceId: string; kind?: string }) {
  const opts: { value: string; label: string }[] = [
    { value: "all", label: "All" },
    { value: "approved", label: "Approved" },
    { value: "published", label: "Gateway-published" },
    { value: "external", label: "External" },
    { value: "drift", label: "Drift" },
    { value: "missing-active-surface", label: "Missing active_surface" },
  ];
  return (
    <div className="mb-4 flex flex-wrap gap-2 text-xs">
      {opts.map((o) => {
        const params = new URLSearchParams();
        if (workspaceId) params.set("workspace_id", workspaceId);
        if (kind) params.set("kind", kind);
        if (o.value !== "all") params.set("filter", o.value);
        const active = filter === o.value;
        return (
          <a
            key={o.value}
            href={`/assets?${params.toString()}`}
            data-test-filter={o.value}
            className={
              active
                ? "rounded border border-neutral-900 bg-neutral-900 px-3 py-1 text-white dark:border-neutral-100 dark:bg-neutral-100 dark:text-neutral-900"
                : "rounded border border-neutral-300 px-3 py-1 text-neutral-700 hover:bg-neutral-50 dark:border-neutral-700 dark:text-neutral-200 dark:hover:bg-neutral-800"
            }
          >
            {o.label}
          </a>
        );
      })}
    </div>
  );
}

// AssetRowBadges renders the small badges next to each asset name in the
// list: provenance=external, gateway_published, drift indicator, and public origin.
function AssetRowBadges({ asset }: { asset: Asset }) {
  const badges: { label: string; tone: string; testid: string }[] = [];
  if (asset.provenance === "external") {
    badges.push({ label: "external", tone: "bg-indigo-100 text-indigo-800 dark:bg-indigo-900 dark:text-indigo-200", testid: "badge-external" });
  }
  if (asset.distribution?.gateway_published) {
    badges.push({ label: "gateway-published", tone: "bg-emerald-100 text-emerald-800 dark:bg-emerald-900 dark:text-emerald-200", testid: "badge-published" });
  }
  if (assetHasDrift(asset)) {
    badges.push({ label: "drift", tone: "bg-orange-100 text-orange-800 dark:bg-orange-900 dark:text-orange-200", testid: "badge-drift" });
  }
  if (!asset.active_surface) {
    badges.push({ label: "no active_surface", tone: "bg-amber-100 text-amber-800 dark:bg-amber-900 dark:text-amber-200", testid: "badge-missing-active-surface" });
  }
  const hasBadges = badges.length > 0 || asset.is_public_origin;
  if (!hasBadges) {
    return null;
  }
  return (
    <span className="mt-1 flex flex-wrap gap-1">
      {badges.map((b) => (
        <span key={b.label} data-testid={b.testid} className={`rounded px-1.5 py-0.5 text-[10px] font-medium uppercase tracking-wide ${b.tone}`}>
          {b.label}
        </span>
      ))}
      {asset.is_public_origin && (
        <PublicOriginBadge
          isPublicOrigin={true}
          originRef={asset.origin_ref}
          lastSyncedAt={asset.last_synced_at}
        />
      )}
    </span>
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

function AssetDetail({ asset, workspaceId, activeTab }: { asset: Asset; workspaceId: string; activeTab: string }) {
  const tabs = ["overview", "how-to", "gateway"] as const;
  type Tab = (typeof tabs)[number];
  const tab: Tab = (tabs as readonly string[]).includes(activeTab) ? (activeTab as Tab) : "overview";
  const tabHref = (t: Tab) => {
    const params = new URLSearchParams();
    if (workspaceId) params.set("workspace_id", workspaceId);
    params.set("asset_id", asset.id);
    if (t !== "overview") params.set("tab", t);
    return `/assets?${params.toString()}`;
  };
  return (
    <article className="space-y-5 rounded border border-neutral-200 bg-white p-5 dark:border-neutral-800 dark:bg-neutral-900">
      <div className="flex flex-col gap-2 md:flex-row md:justify-between">
        <div>
          <h3 className="text-xl font-semibold">{asset.name}</h3>
          <p className="text-sm opacity-70">{asset.id}@{asset.version}</p>
        </div>
        <div className="flex flex-wrap gap-2 text-xs uppercase tracking-wide">
          <span className="rounded bg-neutral-100 px-2 py-1 dark:bg-neutral-800">{asset.lifecycle_state}</span>
          <span className="rounded bg-neutral-100 px-2 py-1 dark:bg-neutral-800">{asset.trust_level}</span>
          {asset.provenance === "external" && (
            <span className="rounded bg-indigo-100 px-2 py-1 text-indigo-800 dark:bg-indigo-900 dark:text-indigo-200">external</span>
          )}
          {asset.is_public_origin && (
            <PublicOriginBadge
              isPublicOrigin={true}
              originRef={asset.origin_ref}
              lastSyncedAt={asset.last_synced_at}
            />
          )}
        </div>
      </div>
      {asset.lifecycle_state === "deprecated" && <p className="rounded border border-yellow-300 bg-yellow-50 p-3 text-sm text-yellow-900 dark:border-yellow-800 dark:bg-yellow-950 dark:text-yellow-100">Deprecated asset. Prefer the recommended replacement in metadata before new adoption.</p>}
      <nav className="flex gap-1 border-b border-neutral-200 dark:border-neutral-800" aria-label="Asset detail tabs">
        {tabs.map((t) => {
          const active = t === tab;
          return (
            <a
              key={t}
              href={tabHref(t)}
              data-testid={`tab-${t}`}
              data-active={active ? "true" : "false"}
              className={
                "px-3 py-2 text-sm capitalize " +
                (active
                  ? "border-b-2 border-neutral-900 font-medium dark:border-neutral-100"
                  : "border-b-2 border-transparent opacity-70 hover:opacity-100")
              }
            >
              {t === "how-to" ? "How-to" : t}
            </a>
          );
        })}
      </nav>
      {tab === "overview" && <OverviewTab asset={asset} />}
      {tab === "how-to" && <HowToTab block={asset.how_to ?? null} />}
      {tab === "gateway" && <GatewayTab asset={asset} />}
    </article>
  );
}

function OverviewTab({ asset }: { asset: Asset }) {
  const scores = asset.eval_scores ?? {};
  return (
    <div className="space-y-4">
      <div className="grid gap-3 md:grid-cols-4">
        {["quality", "safety", "cost", "latency"].map((key) => <Score key={key} label={key} value={Number(scores[key] ?? 0)} />)}
      </div>
      <div className="rounded bg-neutral-950 p-4 text-xs text-neutral-100">
        <p className="font-medium">Production readiness</p>
        <p className="mt-1 opacity-80">{asset.lifecycle_state === "approved" ? "Invocable in production-relevant flows." : "Blocked in production-relevant flows until approved."}</p>
      </div>
    </div>
  );
}

function HowToTab({ block }: { block: HowToBlock | null }) {
  if (!block || (Object.keys(block.install ?? {}).length === 0 && Object.keys(block.usage ?? {}).length === 0)) {
    return (
      <div className="rounded border border-dashed border-neutral-300 p-6 text-sm opacity-70 dark:border-neutral-800" data-testid="how-to-empty">
        No how-to block populated for this asset. Once the asset publisher provides install commands and usage snippets, they will render here. Promotion to <code>approved</code> requires this block to be filled in.
      </div>
    );
  }
  return (
    <div className="space-y-5" data-testid="how-to-tab">
      <Section title="Install per client">
        <KVList items={Object.entries(block.install ?? {})} mono />
      </Section>
      <Section title="Usage per language">
        <KVList items={Object.entries(block.usage ?? {})} block />
      </Section>
      {block.env && block.env.length > 0 && (
        <Section title="Environment variables">
          <ul className="list-inside list-disc text-sm">
            {block.env.map((v) => (
              <li key={v}><code>{v}</code></li>
            ))}
          </ul>
        </Section>
      )}
    </div>
  );
}

function GatewayTab({ asset }: { asset: Asset }) {
  const surface = asset.active_surface;
  if (!surface) {
    return (
      <div className="rounded border border-dashed border-neutral-300 p-6 text-sm opacity-70 dark:border-neutral-800" data-testid="gateway-empty">
        No active_surface populated. The asset is not reachable through a runtime gateway until this block is filled in. Promotion to <code>approved</code> requires it.
      </div>
    );
  }
  return (
    <div className="space-y-3" data-testid="gateway-tab">
      <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
        <KVRow label="Family" value={surface.family} />
        {surface.endpoint && <KVRow label="Endpoint" value={surface.endpoint} mono />}
        {surface.artifact_pointer && <KVRow label="Artifact pointer" value={surface.artifact_pointer} mono />}
        {surface.digest && <KVRow label="Digest" value={surface.digest} mono />}
        {surface.signature_id && <KVRow label="Signature" value={surface.signature_id} mono />}
        <KVRow label="Provenance" value={asset.provenance ?? "internal"} />
        {asset.distribution?.gateway_published && (
          <KVRow label="Gateway-published" value={`channel=${asset.distribution.gateway_channel ?? "stable"}`} />
        )}
      </div>
      <div className="rounded bg-neutral-50 p-4 text-xs dark:bg-neutral-950" data-testid="gateway-invocation-hint">
        <p className="font-medium">Runtime invocation</p>
        <p className="mt-1 opacity-80">
          {surface.family === "mcp" && <>Internal callers reach this MCP at <code>POST {surface.endpoint}</code> through mcp-gateway. The gateway injects signed identity headers and enforces OPA policy + Tenant budgets.</>}
          {surface.family === "a2a" && <>Internal callers reach this agent via <code>POST {surface.endpoint}</code> on a2a-gateway with the A2A JSON-RPC envelope.</>}
          {surface.family === "skill" && <>Consumers fetch this skill artifact through the configured artifact-store adapter from <code>{surface.artifact_pointer}</code>; the gateway verifies the digest against the registry record at fetch time.</>}
        </p>
      </div>
    </div>
  );
}

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div>
      <h4 className="mb-2 text-xs font-semibold uppercase tracking-wide opacity-70">{title}</h4>
      {children}
    </div>
  );
}

function KVList({ items, mono, block }: { items: [string, string][]; mono?: boolean; block?: boolean }) {
  if (items.length === 0) {
    return <p className="text-sm opacity-60">(none)</p>;
  }
  return (
    <dl className="space-y-2 text-sm">
      {items.map(([k, v]) => (
        <div key={k} className={block ? "" : "flex gap-3"}>
          <dt className="min-w-[120px] font-medium">{k}</dt>
          <dd className={mono ? "font-mono" : ""}>
            {block ? <pre className="overflow-x-auto rounded bg-neutral-50 p-2 dark:bg-neutral-950"><code>{v}</code></pre> : v}
          </dd>
        </div>
      ))}
    </dl>
  );
}

function KVRow({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="rounded border border-neutral-200 p-3 text-sm dark:border-neutral-800">
      <dt className="text-xs uppercase tracking-wide opacity-60">{label}</dt>
      <dd className={"mt-1 break-all " + (mono ? "font-mono" : "")}>{value}</dd>
    </div>
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
