import { PageHead } from "@/components/page/PageHead";
import { Card } from "@/components/primitives";
import { ScopeSelect } from "@/components/scope/ScopeSelect";
import { authToken, endpoint, proxyJson } from "@/lib/api";
import { RevokeButton } from "./RevokeButton";

type Asset = {
  id: string;
  version: string;
  type: string;
  name: string;
  trust_level: string;
  distribution: {
    gateway_published: boolean;
    gateway_channel: string;
    package_digest: string | null;
    package_signed_at: string | null;
  };
};

type TokenRow = {
  id: string;
  developer_sub: string;
  assume_workspace_id: string;
  scopes: string[];
  asset_allowlist: string[];
  created_at: string;
  expires_at: string;
  last_used_at: string | null;
  revoked_at: string | null;
};

async function fetchAssets(workspaceId: string, token?: string) {
  if (!workspaceId) return [] as Asset[];
  try {
    return await proxyJson<Asset[]>(
      `${endpoint("REGISTRY_URL")}/v1/workspaces/${encodeURIComponent(workspaceId)}/assets`,
      { token, method: "GET" },
    );
  } catch {
    return [] as Asset[];
  }
}

async function fetchTokens(_token?: string): Promise<TokenRow[]> {
  // The gateway's token listing endpoint is admin-only; for now return [].
  // Wired when /v1/gateway/tokens grows a GET handler.
  return [];
}

export default async function GatewayPage({ searchParams }: { searchParams: { workspace_id?: string } }) {
  const { token } = await authToken();
  const workspaceId = (searchParams.workspace_id ?? "").trim();
  const assets = await fetchAssets(workspaceId, token);
  const published = assets.filter((a) => a.distribution?.gateway_published);
  const tokens = await fetchTokens(token);

  return (
    <>
      <PageHead
        eyebrow="Platform · Skill gateway"
        title="Skill"
        titleEm="gateway"
        sub="Developer-facing distribution: which assets are installable, by whom, and where they were used."
        actions={
          <form method="get" style={{ display: "flex", gap: 8 }}>
            <ScopeSelect kind="workspace" name="workspace_id" defaultValue={workspaceId} className="top-search" style={{ height: 32, width: 200 }} />
            <button type="submit" className="btn btn--secondary">Load</button>
          </form>
        }
      />
      <div className="grid gap-5 lg:grid-cols-[1fr_360px]">
        <section className="space-y-4">
          <Card>
            <div className="p-4">
              <h2 className="text-lg font-semibold">Published assets</h2>
              <p className="text-sm opacity-70">Approved, T1+, with a signed bundle or remote-transport contract.</p>
            </div>
            <div className="border-t border-neutral-200 dark:border-neutral-800">
              {published.length === 0 && (
                <p className="p-4 text-sm opacity-60">
                  {workspaceId ? "No assets are gateway-published in this workspace yet." : "Paste a Workspace ID and press Load."}
                </p>
              )}
              {published.map((a) => (
                <div key={`${a.id}@${a.version}`} className="flex items-start justify-between gap-4 border-b border-neutral-200 px-4 py-3 last:border-0 dark:border-neutral-800">
                  <div className="min-w-0">
                    <p className="font-medium">{a.name} <span className="opacity-50 font-normal">@ {a.version}</span></p>
                    <p className="text-xs opacity-60">{a.type} · {a.trust_level} · channel {a.distribution.gateway_channel}</p>
                    {a.distribution.package_digest && (
                      <p className="mt-1 truncate font-mono text-xs opacity-60">{a.distribution.package_digest}</p>
                    )}
                  </div>
                  <pre className="rounded bg-neutral-950 px-3 py-2 text-xs text-neutral-100">
forge skills install {a.name}@{a.version}
                  </pre>
                </div>
              ))}
            </div>
          </Card>
        </section>

        <aside className="space-y-4">
          <Card>
            <div className="p-4">
              <h2 className="text-lg font-semibold">Personal access tokens</h2>
              <p className="text-sm opacity-70">Revoke leaked tokens immediately. Token issuance happens in the CLI.</p>
            </div>
            <div className="border-t border-neutral-200 dark:border-neutral-800">
              {tokens.length === 0 && (
                <p className="p-4 text-sm opacity-60">No token rows visible from this view.</p>
              )}
              {tokens.map((t) => (
                <div key={t.id} className="flex items-start justify-between gap-3 border-b border-neutral-200 px-4 py-3 last:border-0 dark:border-neutral-800">
                  <div className="min-w-0">
                    <p className="font-mono text-xs">{t.developer_sub}</p>
                    <p className="text-xs opacity-60">{t.scopes.join(", ")} · expires {t.expires_at?.slice(0, 10)}</p>
                  </div>
                  <RevokeButton tokenId={t.id} />
                </div>
              ))}
            </div>
          </Card>
          <Card>
            <div className="p-4 text-sm leading-6">
              <p className="font-medium">Install instructions</p>
              <pre className="mt-2 rounded bg-neutral-950 p-3 text-xs text-neutral-100">
{`# 1. install the forge CLI (pick one)
brew install askenaz-dev/tap/forge         # macOS / Linux
npm install -g @askenaz-dev/forge-cli      # any OS with Node 18+

# 2. log in to your tenant gateway
forge login --gateway https://<tenant>.forge.dev

# 3. browse and install a skill
forge skills list
forge skills install generate-test-cases`}
              </pre>
            </div>
          </Card>
        </aside>
      </div>
    </>
  );
}
