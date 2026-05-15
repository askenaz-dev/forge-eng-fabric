import { authOptions } from "@/auth";
import { getServerSession } from "next-auth";
import { redirect } from "next/navigation";
import { PageHead } from "@/components/page/PageHead";
import { Card, CardHeader, Badge, Button } from "@/components/primitives";
import { Code } from "@/components/primitives";

type Approval = {
  id: string;
  principal: string;
  action: string;
  workspace_id: string;
  openspec_id?: string;
  rationale: string;
  required_approvers: string[];
  approval_mode?: "any" | "dual";
  approvals_given?: string[];
  criticality: string;
  correlation_id: string;
  status: string;
  requested_at: string;
  expires_at: string;
  decided_by?: string;
  decision_comment?: string;
  triggered_by_symptom_id?: string;
  triggered_by_session_id?: string;
  workflow_context?: {
    workflow_id?: string;
    workflow_version?: string;
    execution_id?: string;
    step_id?: string;
    previous_steps?: { id: string; outputs?: Record<string, unknown> }[];
    next_steps?: { id: string; type?: string; inputs?: Record<string, unknown> }[];
    proposed_inputs?: Record<string, unknown>;
  };
};

type SearchParams = { approver?: string; status?: string; decided?: string; error?: string };

const approvalsUrl = () => process.env.APPROVALS_URL ?? "http://localhost:8105";

async function getToken() {
  const session = await getServerSession(authOptions);
  if (!session) redirect("/api/auth/signin");
  return {
    token: (session as { accessToken?: string }).accessToken,
    user: session.user?.email ?? session.user?.name ?? "release-manager",
  };
}

async function decideApproval(formData: FormData) {
  "use server";
  const { token } = await getToken();
  const approvalId = required(formData, "approval_id");
  const actor = required(formData, "actor");
  const decision = required(formData, "decision");
  const comment = String(formData.get("comment") ?? "").trim();
  const response = await fetch(`${approvalsUrl()}/v1/approvals/${approvalId}/decisions`, {
    method: "POST",
    headers: { "content-type": "application/json", ...(token ? { authorization: `Bearer ${token}` } : {}) },
    body: JSON.stringify({ actor, decision, comment }),
  });
  if (!response.ok) redirect(`/approvals?error=${encodeURIComponent(await response.text())}`);
  redirect(`/approvals?approver=${encodeURIComponent(actor)}&decided=1`);
}

async function fetchApprovals(approver: string, status: string, token?: string) {
  const params = new URLSearchParams({ approver, status });
  const response = await fetch(`${approvalsUrl()}/v1/approvals?${params}`, {
    headers: token ? { authorization: `Bearer ${token}` } : {},
    cache: "no-store",
  });
  if (!response.ok) throw new Error(`approvals ${response.status}: ${await response.text()}`);
  return ((await response.json()) as { approvals: Approval[] }).approvals;
}

