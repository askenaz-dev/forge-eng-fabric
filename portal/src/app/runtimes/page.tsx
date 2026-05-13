import Link from "next/link";
import { authToken, correlationId, endpoint } from "@/lib/api";
import { PageHead } from "@/components/page/PageHead";
import { Badge, Button, Card, UpstreamError } from "@/components/primitives";
import { RegisterRuntimeButton } from "./RegisterRuntimeButton";

type Runtime = {
  id: string;
  name: string;
  type: string;
  mode: "byo" | "provisioned";
  visibility?: "workspace" | "tenant";
  status: string;
  region?: string;
  env?: string;
  workspace_id?: string;
  tenant_id?: string;
  labels?: { managed_by?: string; shared?: boolean } & Record<string, unknown>;
};

type FetchOutcome = { runtimes: Runtime[]; status?: number; error: string | null };

async function fetchRuntimes(): Promise<FetchOutcome> {
  const { token } = await authToken();
  const correlation = correlationId();
  try {
    const resp = await fetch(`${endpoint("RUNTIME_REGISTRY_URL")}/v1/runtimes`, {
      headers: {
        ...(token ? { authorization: `Bearer ${token}` } : {}),
        "x-correlation-id": correlation,
      },
      cache: "no-store",
    });
    if (!resp.ok) {
      const detail = await resp.text().catch(() => "");
      return { runtimes: [], status: resp.status, error: `runtime-registry ${resp.status}: ${detail || resp.statusText}` };
    }
    const body = (await resp.json()) as { runtimes?: Runtime[] };
    return { runtimes: body.runtimes ?? [], error: null };
  } catch (e) {
    return { runtimes: [], error: e instanceof Error ? e.message : "runtime-registry unreachable" };
  }
}

function statusTone(status: string): "ok" | "warn" | "err" | "default" {
  if (status === "ready" || status === "healthy") return "ok";
  if (status === "preflight_required" || status === "degraded" || status === "registered") return "warn";
  if (status === "down" || status === "error" || status === "preflight_failed") return "err";
  return "default";
}

function isShared(r: Runtime): boolean {
  return r.visibility === "tenant" || r.labels?.shared === true || r.labels?.managed_by === "forge";
}

export default async function RuntimesPage() {
  const outcome = await fetchRuntimes();
  const shared = outcome.runtimes.filter(isShared);
  const owned = outcome.runtimes.filter((r) => !isShared(r));

  return (
    <>
      <PageHead
        eyebrow="Observability · Runtimes"
        title="Deploy"
        titleEm="targets"
        sub="Register BYO targets, run preflight, or use the shared runtime Forge provides out of the box."
        actions={<RegisterRuntimeButton />}
      />

      {outcome.error && (
        <UpstreamError
          service="runtime-registry"
          status={outcome.status}
          message={outcome.error}
          hint={
            <p style={{ margin: 0 }}>
              Confirm the service is running. With docker-compose:&nbsp;
              <code>docker compose up -d runtime-registry</code>. The default port is <code>8110</code>.
            </p>
          }
        />
      )}

      {!outcome.error && outcome.runtimes.length === 0 && (
        <Card>
          <div style={{ padding: 20, fontSize: 14, lineHeight: 1.6 }}>
            <p style={{ margin: 0 }}>No runtimes available yet.</p>
            <p style={{ margin: "6px 0 0", color: "var(--fg-2)" }}>
              A shared Forge runtime is normally seeded at startup
              (env <code>FORGE_DEFAULT_RUNTIME_TENANT</code>). If you have none,
              start with <strong>Register BYO runtime</strong>.
            </p>
          </div>
        </Card>
      )}

      {shared.length > 0 && (
        <section style={{ marginBottom: 18 }}>
          <h2 className="h-eyebrow" style={{ marginBottom: 8 }}>Shared (Forge-managed)</h2>
          <RuntimeGrid runtimes={shared} />
        </section>
      )}

      {owned.length > 0 && (
        <section>
          <h2 className="h-eyebrow" style={{ marginBottom: 8 }}>Workspace runtimes</h2>
          <RuntimeGrid runtimes={owned} />
        </section>
      )}
    </>
  );
}

function RuntimeGrid({ runtimes }: { runtimes: Runtime[] }) {
  return (
    <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fill, minmax(280px, 1fr))", gap: 12 }}>
      {runtimes.map((runtime) => (
        <Card key={runtime.id}>
          <div style={{ padding: 16 }}>
            <div style={{ display: "flex", justifyContent: "space-between", alignItems: "flex-start", gap: 8 }}>
              <div style={{ minWidth: 0 }}>
                <h3 style={{ fontFamily: "var(--f-display)", fontStyle: "italic", fontSize: 20, margin: 0, letterSpacing: "-0.015em" }}>
                  {runtime.name}
                </h3>
                <p style={{ fontFamily: "var(--f-mono)", fontSize: 11, color: "var(--fg-3)", margin: "2px 0 0", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
                  {runtime.id}
                </p>
              </div>
              <div style={{ display: "flex", gap: 4, flexWrap: "wrap", justifyContent: "flex-end" }}>
                {isShared(runtime) && <Badge tone="info">shared</Badge>}
                <Badge tone={statusTone(runtime.status)}>{runtime.status}</Badge>
              </div>
            </div>
            <div style={{ display: "grid", gridTemplateColumns: "repeat(3, 1fr)", gap: 8, marginTop: 14, fontSize: 13 }}>
              <Field label="Type" value={runtime.type} />
              <Field label="Mode" value={runtime.mode} />
              <Field label={runtime.region ? "Region" : "Env"} value={runtime.region ?? runtime.env ?? "—"} />
            </div>
            <div style={{ display: "flex", gap: 6, marginTop: 16 }}>
              <Link href={`/runtimes/${runtime.id}`}>
                <Button variant="secondary" size="xs">Run preflight</Button>
              </Link>
              {!isShared(runtime) && (
                <Button variant="danger" size="xs">Delete</Button>
              )}
            </div>
          </div>
        </Card>
      ))}
    </div>
  );
}

function Field({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <div className="h-eyebrow" style={{ margin: 0 }}>{label}</div>
      <div style={{ overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{value}</div>
    </div>
  );
}
