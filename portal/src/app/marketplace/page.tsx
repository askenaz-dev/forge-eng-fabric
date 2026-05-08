import { authOptions } from "@/auth";
import { getServerSession } from "next-auth";
import { redirect } from "next/navigation";

type Listing = {
  id: string;
  tenant_id: string;
  workspace_id: string;
  workflow_id: string;
  version: string;
  name: string;
  description: string;
  tags?: string[];
  criticality?: string;
  visibility: string;
  approval_state: string;
  eval_run_id?: string;
  eval_outcome?: string;
};

type SearchParams = {
  tenant_id?: string;
  workspace_id?: string;
  visibility?: string;
  q?: string;
  installed?: string;
  error?: string;
};

const marketplaceUrl = () => process.env.MARKETPLACE_URL ?? "http://localhost:8095";

async function getToken() {
  const session = await getServerSession(authOptions);
  if (!session) redirect("/api/auth/signin");
  return (session as { accessToken?: string }).accessToken;
}

async function fetchListings(params: SearchParams, token?: string) {
  const url = new URL(`${marketplaceUrl()}/v1/marketplace`);
  if (params.tenant_id) url.searchParams.set("tenant_id", params.tenant_id);
  if (params.workspace_id) url.searchParams.set("workspace_id", params.workspace_id);
  if (params.visibility) url.searchParams.set("visibility", params.visibility);
  if (params.q) url.searchParams.set("q", params.q);
  const response = await fetch(url, {
    headers: token ? { authorization: `Bearer ${token}` } : {},
    cache: "no-store",
  });
  if (!response.ok) throw new Error(`marketplace ${response.status}: ${await response.text()}`);
  return ((await response.json()) as { listings: Listing[] }).listings;
}

async function installListing(formData: FormData) {
  "use server";
  const token = await getToken();
  const tenantId = required(formData, "tenant_id");
  const listingId = required(formData, "listing_id");
  const target = required(formData, "target_workspace_id");
  const response = await fetch(`${marketplaceUrl()}/v1/marketplace/install`, {
    method: "POST",
    headers: { "content-type": "application/json", ...(token ? { authorization: `Bearer ${token}` } : {}) },
    body: JSON.stringify({ tenant_id: tenantId, listing_id: listingId, target_workspace_id: target, actor: "portal" }),
  });
  if (!response.ok) {
    redirect(`/marketplace?tenant_id=${tenantId}&workspace_id=${target}&error=${encodeURIComponent(await response.text())}`);
  }
  redirect(`/marketplace?tenant_id=${tenantId}&workspace_id=${target}&installed=1`);
}

export default async function MarketplacePage({ searchParams }: { searchParams: SearchParams }) {
  const token = await getToken();
  const tenantId = searchParams.tenant_id?.trim() ?? "";
  const workspaceId = searchParams.workspace_id?.trim() ?? "";

  let listings: Listing[] = [];
  let error: string | null = searchParams.error ?? null;

  if (tenantId) {
    try {
      listings = await fetchListings(searchParams, token);
    } catch (e) {
      error = e instanceof Error ? e.message : "failed to load marketplace";
    }
  }

  return (
    <section className="space-y-6">
      <div className="flex flex-col gap-3 md:flex-row md:items-end md:justify-between">
        <div>
          <h2 className="text-2xl font-semibold">Marketplace</h2>
          <p className="mt-1 text-sm opacity-70">Browse workflows shared in your Tenant. Install pins to an exact version.</p>
        </div>
        <form className="flex flex-wrap gap-2" method="get">
          <input
            name="tenant_id"
            defaultValue={tenantId}
            placeholder="Tenant ID"
            className="min-w-0 rounded border border-neutral-300 bg-transparent px-3 py-2 text-sm dark:border-neutral-700"
          />
          <input
            name="workspace_id"
            defaultValue={workspaceId}
            placeholder="Workspace ID"
            className="min-w-0 rounded border border-neutral-300 bg-transparent px-3 py-2 text-sm dark:border-neutral-700"
          />
          <select
            name="visibility"
            defaultValue={searchParams.visibility ?? ""}
            className="rounded border border-neutral-300 bg-transparent px-2 py-2 text-sm dark:border-neutral-700"
          >
            <option value="">any visibility</option>
            <option value="workspace">workspace</option>
            <option value="tenant">tenant</option>
            <option value="forge-certified">forge-certified</option>
          </select>
          <input
            name="q"
            defaultValue={searchParams.q ?? ""}
            placeholder="search"
            className="rounded border border-neutral-300 bg-transparent px-3 py-2 text-sm dark:border-neutral-700"
          />
          <button className="rounded bg-neutral-900 px-4 py-2 text-sm text-white dark:bg-neutral-100 dark:text-neutral-900">Search</button>
        </form>
      </div>

      {searchParams.installed && (
        <p className="rounded border border-green-300 bg-green-50 p-3 text-sm text-green-800 dark:border-green-800 dark:bg-green-950 dark:text-green-200">
          Installed.
        </p>
      )}
      {error && (
        <p className="rounded border border-red-300 bg-red-50 p-3 text-sm text-red-800 dark:border-red-800 dark:bg-red-950 dark:text-red-200">
          {error}
        </p>
      )}

      <ul className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
        {listings.map((l) => (
          <li key={l.id} className="rounded border border-neutral-200 bg-white p-4 dark:border-neutral-800 dark:bg-neutral-900">
            <header className="flex items-start justify-between gap-2">
              <div>
                <h3 className="font-medium">{l.name}</h3>
                <p className="text-xs opacity-60">
                  {l.workflow_id}@{l.version}
                </p>
              </div>
              <span className="rounded bg-neutral-100 px-2 py-0.5 text-xs dark:bg-neutral-800">{l.visibility}</span>
            </header>
            <p className="mt-2 text-sm">{l.description}</p>
            {l.tags && l.tags.length > 0 && (
              <p className="mt-2 text-xs opacity-70">{l.tags.join(" · ")}</p>
            )}
            {l.eval_outcome && (
              <p className="mt-1 text-xs">
                eval: <strong>{l.eval_outcome}</strong>
                {l.eval_run_id ? ` · ${l.eval_run_id}` : ""}
              </p>
            )}
            <p className="mt-1 text-xs opacity-60">approval: {l.approval_state}</p>
            <form action={installListing} className="mt-3 flex flex-wrap gap-2 text-sm">
              <input type="hidden" name="tenant_id" value={l.tenant_id} />
              <input type="hidden" name="listing_id" value={l.id} />
              <input
                name="target_workspace_id"
                defaultValue={workspaceId}
                placeholder="target workspace"
                required
                className="min-w-0 rounded border border-neutral-300 bg-transparent px-2 py-1 dark:border-neutral-700"
              />
              <button className="rounded bg-neutral-900 px-3 py-1 text-white dark:bg-neutral-100 dark:text-neutral-900">Install</button>
            </form>
          </li>
        ))}
        {tenantId && listings.length === 0 && !error && (
          <li className="rounded border border-dashed border-neutral-300 p-6 text-sm opacity-70 dark:border-neutral-800">
            No listings match. Publish a workflow at visibility=tenant to surface it here.
          </li>
        )}
      </ul>
    </section>
  );
}

function required(formData: FormData, key: string) {
  const v = String(formData.get(key) ?? "").trim();
  if (!v) throw new Error(`${key} is required`);
  return v;
}
