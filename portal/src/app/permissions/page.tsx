import { authOptions } from "@/auth";
import { getServerSession } from "next-auth";
import { redirect } from "next/navigation";

type Grant = {
  id: string;
  subject: string;
  scope_kind: string;
  scope_id: string;
  action_class: string;
  max_criticality: string;
  expires_at: string;
  justification: string;
  requester: string;
  approver: string;
  status: string;
  audit_history: { actor: string; action: string; timestamp: string; rationale?: string }[];
};

type SearchParams = { scope_id?: string; saved?: string; revoked?: string; error?: string };

const permissionsUrl = () => process.env.PERMISSIONS_URL ?? "http://localhost:8092";

async function getToken() {
  const session = await getServerSession(authOptions);
  if (!session) redirect("/api/auth/signin");
  return {
    token: (session as { accessToken?: string }).accessToken,
    user: session.user?.email ?? session.user?.name ?? "workspace-owner",
  };
}

async function createGrant(formData: FormData) {
  "use server";
  const { token, user } = await getToken();
  const payload = {
    subject: "alfred",
    scope_kind: required(formData, "scope_kind"),
    scope_id: required(formData, "scope_id"),
    action_class: required(formData, "action_class"),
    max_criticality: required(formData, "max_criticality"),
    expiration_days: Number(required(formData, "expiration_days")),
    justification: required(formData, "justification"),
    requester: user,
    approver: user,
  };
  const response = await fetch(`${permissionsUrl()}/v1/permissions/grants`, {
    method: "POST",
    headers: { "content-type": "application/json", ...(token ? { authorization: `Bearer ${token}` } : {}) },
    body: JSON.stringify(payload),
  });
  if (!response.ok) redirect(`/permissions?error=${encodeURIComponent(await response.text())}`);
  redirect(`/permissions?scope_id=${encodeURIComponent(payload.scope_id)}&saved=1`);
}

async function revokeGrant(formData: FormData) {
  "use server";
  const { token, user } = await getToken();
  const grantId = required(formData, "grant_id");
  const scopeId = required(formData, "scope_id");
  const response = await fetch(`${permissionsUrl()}/v1/permissions/grants/${grantId}/revoke`, {
    method: "POST",
    headers: { "content-type": "application/json", ...(token ? { authorization: `Bearer ${token}` } : {}) },
    body: JSON.stringify({ actor: user, rationale: "revoked from Portal" }),
  });
  if (!response.ok) redirect(`/permissions?error=${encodeURIComponent(await response.text())}`);
  redirect(`/permissions?scope_id=${encodeURIComponent(scopeId)}&revoked=1`);
}

async function fetchGrants(scopeId: string, token?: string) {
  const params = new URLSearchParams();
  if (scopeId) params.set("scope_id", scopeId);
  const response = await fetch(`${permissionsUrl()}/v1/permissions/grants?${params}`, {
    headers: token ? { authorization: `Bearer ${token}` } : {},
    cache: "no-store",
  });
  if (!response.ok) throw new Error(`permissions ${response.status}: ${await response.text()}`);
  return ((await response.json()) as { grants: Grant[] }).grants;
}

