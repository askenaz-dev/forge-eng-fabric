import { authToken, correlationId, endpoint, proxyJson } from "@/lib/api";
import { PageHead } from "@/components/page/PageHead";
import { Card, Badge, Button } from "@/components/primitives";

type Deployment = {
  id: string;
  asset: string;
  env: string;
  status: string;
  revision: string;
  runtime: string;
  image: string;
  stage: string;
};

const STAGES = ["preflight", "policy", "image-verify", "render", "apply", "verify", "notify"];

async function fetchDeployments(): Promise<Deployment[]> {
  const { token } = await authToken();
  try {
    const data = await proxyJson<{ deployments: Deployment[] }>(
      `${endpoint("DEPLOY_URL")}/v1/deployments?limit=50`,
      { token, correlation: correlationId() },
    );
    return data.deployments ?? [];
  } catch {
    return [];
  }
}

export default async function DeploymentsPage() {
  const deployments = await fetchDeployments();
  return (
    <>
      <PageHead
        eyebrow="Observability · Deployments"
        title="Release history &"
        titleEm="live status"
        sub="Inspect stages, verify image status, and rollback to the previous revision with an audited reason."
      />
      <Card>
        {deployments.length === 0 && (
          <div className="note" style={{ padding: 24, textAlign: "center" }}>
            No deployments to display.
          </div>
        )}
        {deployments.map((deployment) => (
          <div key={deployment.id} style={{ borderBottom: "1px solid var(--border)", padding: 16 }}>
            <div style={{ display: "flex", flexWrap: "wrap", gap: 12, alignItems: "center", justifyContent: "space-between" }}>
              <div>
                <h3 style={{ fontFamily: "var(--f-display)", fontSize: 22, fontStyle: "italic", margin: 0, letterSpacing: "-0.015em" }}>
                  {deployment.asset}{" "}
                  <span style={{ fontFamily: "var(--f-mono)", fontSize: 12, color: "var(--fg-3)", fontStyle: "normal" }}>
                    {deployment.env}
                  </span>
                </h3>
                <p style={{ fontFamily: "var(--f-mono)", fontSize: 11, color: "var(--fg-3)", margin: "4px 0 0" }}>
                  {deployment.id} · {deployment.revision} · {deployment.runtime} · {deployment.image}
                </p>
              </div>
              <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
                <Badge tone={statusTone(deployment.status)}>{deployment.status}</Badge>
                <Button variant="secondary" size="xs">Rollback to previous</Button>
              </div>
            </div>
            <div style={{ marginTop: 16, display: "grid", gridTemplateColumns: "repeat(7, 1fr)", gap: 8 }}>
              {STAGES.map((stage) => (
                <div
                  key={stage}
                  style={{
                    padding: "8px",
                    borderRadius: "var(--r-2)",
                    fontFamily: "var(--f-mono)",
                    fontSize: 11,
                    background: stage === deployment.stage ? "var(--primary)" : "var(--bg-hover)",
                    color: stage === deployment.stage ? "var(--on-primary)" : "var(--fg-2)",
                  }}
                >
                  {stage}
                </div>
              ))}
            </div>
            <details style={{ marginTop: 12, fontSize: 13 }}>
              <summary style={{ cursor: "pointer", color: "var(--fg-2)" }}>Rollback confirmation</summary>
              <div style={{ marginTop: 8, background: "var(--bg-sunk)", borderRadius: "var(--r-2)", padding: 12 }}>
                <label className="h-eyebrow" style={{ display: "block" }}>Reason</label>
                <textarea
                  style={{
                    marginTop: 4,
                    minHeight: 80,
                    width: "100%",
                    border: "1px solid var(--border)",
                    borderRadius: "var(--r-2)",
                    background: "var(--bg-card)",
                    padding: 8,
                    fontFamily: "var(--f-sans)",
                    fontSize: 13,
                    color: "var(--fg)",
                  }}
                  placeholder="Explain customer impact and recovery intent"
                />
                <Button variant="danger" size="xs" style={{ marginTop: 8 }}>Confirm rollback</Button>
              </div>
            </details>
          </div>
        ))}
      </Card>
    </>
  );
}

function statusTone(status: string): "ok" | "warn" | "err" | "default" {
  if (status === "verified" || status === "succeeded") return "ok";
  if (status === "running" || status === "in_progress") return "warn";
  if (status === "rolled_back" || status === "failed") return "err";
  return "default";
}
