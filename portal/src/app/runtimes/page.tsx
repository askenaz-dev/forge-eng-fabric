import Link from "next/link";
import { authToken, correlationId, endpoint, proxyJson } from "@/lib/api";
import { PageHead } from "@/components/page/PageHead";
import { Badge, Button, Card } from "@/components/primitives";

type Runtime = {
  id: string;
  name: string;
  type: string;
  mode: "byo" | "provisioned";
  status: string;
  env: string;
};

async function fetchRuntimes(): Promise<Runtime[]> {
  const { token } = await authToken();
  try {
    const data = await proxyJson<{ runtimes: Runtime[] }>(
      `${endpoint("CONTROL_PLANE_URL")}/v1/runtimes?limit=50`,
      { token, correlation: correlationId() },
    );
    return data.runtimes ?? [];
  } catch {
    return [];
  }
}

function statusTone(status: string): "ok" | "warn" | "err" | "default" {
  if (status === "ready" || status === "healthy") return "ok";
  if (status === "preflight_required" || status === "degraded") return "warn";
  if (status === "down" || status === "error") return "err";
  return "default";
}

export default async function RuntimesPage() {
  const runtimes = await fetchRuntimes();
  return (
    <>
      <PageHead
        eyebrow="Observability · Runtimes"
        title="Deploy"
        titleEm="targets"
        sub="Register BYO targets, run preflight, or track Forge-provisioned runtimes."
        actions={<Button variant="primary">Register BYO runtime</Button>}
      />
      <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fill, minmax(280px, 1fr))", gap: 12 }}>
        {runtimes.length === 0 && (
          <Card>
            <div className="note" style={{ padding: 24, textAlign: "center" }}>
              No runtimes registered yet.
            </div>
          </Card>
        )}
        {runtimes.map((runtime) => (
          <Card key={runtime.id}>
            <div style={{ padding: 16 }}>
              <div style={{ display: "flex", justifyContent: "space-between", alignItems: "flex-start", gap: 8 }}>
                <div>
                  <h3 style={{ fontFamily: "var(--f-display)", fontStyle: "italic", fontSize: 20, margin: 0, letterSpacing: "-0.015em" }}>
                    {runtime.name}
                  </h3>
                  <p style={{ fontFamily: "var(--f-mono)", fontSize: 11, color: "var(--fg-3)", margin: "2px 0 0" }}>{runtime.id}</p>
                </div>
                <Badge tone={statusTone(runtime.status)}>{runtime.status}</Badge>
              </div>
              <div style={{ display: "grid", gridTemplateColumns: "repeat(3, 1fr)", gap: 8, marginTop: 14, fontSize: 13 }}>
                <div>
                  <div className="h-eyebrow" style={{ margin: 0 }}>Type</div>
                  <div>{runtime.type}</div>
                </div>
                <div>
                  <div className="h-eyebrow" style={{ margin: 0 }}>Mode</div>
                  <div>{runtime.mode}</div>
                </div>
                <div>
                  <div className="h-eyebrow" style={{ margin: 0 }}>Env</div>
                  <div>{runtime.env}</div>
                </div>
              </div>
              <div style={{ display: "flex", gap: 6, marginTop: 16 }}>
                <Link href={`/runtimes/${runtime.id}`}>
                  <Button variant="secondary" size="xs">Run preflight</Button>
                </Link>
                <Button variant="danger" size="xs">Delete</Button>
              </div>
            </div>
          </Card>
        ))}
      </div>
    </>
  );
}