export default async function PermissionsPage({ searchParams }: { searchParams: SearchParams }) {
  const { token } = await getToken();
  const scopeId = searchParams.scope_id?.trim() ?? "";
  let grants: Grant[] = [];
  let error = searchParams.error ?? null;
  try {
    grants = await fetchGrants(scopeId, token);
  } catch (e) {
    error = e instanceof Error ? e.message : "failed to load grants";
  }

  return (
    <section className="space-y-5">
      <div>
        <h2 className="text-2xl font-semibold">Delegated Permissions</h2>
        <p className="mt-1 text-sm opacity-70">Grant and revoke scoped Alfred permissions with audit history and expiration.</p>
      </div>
      {searchParams.saved && <p className="rounded border border-green-300 bg-green-50 p-3 text-sm text-green-800 dark:border-green-800 dark:bg-green-950 dark:text-green-200">Grant created.</p>}
      {searchParams.revoked && <p className="rounded border border-green-300 bg-green-50 p-3 text-sm text-green-800 dark:border-green-800 dark:bg-green-950 dark:text-green-200">Grant revoked.</p>}
      {error && <p className="rounded border border-red-300 bg-red-50 p-3 text-sm text-red-800 dark:border-red-800 dark:bg-red-950 dark:text-red-200">{error}</p>}

      <div className="grid gap-5 lg:grid-cols-[360px_1fr]">
        <form action={createGrant} className="space-y-4 rounded border border-neutral-200 bg-white p-5 dark:border-neutral-800 dark:bg-neutral-900">
          <h3 className="font-medium">Grant Alfred</h3>
          <Select name="scope_kind" label="Scope kind" values={["workspace", "repo", "environment", "cloud_project"]} />
          <Field name="scope_id" label="Scope ID" defaultValue={scopeId} />
          <Field name="action_class" label="Action class" defaultValue="openspec:write" />
          <Select name="max_criticality" label="Max criticality" values={["low", "medium", "high", "critical"]} />
          <Field name="expiration_days" label="Expiration days" defaultValue="30" />
          <TextArea name="justification" label="Justification" />
          <button className="rounded bg-neutral-900 px-4 py-2 text-sm font-medium text-white dark:bg-neutral-100 dark:text-neutral-900">Create grant</button>
        </form>

        <div className="space-y-4">
          <form className="flex gap-2" method="get">
            <input name="scope_id" defaultValue={scopeId} placeholder="Filter by scope ID" className="min-w-0 flex-1 rounded border border-neutral-300 bg-transparent px-3 py-2 text-sm dark:border-neutral-700" />
            <button className="rounded border border-neutral-300 px-4 py-2 text-sm dark:border-neutral-700">Filter</button>
          </form>
          {grants.map((grant) => (
            <article key={grant.id} className="rounded border border-neutral-200 bg-white p-5 dark:border-neutral-800 dark:bg-neutral-900">
              <div className="flex flex-col gap-2 md:flex-row md:justify-between">
                <div>
                  <h3 className="font-medium">{grant.action_class} on {grant.scope_kind}:{grant.scope_id}</h3>
                  <p className="mt-1 text-sm opacity-70">{grant.justification}</p>
                </div>
                <span className="rounded bg-neutral-100 px-2 py-1 text-xs uppercase tracking-wide dark:bg-neutral-800">{grant.status}</span>
              </div>
              <p className="mt-3 text-xs opacity-60">Expires {new Date(grant.expires_at).toLocaleString()} · max {grant.max_criticality}</p>
              <div className="mt-3 text-xs opacity-70">Audit: {grant.audit_history.map((entry) => `${entry.action} by ${entry.actor}`).join(", ")}</div>
              {grant.status === "active" && (
                <form action={revokeGrant} className="mt-4">
                  <input type="hidden" name="grant_id" value={grant.id} />
                  <input type="hidden" name="scope_id" value={grant.scope_id} />
                  <button className="rounded bg-red-700 px-4 py-2 text-sm text-white">Revoke</button>
                </form>
              )}
            </article>
          ))}
          {grants.length === 0 && !error && <p className="rounded border border-dashed border-neutral-300 p-6 text-sm opacity-70 dark:border-neutral-800">No grants found.</p>}
        </div>
      </div>
    </section>
  );
}

function Field({ name, label, defaultValue }: { name: string; label: string; defaultValue?: string }) {
  return (
    <label className="grid gap-1 text-sm">
      <span className="font-medium">{label}</span>
      <input name={name} required defaultValue={defaultValue} className="rounded border border-neutral-300 bg-transparent px-3 py-2 dark:border-neutral-700" />
    </label>
  );
}

function Select({ name, label, values }: { name: string; label: string; values: string[] }) {
  return (
    <label className="grid gap-1 text-sm">
      <span className="font-medium">{label}</span>
      <select name={name} className="rounded border border-neutral-300 bg-transparent px-3 py-2 dark:border-neutral-700">
        {values.map((value) => <option key={value} value={value}>{value}</option>)}
      </select>
    </label>
  );
}

function TextArea({ name, label }: { name: string; label: string }) {
  return (
    <label className="grid gap-1 text-sm">
      <span className="font-medium">{label}</span>
      <textarea name={name} required rows={3} className="rounded border border-neutral-300 bg-transparent px-3 py-2 dark:border-neutral-700" />
    </label>
  );
}

function required(formData: FormData, key: string) {
  const value = String(formData.get(key) ?? "").trim();
  if (!value) throw new Error(`${key} is required`);
  return value;
}
