import { authOptions } from "@/auth";
import { getServerSession } from "next-auth";
import { redirect } from "next/navigation";
import { PageHead } from "@/components/page/PageHead";
import { Badge, Button, Card } from "@/components/primitives";
import { ScopeSelect } from "@/components/scope/ScopeSelect";

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
    <>
      <PageHead
        eyebrow="Platform · Marketplace"
        title="Forge"
        titleEm="marketplace"
        sub="Browse workflows shared in your Tenant. Install pins to an exact version."
        actions={
          <form method="get" style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
            <ScopeSelect kind="tenant" name="tenant_id" defaultValue={tenantId} className="top-search" style={{ height: 32, width: 160 }} />
            <ScopeSelect kind="workspace" name="workspace_id" defaultValue={workspaceId} className="top-search" style={{ height: 32, width: 180 }} />
            <select
              name="visibility"
              defaultValue={searchParams.visibility ?? ""}
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
              <option value="">any visibility</option>
              <option value="workspace">workspace</option>
              <option value="tenant">tenant</option>
              <option value="forge-certified">forge-certified</option>
            </select>
            <input name="q" defaultValue={searchParams.q ?? ""} placeholder="search" className="top-search" style={{ height: 32, width: 160 }} />
            <Button variant="primary" type="submit">Search</Button>
          </form>
        }
      />

      {searchParams.installed && (
        <Card style={{ marginBottom: 16 }}><div style={{ padding: 14, color: "var(--thread)" }}>Installed.</div></Card>
      )}
      {error && (
        <Card style={{ marginBottom: 16 }}><div style={{ padding: 14, color: "var(--rust)" }}>{error}</div></Card>
      )}

      <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fill, minmax(280px, 1fr))", gap: 12 }}>
        {listings.map((l) => (
          <Card key={l.id}>
            <div style={{ padding: 14, display: "flex", flexDirection: "column", gap: 8 }}>
              <div style={{ display: "flex", justifyContent: "space-between", gap: 8, alignItems: "flex-start" }}>
                <div>
                  <h3 style={{ fontFamily: "var(--f-display)", fontStyle: "italic", fontSize: 20, margin: 0, letterSpacing: "-0.015em" }}>{l.name}</h3>
                  <p style={{ fontFamily: "var(--f-mono)", fontSize: 11, color: "var(--fg-3)", margin: "2px 0 0" }}>{l.workflow_id}@{l.version}</p>
                </div>
                <Badge>{l.visibility}</Badge>
              </div>
              <p style={{ fontSize: 13, color: "var(--fg-2)", margin: 0 }}>{l.description}</p>
              {l.tags && l.tags.length > 0 && <p style={{ fontFamily: "var(--f-mono)", fontSize: 11, color: "var(--fg-3)", margin: 0 }}>{l.tags.join(" · ")}</p>}
              {l.eval_outcome && (
                <p style={{ fontFamily: "var(--f-mono)", fontSize: 11, margin: 0 }}>
                  eval: <strong style={{ color: "var(--fg)" }}>{l.eval_outcome}</strong>
                  {l.eval_run_id ? ` · ${l.eval_run_id}` : ""}
                </p>
              )}
              <p style={{ fontFamily: "var(--f-mono)", fontSize: 11, color: "var(--fg-3)", margin: 0 }}>approval: {l.approval_state}</p>
              <form action={installListing} style={{ display: "flex", gap: 6, marginTop: 6 }}>
                <input type="hidden" name="tenant_id" value={l.tenant_id} />
                <input type="hidden" name="listing_id" value={l.id} />
                <input
                  name="target_workspace_id"
                  defaultValue={workspaceId}
                  placeholder="target workspace"
                  required
                  className="top-search"
                  style={{ height: 28, minWidth: 0, flex: 1 }}
                />
                <Button variant="primary" size="xs" type="submit">Install</Button>
              </form>
            </div>
          </Card>
        ))}
        {tenantId && listings.length === 0 && !error && (
          <Card>
            <div className="note" style={{ padding: 24, textAlign: "center" }}>
              No listings match. Publish a workflow at visibility=tenant to surface it here.
            </div>
          </Card>
        )}
      </div>
    </>
  );
}

function required(formData: FormData, key: string) {
  const v = String(formData.get(key) ?? "").trim();
  if (!v) throw new Error(`${key} is required`);
  return v;
}
