// Pin assets step — Alfred Wizard · Pin assets.
// Implements active-registry-gateways §7.3. Surfaces three filterable
// catalogs (skills, MCPs, agents) sourced from the registry, lets the
// user pin a curated set, and POSTs the chosen ids back to the alfred
// draft so they travel with the OpenSpec into orchestration.

import { authOptions } from "@/auth";
import { getServerSession } from "next-auth";
import { redirect } from "next/navigation";
import { PageHead } from "@/components/page/PageHead";
import { Card } from "@/components/primitives";
import { endpoint } from "@/lib/api";

type Asset = {
  id: string;
  name: string;
  type: "mcp" | "skill" | "agent" | string;
  version: string;
  lifecycle_state: string;
  provenance?: string;
  description?: string;
  how_to?: Record<string, unknown> | null;
  active_surface?: Record<string, unknown> | null;
};

type SearchParams = {
  draft_id?: string;
  workspace_id?: string;
  filter_skill?: string;
  filter_mcp?: string;
  filter_agent?: string;
  error?: string;
  saved?: string;
};

async function getToken() {
  const session = await getServerSession(authOptions);
  if (!session) redirect("/api/auth/signin");
  return (session as { accessToken?: string }).accessToken;
}

async function fetchCatalog(workspaceId: string, token: string | undefined): Promise<Asset[]> {
  if (!workspaceId) return [];
  try {
    const r = await fetch(
      `${endpoint("REGISTRY_URL")}/v1/workspaces/${encodeURIComponent(workspaceId)}/assets?lifecycle_state=approved`,
      { headers: token ? { authorization: `Bearer ${token}` } : {}, cache: "no-store" },
    );
    if (!r.ok) return [];
    return (await r.json()) as Asset[];
  } catch {
    return [];
  }
}

async function fetchDraft(draftId: string, token: string | undefined): Promise<any> {
  const alfredURL = process.env.ALFRED_URL ?? "http://localhost:8090";
  try {
    const r = await fetch(`${alfredURL}/v1/intent/${draftId}`, {
      headers: token ? { authorization: `Bearer ${token}` } : {},
      cache: "no-store",
    });
    if (!r.ok) return null;
    return await r.json();
  } catch {
    return null;
  }
}

async function savePins(formData: FormData) {
  "use server";
  const token = (await getToken()) ?? "";
  const draftId = String(formData.get("draft_id") ?? "").trim();
  const workspaceId = String(formData.get("workspace_id") ?? "").trim();
  const skills = formData.getAll("pin_skill").map(String);
  const mcps = formData.getAll("pin_mcp").map(String);
  const agents = formData.getAll("pin_agent").map(String);

  if (!draftId) {
    redirect(`/alfred/wizard/pin-assets?error=${encodeURIComponent("missing draft_id")}`);
  }
  const alfredURL = process.env.ALFRED_URL ?? "http://localhost:8090";
  try {
    const r = await fetch(`${alfredURL}/v1/intent/${draftId}/answer`, {
      method: "POST",
      headers: {
        "content-type": "application/json",
        ...(token ? { authorization: `Bearer ${token}` } : {}),
      },
      body: JSON.stringify({
        answer: "pinned",
        field_updates: {
          selected_assets: { skills, mcps, agents },
        },
      }),
    });
    if (!r.ok) {
      const text = await r.text();
      redirect(`/alfred/wizard/pin-assets?draft_id=${draftId}&workspace_id=${workspaceId}&error=${encodeURIComponent(`alfred ${r.status}: ${text}`)}`);
    }
    redirect(`/alfred/wizard/pin-assets?draft_id=${draftId}&workspace_id=${workspaceId}&saved=1`);
  } catch (e: any) {
    redirect(`/alfred/wizard/pin-assets?draft_id=${draftId}&workspace_id=${workspaceId}&error=${encodeURIComponent(e?.message ?? "save failed")}`);
  }
}

