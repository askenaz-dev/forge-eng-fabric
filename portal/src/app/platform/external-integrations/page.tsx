// External integrations view — Platform · External integrations.
// Lists registered external MCPs and external A2A partners per Workspace,
// surfaces drift state, lets operators register a new external endpoint
// and manage credential refs. Implements active-registry-gateways §7.2.
//
// The page is a server component that fetches from services/registry's
// /v1/registry/{mcps,agents}/external?workspace_id=... endpoints (the
// registry list is workspace-scoped) and from services/a2a-gateway's
// /v1/gw/a2a/partners endpoint for inbound partner enrollment.

import { authOptions } from "@/auth";
import { getServerSession } from "next-auth";
import { redirect } from "next/navigation";
import { randomUUID } from "crypto";
import { PageHead } from "@/components/page/PageHead";
import { Card } from "@/components/primitives";
import { ScopeSelect } from "@/components/scope/ScopeSelect";
import { endpoint } from "@/lib/api";

type ExternalAsset = {
  id: string;
  type: "mcp" | "agent";
  name: string;
  version: string;
  lifecycle_state: string;
  provenance: "external";
  endpoint?: string;
  credential_ref?: string;
  allowlist?: string[];
  task_allowlist?: string[];
  manifest_hash?: string;
  agent_card_hash?: string;
  manifest_fetched_at?: string;
  agent_card_fetched_at?: string;
  metadata?: Record<string, unknown>;
};

type Partner = {
  name: string;
  tenant_id: string;
  workspace_id?: string;
  allowed_assets: string[];
  created_at: string;
  created_by?: string;
};

type SearchParams = {
  workspace_id?: string;
  error?: string;
  registered?: string;
};

async function getToken() {
  const session = await getServerSession(authOptions);
  if (!session) redirect("/api/auth/signin");
  return (session as { accessToken?: string }).accessToken;
}

async function fetchExternalAssets(workspaceId: string, token: string | undefined): Promise<ExternalAsset[]> {
  if (!workspaceId) return [];
  try {
    const r = await fetch(
      `${endpoint("REGISTRY_URL")}/v1/workspaces/${encodeURIComponent(workspaceId)}/assets?provenance=external`,
      { headers: token ? { authorization: `Bearer ${token}` } : {}, cache: "no-store" },
    );
    if (!r.ok) return [];
    return (await r.json()) as ExternalAsset[];
  } catch {
    return [];
  }
}

async function fetchPartners(token: string | undefined): Promise<Partner[]> {
  const a2aURL = process.env.A2A_GATEWAY_URL ?? "http://localhost:8093";
  try {
    const r = await fetch(`${a2aURL}/v1/gw/a2a/partners`, {
      headers: token ? { authorization: `Bearer ${token}` } : {},
      cache: "no-store",
    });
    if (!r.ok) return [];
    const body = (await r.json()) as { items?: Partner[] };
    return body.items ?? [];
  } catch {
    return [];
  }
}

async function registerExternalMCP(formData: FormData) {
  "use server";
  const token = (await getToken()) ?? "";
  const workspaceId = String(formData.get("workspace_id") ?? "").trim();
  const name = String(formData.get("name") ?? "").trim();
  const version = String(formData.get("version") ?? "0.1.0").trim();
  const endpointURL = String(formData.get("endpoint_url") ?? "").trim();
  const credentialRef = String(formData.get("credential_ref") ?? "").trim();
  const allowlistRaw = String(formData.get("allowlist") ?? "").trim();
  if (!workspaceId || !name || !endpointURL || !credentialRef) {
    redirect(`/platform/external-integrations?workspace_id=${workspaceId}&error=${encodeURIComponent("workspace_id, name, endpoint_url and credential_ref are required")}`);
  }
  try {
    const r = await fetch(`${endpoint("REGISTRY_URL")}/v1/registry/mcps/external`, {
      method: "POST",
      headers: {
        "content-type": "application/json",
        "x-correlation-id": randomUUID(),
        ...(token ? { authorization: `Bearer ${token}` } : {}),
      },
      body: JSON.stringify({
        workspace_id: workspaceId,
        name,
        version,
        owner_team: "platform-integrations",
        endpoint_url: endpointURL,
        credential_ref: credentialRef,
        allowlist: allowlistRaw ? allowlistRaw.split(",").map((s) => s.trim()).filter(Boolean) : [],
      }),
    });
    if (!r.ok) {
      const text = await r.text();
      redirect(`/platform/external-integrations?workspace_id=${workspaceId}&error=${encodeURIComponent(`registry ${r.status}: ${text}`)}`);
    }
    redirect(`/platform/external-integrations?workspace_id=${workspaceId}&registered=${encodeURIComponent("mcp:" + name)}`);
  } catch (e: any) {
    redirect(`/platform/external-integrations?workspace_id=${workspaceId}&error=${encodeURIComponent(e?.message ?? "fetch failed")}`);
  }
}

