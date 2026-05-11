import { authOptions } from "@/auth";
import { getServerSession } from "next-auth";
import { redirect } from "next/navigation";

type Approval = {
  id: string;
  principal: string;
  action: string;
  workspace_id: string;
  openspec_id?: string;
  rationale: string;
  required_approvers: string[];
  criticality: string;
  correlation_id: string;
  status: string;
  requested_at: string;
  expires_at: string;
  decided_by?: string;
  decision_comment?: string;
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
    <section className="space-y-5">
      <div className="flex flex-col gap-3 md:flex-row md:items-end md:justify-between">
        <div>
          <h2 className="text-2xl font-semibold">Approvals Inbox</h2>
          <p className="mt-1 text-sm opacity-70">Review policy pauses with intent, OpenSpec, action and telemetry context.</p>
        </div>
        <form className="flex flex-wrap gap-2 text-sm" method="get">
          <input name="approver" defaultValue={approver} className="rounded border border-neutral-300 bg-transparent px-3 py-2 dark:border-neutral-700" />
          <select name="status" defaultValue={status} className="rounded border border-neutral-300 bg-transparent px-3 py-2 dark:border-neutral-700">
            <option value="pending">pending</option>
            <option value="approved">approved</option>
            <option value="rejected">rejected</option>
            <option value="expired">expired</option>
          </select>
          <button className="rounded bg-neutral-900 px-4 py-2 text-white dark:bg-neutral-100 dark:text-neutral-900">Load</button>
        </form>
      </div>

      {searchParams.decided && <p className="rounded border border-green-300 bg-green-50 p-3 text-sm text-green-800 dark:border-green-800 dark:bg-green-950 dark:text-green-200">Approval decision recorded.</p>}
      {error && <p className="rounded border border-red-300 bg-red-50 p-3 text-sm text-red-800 dark:border-red-800 dark:bg-red-950 dark:text-red-200">{error}</p>}

      <div className="grid gap-4">
        {approvals.map((approval) => (
          <article key={approval.id} className="rounded border border-neutral-200 bg-white p-5 dark:border-neutral-800 dark:bg-neutral-900">
            <div className="flex flex-col gap-2 md:flex-row md:items-start md:justify-between">
              <div>
                <h3 className="font-medium">{approval.action}</h3>
                <p className="mt-1 text-sm opacity-70">{approval.rationale}</p>
                <p className="mt-2 text-xs opacity-60">Workspace <code>{approval.workspace_id}</code> · correlation <code>{approval.correlation_id}</code></p>
              </div>
              <span className="rounded bg-neutral-100 px-2 py-1 text-xs uppercase tracking-wide dark:bg-neutral-800">{approval.status}</span>
            </div>
            <div className="mt-4 grid gap-3 text-sm md:grid-cols-3">
              <p><span className="font-medium">Principal:</span> {approval.principal}</p>
              <p><span className="font-medium">Criticality:</span> {approval.criticality}</p>
              <p><span className="font-medium">Expires:</span> {new Date(approval.expires_at).toLocaleString()}</p>
            </div>
            {approval.workflow_context && (
              <WorkflowContext context={approval.workflow_context} />
            )}
            {approval.status === "pending" && (
              <form action={decideApproval} className="mt-4 flex flex-col gap-2 md:flex-row">
                <input type="hidden" name="approval_id" value={approval.id} />
                <input type="hidden" name="actor" value={approver} />
                <input name="comment" placeholder="Decision comment" className="min-w-0 flex-1 rounded border border-neutral-300 bg-transparent px-3 py-2 text-sm dark:border-neutral-700" />
                <button name="decision" value="approved" className="rounded bg-green-700 px-4 py-2 text-sm text-white">Approve</button>
                <button name="decision" value="rejected" className="rounded bg-red-700 px-4 py-2 text-sm text-white">Reject</button>
              </form>
            )}
          </article>
        ))}
        {approvals.length === 0 && !error && <p className="rounded border border-dashed border-neutral-300 p-6 text-sm opacity-70 dark:border-neutral-800">No approvals match this inbox filter.</p>}
      </div>
    </section>
  );
}

function required(formData: FormData, key: string) {
  const value = String(formData.get(key) ?? "").trim();
  if (!value) throw new Error(`${key} is required`);
  return value;
}

function WorkflowContext({ context }: { context: NonNullable<Approval["workflow_context"]> }) {
  return (
    <section className="mt-4 rounded border border-blue-300 bg-blue-50 p-3 text-sm dark:border-blue-800 dark:bg-blue-950">
      <p className="font-medium">Workflow context</p>
      <p className="text-xs opacity-70">
        {context.workflow_id ? `${context.workflow_id}@${context.workflow_version ?? ""}` : ""}
        {context.step_id ? ` · step ${context.step_id}` : ""}
      </p>
      {context.previous_steps && context.previous_steps.length > 0 && (
        <div className="mt-2">
          <p className="text-xs uppercase opacity-60">Previous steps</p>
          <ul className="text-xs">
            {context.previous_steps.map((s) => (
              <li key={s.id}>
                <code>{s.id}</code> {s.outputs ? `→ ${JSON.stringify(s.outputs)}` : ""}
              </li>
            ))}
          </ul>
        </div>
      )}
      {context.next_steps && context.next_steps.length > 0 && (
        <div className="mt-2">
          <p className="text-xs uppercase opacity-60">Next step</p>
          <ul className="text-xs">
            {context.next_steps.map((s) => (
              <li key={s.id}>
                <code>{s.id}</code> ({s.type ?? "?"}){s.inputs ? ` ← ${JSON.stringify(s.inputs)}` : ""}
              </li>
            ))}
          </ul>
        </div>
      )}
      {context.proposed_inputs && (
        <div className="mt-2">
          <p className="text-xs uppercase opacity-60">Proposed inputs (you can modify before approving)</p>
          <pre className="mt-1 overflow-auto rounded bg-neutral-950 p-2 font-mono text-xs text-neutral-100">
            {JSON.stringify(context.proposed_inputs, null, 2)}
          </pre>
        </div>
      )}
    </section>
  );
}