export default async function PinAssetsPage({ searchParams }: { searchParams: SearchParams }) {
  const token = await getToken();
  const draftId = (searchParams.draft_id ?? "").trim();
  const workspaceId = (searchParams.workspace_id ?? "").trim();
  const assets = await fetchCatalog(workspaceId, token);
  const draft = draftId ? await fetchDraft(draftId, token) : null;
  const existing = draft?.draft?.selected_assets ?? draft?.selected_assets ?? { skills: [], mcps: [], agents: [] };

  const skills = filterAssets(assets, "skill", searchParams.filter_skill);
  const mcps = filterAssets(assets, "mcp", searchParams.filter_mcp);
  const agents = filterAssets(assets, "agent", searchParams.filter_agent);

  return (
    <>
      <PageHead
        eyebrow="Alfred · Wizard · Pin assets"
        title="Pin"
        titleEm="assets"
        sub="Select the skills, MCPs and agents Alfred is permitted to use when this OpenSpec runs. Empty selections let Alfred pick from the full approved catalog at orchestration time."
      />
      {searchParams.error && (
        <Card style={{ marginBottom: 16 }}>
          <div data-testid="error-banner" style={{ padding: 14, color: "var(--rust)" }}>{searchParams.error}</div>
        </Card>
      )}
      {searchParams.saved && (
        <Card style={{ marginBottom: 16 }}>
          <div data-testid="saved-banner" style={{ padding: 14 }}>Pinned assets saved on draft {draftId}.</div>
        </Card>
      )}
      <form action={savePins} data-testid="pin-form" className="space-y-6">
        <input type="hidden" name="draft_id" value={draftId} />
        <input type="hidden" name="workspace_id" value={workspaceId} />
        <PinList
          family="skill"
          label="Skills"
          inputName="pin_skill"
          filter={searchParams.filter_skill}
          filterKey="filter_skill"
          draftId={draftId}
          workspaceId={workspaceId}
          assets={skills}
          checked={new Set(existing.skills ?? [])}
        />
        <PinList
          family="mcp"
          label="MCPs"
          inputName="pin_mcp"
          filter={searchParams.filter_mcp}
          filterKey="filter_mcp"
          draftId={draftId}
          workspaceId={workspaceId}
          assets={mcps}
          checked={new Set(existing.mcps ?? [])}
        />
        <PinList
          family="agent"
          label="Agents"
          inputName="pin_agent"
          filter={searchParams.filter_agent}
          filterKey="filter_agent"
          draftId={draftId}
          workspaceId={workspaceId}
          assets={agents}
          checked={new Set(existing.agents ?? [])}
        />
        <div className="flex gap-3">
          <button
            type="submit"
            data-testid="pin-save"
            disabled={!draftId}
            className="rounded border border-neutral-900 bg-neutral-900 px-4 py-2 text-sm text-white disabled:opacity-50 dark:border-neutral-100 dark:bg-neutral-100 dark:text-neutral-900"
          >
            Save pinned assets
          </button>
          <a
            href={`/alfred/wizard?wizard=1&draft_id=${draftId}&workspace_id=${workspaceId}`}
            className="rounded border border-neutral-300 px-4 py-2 text-sm dark:border-neutral-700"
          >
            Back to wizard
          </a>
        </div>
      </form>
    </>
  );
}

function PinList({
  family,
  label,
  inputName,
  filter,
  filterKey,
  draftId,
  workspaceId,
  assets,
  checked,
}: {
  family: "skill" | "mcp" | "agent";
  label: string;
  inputName: string;
  filter: string | undefined;
  filterKey: string;
  draftId: string;
  workspaceId: string;
  assets: Asset[];
  checked: Set<string>;
}) {
  const url = (filterValue: string | null) => {
    const p = new URLSearchParams();
    if (draftId) p.set("draft_id", draftId);
    if (workspaceId) p.set("workspace_id", workspaceId);
    if (filterValue) p.set(filterKey, filterValue);
    return `/alfred/wizard/pin-assets?${p.toString()}`;
  };
  return (
    <section data-testid={`pin-list-${family}`} className="rounded border border-neutral-200 bg-white p-5 dark:border-neutral-800 dark:bg-neutral-900">
      <header className="mb-3 flex flex-wrap items-center justify-between gap-2">
        <h2 className="text-lg font-semibold">{label}</h2>
        <form method="get" className="flex items-center gap-2 text-xs">
          <input type="hidden" name="draft_id" value={draftId} />
          <input type="hidden" name="workspace_id" value={workspaceId} />
          <label className="flex items-center gap-1">
            <span className="opacity-70">filter</span>
            <input
              type="search"
              name={filterKey}
              defaultValue={filter ?? ""}
              placeholder="name or id"
              data-testid={`filter-${family}`}
              className="rounded border border-neutral-300 bg-transparent px-2 py-1 dark:border-neutral-700"
            />
          </label>
          <button type="submit" className="rounded border border-neutral-300 px-2 py-1 dark:border-neutral-700">Apply</button>
          {filter && (
            <a href={url(null)} className="opacity-70 underline">Clear</a>
          )}
        </form>
      </header>
      {assets.length === 0 ? (
        <p className="rounded border border-dashed border-neutral-300 p-4 text-sm opacity-70 dark:border-neutral-800">
          No approved {family} assets {filter ? "match this filter" : "in this Workspace yet"}.
        </p>
      ) : (
        <ul className="space-y-1 text-sm">
          {assets.map((a) => (
            <li key={a.id + "@" + a.version}>
              <label className="flex cursor-pointer items-start gap-3 rounded border border-transparent px-2 py-1 hover:border-neutral-200 hover:bg-neutral-50 dark:hover:border-neutral-700 dark:hover:bg-neutral-800">
                <input
                  type="checkbox"
                  name={inputName}
                  value={a.id}
                  defaultChecked={checked.has(a.id)}
                  data-testid={`pin-checkbox-${a.id}`}
                  className="mt-1"
                />
                <span className="flex-1">
                  <span className="block font-medium">{a.name}</span>
                  <span className="block text-xs opacity-60">
                    {a.id} · {a.version}
                    {a.provenance === "external" && " · external"}
                  </span>
                  {a.description && <span className="block text-xs opacity-70">{a.description}</span>}
                </span>
              </label>
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}

function filterAssets(assets: Asset[], family: "skill" | "mcp" | "agent", filter: string | undefined): Asset[] {
  const filtered = assets.filter((a) => a.type === family);
  if (!filter || !filter.trim()) return filtered;
  const needle = filter.trim().toLowerCase();
  return filtered.filter((a) => a.id.toLowerCase().includes(needle) || a.name.toLowerCase().includes(needle));
}