async function registerExternalA2A(formData: FormData) {
  "use server";
  const token = (await getToken()) ?? "";
  const workspaceId = String(formData.get("workspace_id") ?? "").trim();
  const name = String(formData.get("name") ?? "").trim();
  const version = String(formData.get("version") ?? "0.1.0").trim();
  const endpointURL = String(formData.get("endpoint_url") ?? "").trim();
  const credentialRef = String(formData.get("credential_ref") ?? "").trim();
  const allowlistRaw = String(formData.get("allowlist") ?? "").trim();
  if (!workspaceId || !name || !endpointURL || !credentialRef) {
    redirect(`/platform/external-integrations?workspace_id=${workspaceId}&error=${encodeURIComponent("workspace_id, name, endpoint_url and credential_ref are required")}`);
  }
  try {
    const r = await fetch(`${endpoint("REGISTRY_URL")}/v1/registry/agents/external`, {
      method: "POST",
      headers: {
        "content-type": "application/json",
        "x-correlation-id": randomUUID(),
        ...(token ? { authorization: `Bearer ${token}` } : {}),
      },
      body: JSON.stringify({
        workspace_id: workspaceId,
        name,
        version,
        owner_team: "platform-integrations",
        endpoint_url: endpointURL,
        credential_ref: credentialRef,
        allowlist: allowlistRaw ? allowlistRaw.split(",").map((s) => s.trim()).filter(Boolean) : [],
      }),
    });
    if (!r.ok) {
      const text = await r.text();
      redirect(`/platform/external-integrations?workspace_id=${workspaceId}&error=${encodeURIComponent(`registry ${r.status}: ${text}`)}`);
    }
    redirect(`/platform/external-integrations?workspace_id=${workspaceId}&registered=${encodeURIComponent("a2a:" + name)}`);
  } catch (e: any) {
    redirect(`/platform/external-integrations?workspace_id=${workspaceId}&error=${encodeURIComponent(e?.message ?? "fetch failed")}`);
  }
}

export default async function ExternalIntegrationsPage({ searchParams }: { searchParams: SearchParams }) {
  const token = await getToken();
  const workspaceId = searchParams.workspace_id?.trim() ?? "";
  const externals = await fetchExternalAssets(workspaceId, token);
  const partners = await fetchPartners(token);

  return (
    <>
      <PageHead
        eyebrow="Platform · External integrations"
        title="External"
        titleEm="integrations"
        sub="Register vendor MCPs and partner A2A agents per Workspace. Credentials live in vault and are brokered at invocation time."
        actions={
          <form method="get" style={{ display: "flex", gap: 8 }}>
            <ScopeSelect kind="workspace" name="workspace_id" defaultValue={workspaceId} className="top-search" style={{ height: 32, width: 200 }} />
            <button className="rounded border border-neutral-300 px-3 py-1 text-sm dark:border-neutral-700" type="submit">Load</button>
          </form>
        }
      />
      {searchParams.error && (
        <Card style={{ marginBottom: 16 }}>
          <div data-testid="error-banner" style={{ padding: 14, color: "var(--rust)" }}>{searchParams.error}</div>
        </Card>
      )}
      {searchParams.registered && (
        <Card style={{ marginBottom: 16 }}>
          <div data-testid="registered-banner" style={{ padding: 14 }}>
            Registered {searchParams.registered}.
          </div>
        </Card>
      )}
      <div className="grid gap-6 lg:grid-cols-2">
        <Panel
          title="External MCPs"
          assets={externals.filter((a) => a.type === "mcp")}
          formAction={registerExternalMCP}
          workspaceId={workspaceId}
          testid="external-mcps"
          allowlistLabel="Tool allowlist (comma-separated)"
        />
        <Panel
          title="External A2A agents"
          assets={externals.filter((a) => a.type === "agent")}
          formAction={registerExternalA2A}
          workspaceId={workspaceId}
          testid="external-a2a"
          allowlistLabel="Task allowlist (comma-separated)"
        />
      </div>
      <section className="mt-8" data-testid="partners-section">
        <h2 className="mb-2 text-lg font-semibold">Inbound partners (A2A)</h2>
        <p className="mb-3 text-sm opacity-70">
          External agents enrolled to invoke this Tenant&apos;s internal agents. Each partner carries a vault-backed credential that the gateway verifies on every inbound call. Empty <code>allowed_assets</code> means deny-all.
        </p>
        {partners.length === 0 ? (
          <p className="rounded border border-dashed border-neutral-300 p-4 text-sm opacity-70 dark:border-neutral-800">
            No partners enrolled yet. Use the a2a-gateway enrollment endpoint to add one.
          </p>
        ) : (
          <ul className="space-y-2" data-testid="partners-list">
            {partners.map((p) => (
              <li key={p.tenant_id + "/" + p.name} className="rounded border border-neutral-200 p-3 text-sm dark:border-neutral-800">
                <div className="flex justify-between">
                  <span className="font-medium">{p.name}</span>
                  <span className="text-xs opacity-60">enrolled {new Date(p.created_at).toLocaleString()}</span>
                </div>
                <div className="mt-1 text-xs opacity-70">
                  workspace={p.workspace_id || "(any)"} · allowed: {p.allowed_assets.length === 0 ? <em>(none — deny-all)</em> : p.allowed_assets.join(", ")}
                </div>
              </li>
            ))}
          </ul>
        )}
      </section>
    </>
  );
}

