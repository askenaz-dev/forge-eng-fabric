import { authToken, correlationId, endpoint, proxyJson } from "@/lib/api";
import { PageHead } from "@/components/page/PageHead";
import { Badge, Button, Card } from "@/components/primitives";

type DriftFinding = {
  id: string;
  resource: string;
  field: string;
  severity: "low" | "medium" | "high";
  status: string;
};

async function fetchFindings(): Promise<DriftFinding[]> {
  const { token } = await authToken();
  try {
    const data = await proxyJson<{ findings: DriftFinding[] }>(
      `${endpoint("DEPLOY_URL")}/v1/drift/findings?limit=50`,
      { token, correlation: correlationId() },
    );
    return data.findings ?? [];
  } catch {
    return [];
  }
}

const SEVERITY_TONE = {
  low: "default" as const,
  medium: "warn" as const,
  high: "err" as const,
};

export default async function DriftPage() {
  const findings = await fetchFindings();
  return (
    <>
      <PageHead
        eyebrow="Observability · IaC Drift"
        title="Terraform drift"
        titleEm="findings"
        sub="Review hourly findings, route high-severity changes, and ask Alfred to propose remediation PRs."
      />
      <div className="stack">
        {findings.length === 0 && (
          <Card>
            <div className="note" style={{ padding: 24, textAlign: "center" }}>
              No drift findings reported.
            </div>
          </Card>
        )}
        {findings.map((finding) => (
          <Card key={finding.id}>
            <div style={{ padding: 16, display: "flex", flexWrap: "wrap", gap: 12, alignItems: "center", justifyContent: "space-between" }}>
              <div>
                <h3 style={{ fontFamily: "var(--f-display)", fontStyle: "italic", fontSize: 20, margin: 0, letterSpacing: "-0.015em" }}>
                  {finding.resource}
                </h3>
                <p style={{ fontFamily: "var(--f-mono)", fontSize: 11, color: "var(--fg-3)", margin: "2px 0 0" }}>
                  {finding.id} · field {finding.field}
                </p>
              </div>
              <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
                <Badge tone={SEVERITY_TONE[finding.severity] ?? "default"}>{finding.severity}</Badge>
                <span style={{ fontSize: 13, color: "var(--fg-2)" }}>{finding.status}</span>
                <Button variant="primary" size="xs">Propose PR</Button>
              </div>
            </div>
          </Card>
        ))}
      </div>
    </>
  );
}