export default async function ApprovalsPage({ searchParams }: { searchParams: SearchParams }) {
  const { token, user } = await getToken();
  const approver = searchParams.approver?.trim() || user;
  const status = searchParams.status?.trim() || "pending";
  let approvals: Approval[] = [];
  let error = searchParams.error ?? null;
  try {
    approvals = await fetchApprovals(approver, status, token);
  } catch (e) {
    error = e instanceof Error ? e.message : "failed to load approvals";
  }

  return (
    <>
      <PageHead
        eyebrow="Governance · Approvals"
        title="Cola de"
        titleEm="aprobación"
        sub="Pausas de política con contexto de intención, OpenSpec, acción y telemetría."
        actions={
          <form method="get" style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
            <input
              name="approver"
              defaultValue={approver}
              className="top-search"
              style={{ height: 32, width: 220 }}
            />
            <select
              name="status"
              defaultValue={status}
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
              <option value="pending">pending</option>
              <option value="approved">approved</option>
              <option value="rejected">rejected</option>
              <option value="expired">expired</option>
            </select>
            <Button variant="primary" type="submit">
              Load
            </Button>
          </form>
        }
      />

      {searchParams.decided && (
        <Card style={{ marginBottom: 16 }}>
          <div style={{ padding: 14, color: "var(--thread)" }}>Approval decision recorded.</div>
        </Card>
      )}

      {error && (
        <Card style={{ marginBottom: 16 }}>
          <div style={{ padding: 14, color: "var(--rust)" }}>{error}</div>
        </Card>
      )}

      <div className="stack">
        {approvals.map((approval) => (
          <Card key={approval.id}>
            <CardHeader
              title={approval.action}
              sub={`${approval.workspace_id} · ${approval.correlation_id}`}
              right={<Badge tone={approval.status === "pending" ? "warn" : "default"}>{approval.status}</Badge>}
            />
            <div style={{ padding: "14px 18px", display: "grid", gap: 12 }}>
              <p style={{ color: "var(--fg-2)", margin: 0 }}>{approval.rationale}</p>
              <div style={{ display: "grid", gridTemplateColumns: "repeat(3, 1fr)", gap: 8, fontFamily: "var(--f-mono)", fontSize: 12 }}>
                <span>
                  <span style={{ color: "var(--fg-3)" }}>principal</span> {approval.principal}
                </span>
                <span>
                  <span style={{ color: "var(--fg-3)" }}>criticality</span> {approval.criticality}
                </span>
                <span>
                  <span style={{ color: "var(--fg-3)" }}>expires</span> {new Date(approval.expires_at).toLocaleString()}
                </span>
              </div>
              {approval.workflow_context && <WorkflowContext context={approval.workflow_context} />}
              {(approval.triggered_by_symptom_id || approval.approval_mode === "dual") && (
                <AutonomousContext approval={approval} />
              )}
              {approval.status === "pending" && (
                <form action={decideApproval} style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
                  <input type="hidden" name="approval_id" value={approval.id} />
                  <input type="hidden" name="actor" value={approver} />
                  <input
                    name="comment"
                    placeholder="Decision comment"
                    className="top-search"
                    style={{ minWidth: 0, flex: 1, height: 32 }}
                  />
                  <Button variant="primary" name="decision" value="approved" type="submit">
                    Approve
                  </Button>
                  <Button variant="danger" name="decision" value="rejected" type="submit">
                    Reject
                  </Button>
                </form>
              )}
            </div>
          </Card>
        ))}
        {approvals.length === 0 && !error && (
          <Card>
            <div className="note" style={{ padding: 24, textAlign: "center" }}>
              No approvals match this inbox filter.
            </div>
          </Card>
        )}
      </div>
    </>
  );
}

function required(formData: FormData, key: string) {
  const value = String(formData.get(key) ?? "").trim();
  if (!value) throw new Error(`${key} is required`);
  return value;
}