function Panel({
  title,
  assets,
  formAction,
  workspaceId,
  testid,
  allowlistLabel,
}: {
  title: string;
  assets: ExternalAsset[];
  formAction: (fd: FormData) => Promise<void>;
  workspaceId: string;
  testid: string;
  allowlistLabel: string;
}) {
  return (
    <section data-testid={testid} className="rounded border border-neutral-200 bg-white p-5 dark:border-neutral-800 dark:bg-neutral-900">
      <h2 className="mb-3 text-lg font-semibold">{title}</h2>
      {assets.length === 0 ? (
        <p className="rounded border border-dashed border-neutral-300 p-4 text-sm opacity-70 dark:border-neutral-800">
          No external integrations registered for this Workspace yet.
        </p>
      ) : (
        <ul className="space-y-3" data-testid={`${testid}-list`}>
          {assets.map((asset) => (
            <li key={asset.id} className="rounded border border-neutral-200 p-3 text-sm dark:border-neutral-800">
              <div className="flex flex-wrap items-center gap-2">
                <span className="font-medium">{asset.name}</span>
                <span className="rounded bg-neutral-100 px-2 py-0.5 text-xs uppercase tracking-wide dark:bg-neutral-800">{asset.lifecycle_state}</span>
                {hasDrift(asset) && (
                  <span data-testid="drift-badge" className="rounded bg-orange-100 px-2 py-0.5 text-xs uppercase tracking-wide text-orange-800 dark:bg-orange-900 dark:text-orange-200">
                    drift
                  </span>
                )}
              </div>
              <div className="mt-1 grid gap-1 text-xs opacity-70 md:grid-cols-2">
                <div>endpoint: <code>{asset.endpoint ?? "(unknown)"}</code></div>
                <div>credential: <code>{asset.credential_ref ?? "(unknown)"}</code></div>
                <div>last-verified: {fmtDate(asset.manifest_fetched_at ?? asset.agent_card_fetched_at)}</div>
                <div>
                  allowlist:{" "}
                  {(asset.allowlist ?? asset.task_allowlist ?? []).length === 0 ? (
                    <em>(none — deny-all)</em>
                  ) : (
                    (asset.allowlist ?? asset.task_allowlist ?? []).join(", ")
                  )}
                </div>
              </div>
            </li>
          ))}
        </ul>
      )}
      <form action={formAction} className="mt-5 space-y-3" data-testid={`${testid}-form`}>
        <input type="hidden" name="workspace_id" value={workspaceId} />
        <div className="grid gap-2 md:grid-cols-2">
          <Field name="name" label="Name" placeholder="vendor-x" required />
          <Field name="version" label="Version" placeholder="0.1.0" defaultValue="0.1.0" required />
          <Field
            name="endpoint_url"
            label="Endpoint URL"
            placeholder="https://vendor-x.example.com/mcp"
            className="md:col-span-2"
            required
          />
          <Field
            name="credential_ref"
            label="Credential ref"
            placeholder="vault://kv/forge/vendor-x"
            className="md:col-span-2"
            pattern="^(vault|aws-sm|gcp-sm|azure-kv)://.+"
            required
          />
          <Field name="allowlist" label={allowlistLabel} placeholder="read_doc, list_docs" className="md:col-span-2" />
        </div>
        <button
          type="submit"
          disabled={!workspaceId}
          className="rounded border border-neutral-900 bg-neutral-900 px-4 py-1.5 text-sm text-white disabled:opacity-50 dark:border-neutral-100 dark:bg-neutral-100 dark:text-neutral-900"
        >
          Register
        </button>
      </form>
    </section>
  );
}

function Field(props: {
  name: string;
  label: string;
  placeholder?: string;
  defaultValue?: string;
  className?: string;
  pattern?: string;
  required?: boolean;
}) {
  return (
    <label className={"flex flex-col text-xs " + (props.className ?? "")}>
      <span className="mb-1 font-medium opacity-70">{props.label}</span>
      <input
        type="text"
        name={props.name}
        placeholder={props.placeholder}
        defaultValue={props.defaultValue}
        pattern={props.pattern}
        required={props.required}
        className="rounded border border-neutral-300 bg-transparent px-2 py-1 dark:border-neutral-700"
      />
    </label>
  );
}

function hasDrift(asset: ExternalAsset): boolean {
  const m = (asset.metadata ?? {}) as Record<string, any>;
  return Boolean(m.drift_detected) || asset.lifecycle_state === "deprecated";
}

function fmtDate(raw: string | undefined): string {
  if (!raw) return "(never)";
  try {
    return new Date(raw).toLocaleString();
  } catch {
    return raw;
  }
}
