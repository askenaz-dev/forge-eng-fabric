import { authToken, endpoint } from "@/lib/api";
import { PageHead } from "@/components/page/PageHead";
import { UpstreamError } from "@/components/primitives";
import { TenantsClient } from "./TenantsClient";

type Tenant = { id: string; name: string; created_at?: string };

type FetchOutcome = { tenants: Tenant[]; status?: number; error: string | null };

async function fetchTenants(token?: string): Promise<FetchOutcome> {
  try {
    const resp = await fetch(`${endpoint("CONTROL_PLANE_URL")}/v1/tenants`, {
      headers: token ? { authorization: `Bearer ${token}` } : {},
      cache: "no-store",
    });
    if (!resp.ok) {
      const detail = await resp.text().catch(() => "");
      return { tenants: [], status: resp.status, error: `control-plane ${resp.status}: ${detail || resp.statusText}` };
    }
    const body = (await resp.json()) as Tenant[] | { tenants: Tenant[] };
    const list = Array.isArray(body) ? body : (body.tenants ?? []);
    return { tenants: list, error: null };
  } catch (e) {
    return { tenants: [], error: e instanceof Error ? e.message : "control-plane unreachable" };
  }
}

export default async function TenantsAdminPage() {
  const { token } = await authToken();
  const outcome = await fetchTenants(token);

  return (
    <>
      <PageHead
        eyebrow="Account · Admin"
        title="Tenants"
        titleEm="admin"
        sub="Create new tenants and review existing ones. Restricted to platform-admin."
      />
      {outcome.error && (
        <UpstreamError
          service="control-plane"
          status={outcome.status}
          message={outcome.error}
          hint={
            outcome.status === 401 ? (
              <ul style={{ margin: 0, paddingLeft: 18, lineHeight: 1.7 }}>
                <li>Confirm Keycloak is running at <code>http://localhost:8080</code> (it provides the JWT the control-plane verifies).</li>
                <li>Sign out and back in via <a href="/api/auth/signin" style={{ textDecoration: "underline" }}>/api/auth/signin</a> if your session is stale.</li>
                <li>While Keycloak is being set up, you can seed a tenant from the CLI: <code>bash deploy/compose/scripts/seed-portal.sh</code>.</li>
              </ul>
            ) : outcome.status === 403 ? (
              <p style={{ margin: 0 }}>Your token is valid but does not carry the <code>platform-admin</code> role. Ask a platform admin to grant it in Keycloak.</p>
            ) : (
              <p style={{ margin: 0 }}>Check <code>services/control-plane</code> logs and that <code>CONTROL_PLANE_URL</code> in the portal env points to a reachable instance.</p>
            )
          }
        />
      )}
      <TenantsClient initialTenants={outcome.tenants} />
    </>
  );
}
