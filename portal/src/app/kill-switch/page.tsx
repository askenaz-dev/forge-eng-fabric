import { authOptions } from "@/auth";
import { getServerSession } from "next-auth";
import { redirect } from "next/navigation";
import { PageHead } from "@/components/page/PageHead";
import { ScopeSelect } from "@/components/scope/ScopeSelect";

type KillSwitchState = { active: boolean };

const healingUrl = () => process.env.HEALING_ENGINE_URL ?? "http://localhost:8102";

async function getToken() {
  const session = await getServerSession(authOptions);
  if (!session) redirect("/api/auth/signin");
  return (session as { accessToken?: string }).accessToken;
}

async function fetchState(workspaceId: string, token?: string): Promise<KillSwitchState> {
  const params = workspaceId ? `?workspace_id=${encodeURIComponent(workspaceId)}` : "";
  const response = await fetch(`${healingUrl()}/v1/kill-switch${params}`, {
    headers: token ? { authorization: `Bearer ${token}` } : {},
    cache: "no-store",
  });
  if (!response.ok) return { active: false };
  return (await response.json()) as KillSwitchState;
}

async function toggleKillSwitch(formData: FormData) {
  "use server";
  const token = await getToken();
  const workspaceId = String(formData.get("workspace_id") ?? "");
  const active = formData.get("active") === "true";
  const reason = String(formData.get("reason") ?? "");
  const actor = String(formData.get("actor") ?? "portal-user");
  await fetch(`${healingUrl()}/v1/kill-switch`, {
    method: "POST",
    headers: { "content-type": "application/json", ...(token ? { authorization: `Bearer ${token}` } : {}) },
    body: JSON.stringify({ workspace_id: workspaceId, active, reason, actor }),
  });
  redirect(`/kill-switch${workspaceId ? `?workspace_id=${workspaceId}` : ""}`);
}

export default async function KillSwitchPage({ searchParams }: { searchParams: { workspace_id?: string } }) {
  const token = await getToken();
  const workspaceId = searchParams.workspace_id ?? "";
  const state = await fetchState(workspaceId, token);

  return (
    <>
      <PageHead
        eyebrow="Governance · Kill switch"
        title="Healing"
        titleEm="kill switch"
        sub="Activate to immediately degrade ALL healing actions to L1 (Notify-only). Requires platform-admin or security-approver."
      />

      <div className={`rounded border p-5 ${state.active ? "border-red-400 bg-red-50 dark:border-red-700 dark:bg-red-950/30" : "border-emerald-400 bg-emerald-50 dark:border-emerald-700 dark:bg-emerald-950/30"}`} data-testid="kill-switch-state">
        <p className="font-medium">Status: <span data-testid="kill-switch-status">{state.active ? "ACTIVE" : "INACTIVE"}</span></p>
        <p className="text-sm opacity-70">{state.active ? "Healing engine is suppressing all autonomous actions." : "Healing engine is operating normally."}</p>
      </div>

      <form action={toggleKillSwitch} className="space-y-3 rounded border border-neutral-200 bg-white p-5 dark:border-neutral-800 dark:bg-neutral-900">
        <h3 className="font-medium">Toggle</h3>
        <input type="hidden" name="active" value={state.active ? "false" : "true"} />
        <label className="grid gap-1 text-sm">
          <span>Workspace ID (leave blank for global)</span>
          <ScopeSelect kind="workspace" name="workspace_id" defaultValue={workspaceId} className="rounded border border-neutral-300 bg-transparent px-3 py-2 dark:border-neutral-700" />
        </label>
        <label className="grid gap-1 text-sm">
          <span>Reason (mandatory)</span>
          <textarea name="reason" required rows={3} className="rounded border border-neutral-300 bg-transparent px-3 py-2 dark:border-neutral-700" />
        </label>
        <button data-testid={state.active ? "kill-switch-deactivate" : "kill-switch-activate"} className={`rounded px-4 py-2 text-sm font-medium text-white ${state.active ? "bg-emerald-600" : "bg-red-600"}`}>
          {state.active ? "Deactivate kill switch" : "Activate kill switch"}
        </button>
      </form>

      <div className="rounded border border-neutral-200 bg-white p-5 dark:border-neutral-800 dark:bg-neutral-900">
        <h3 className="font-medium">Audit log</h3>
        <p className="mt-1 text-sm opacity-70">All toggles emit <code>healing.kill_switch.toggled.v1</code> CloudEvents — see Audit Service or Grafana dashboard &ldquo;Phase 6 — Autonomous Ops&rdquo;.</p>
      </div>
    </>
  );
}