function AutonomousContext({ approval }: { approval: Approval }) {
  const isDual = approval.approval_mode === "dual";
  const given = approval.approvals_given ?? [];
  const required = approval.required_approvers ?? [];
  const missing = required.filter((r) => !given.includes(r));

  return (
    <div
      style={{
        background: "color-mix(in oklch, var(--warn), transparent 92%)",
        border: "1px solid color-mix(in oklch, var(--warn), transparent 70%)",
        borderRadius: "var(--r-3)",
        padding: "12px 14px",
        fontSize: 13,
        display: "grid",
        gap: 6,
      }}
    >
      <p style={{ margin: 0, fontWeight: 500 }}>
        Autonomous action context
        {isDual && (
          <span
            style={{
              marginLeft: 8,
              fontSize: 11,
              fontFamily: "var(--f-mono)",
              background: "var(--warn)",
              color: "#fff",
              borderRadius: 4,
              padding: "1px 6px",
            }}
          >
            dual-approval
          </span>
        )}
      </p>
      {approval.triggered_by_symptom_id && (
        <p style={{ margin: 0, fontFamily: "var(--f-mono)", fontSize: 11.5, color: "var(--fg-3)" }}>
          symptom:{" "}
          <a
            href={`/alfred/autonomous-activity?symptom=${approval.triggered_by_symptom_id}`}
            style={{ color: "var(--thread)" }}
          >
            {approval.triggered_by_symptom_id}
          </a>
        </p>
      )}
      {approval.triggered_by_session_id && (
        <p style={{ margin: 0, fontFamily: "var(--f-mono)", fontSize: 11.5, color: "var(--fg-3)" }}>
          session:{" "}
          <a
            href={`/alfred/sessions/${approval.triggered_by_session_id}`}
            style={{ color: "var(--thread)" }}
          >
            {approval.triggered_by_session_id}
          </a>
        </p>
      )}
      {isDual && required.length > 0 && (
        <div style={{ marginTop: 4 }}>
          <div className="h-eyebrow" style={{ margin: 0 }}>Approval progress</div>
          <ul style={{ fontFamily: "var(--f-mono)", fontSize: 11.5, margin: "4px 0", paddingLeft: 18 }}>
            {required.map((r) => (
              <li key={r} style={{ color: given.includes(r) ? "var(--ok)" : "var(--fg-3)" }}>
                {given.includes(r) ? "✓" : "○"} {r}
              </li>
            ))}
          </ul>
          {missing.length > 0 && (
            <p style={{ margin: "4px 0 0", color: "var(--fg-3)", fontSize: 11 }}>
              Awaiting: {missing.join(", ")}
            </p>
          )}
        </div>
      )}
    </div>
  );
}

function WorkflowContext({ context }: { context: NonNullable<Approval["workflow_context"]> }) {
  return (
    <div
      style={{
        background: "color-mix(in oklch, var(--info), transparent 90%)",
        border: "1px solid color-mix(in oklch, var(--info), transparent 70%)",
        borderRadius: "var(--r-3)",
        padding: "12px 14px",
        fontSize: 13,
      }}
    >
      <p style={{ margin: 0, fontWeight: 500 }}>Workflow context</p>
      <p style={{ margin: "4px 0", fontFamily: "var(--f-mono)", fontSize: 11, color: "var(--fg-3)" }}>
        {context.workflow_id ? `${context.workflow_id}@${context.workflow_version ?? ""}` : ""}
        {context.step_id ? ` · step ${context.step_id}` : ""}
      </p>
      {context.previous_steps && context.previous_steps.length > 0 && (
        <div style={{ marginTop: 8 }}>
          <div className="h-eyebrow" style={{ margin: 0 }}>Previous steps</div>
          <ul style={{ fontFamily: "var(--f-mono)", fontSize: 11.5, margin: "4px 0", paddingLeft: 18 }}>
            {context.previous_steps.map((s) => (
              <li key={s.id}>
                <code>{s.id}</code> {s.outputs ? `→ ${JSON.stringify(s.outputs)}` : ""}
              </li>
            ))}
          </ul>
        </div>
      )}
      {context.next_steps && context.next_steps.length > 0 && (
        <div style={{ marginTop: 8 }}>
          <div className="h-eyebrow" style={{ margin: 0 }}>Next step</div>
          <ul style={{ fontFamily: "var(--f-mono)", fontSize: 11.5, margin: "4px 0", paddingLeft: 18 }}>
            {context.next_steps.map((s) => (
              <li key={s.id}>
                <code>{s.id}</code> ({s.type ?? "?"})
                {s.inputs ? ` ← ${JSON.stringify(s.inputs)}` : ""}
              </li>
            ))}
          </ul>
        </div>
      )}
      {context.proposed_inputs && (
        <div style={{ marginTop: 8 }}>
          <div className="h-eyebrow" style={{ margin: 0 }}>Proposed inputs</div>
          <Code style={{ marginTop: 4 }}>{JSON.stringify(context.proposed_inputs, null, 2)}</Code>
        </div>
      )}
    </div>
  );
}
