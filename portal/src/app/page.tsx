import { getServerSession } from "next-auth";
import { authOptions } from "@/auth";
import { redirect } from "next/navigation";
import { headers } from "next/headers";
import { randomUUID } from "crypto";

type Workspace = {
  id: string;
  tenant_id: string;
  business_unit_id: string;
  name: string;
  description?: string;
  owners: string[];
  created_at: string;
};

async function fetchWorkspaces(
  token: string,
  correlationId: string,
): Promise<{ workspaces: Workspace[]; responseCorrelationId: string }> {
  const cp = process.env.CONTROL_PLANE_URL ?? "http://localhost:8081";
  const r = await fetch(`${cp}/v1/workspaces`, {
    headers: { authorization: `Bearer ${token}`, "x-correlation-id": correlationId },
    cache: "no-store",
  });
  if (!r.ok) throw new Error(`control-plane ${r.status}`);
  return {
    workspaces: await r.json(),
    responseCorrelationId: r.headers.get("x-correlation-id") ?? correlationId,
  };
}

export default async function HomePage() {
  const session = await getServerSession(authOptions);
  if (!session) redirect("/api/auth/signin");

  const token = (session as any).accessToken as string | undefined;
  const correlationId = headers().get("x-correlation-id") ?? randomUUID();
  let responseCorrelationId = correlationId;
  let workspaces: Workspace[] = [];
  let error: string | null = null;
  if (token) {
    try {
      const result = await fetchWorkspaces(token, correlationId);
      workspaces = result.workspaces;
      responseCorrelationId = result.responseCorrelationId;
    } catch (e: any) {
      error = e.message;
    }
  } else {
    error = "no access token in session";
  }

  return (
    <section className="space-y-4">
      <div className="flex items-baseline justify-between">
        <h2 className="text-2xl font-semibold">Workspaces</h2>
        <div className="flex items-center gap-3">
          <a className="rounded bg-neutral-900 px-3 py-2 text-sm text-white dark:bg-neutral-100 dark:text-neutral-900" href="/workspaces/new">
            New workspace
          </a>
          <span className="text-sm opacity-70">signed in as {session.user?.email ?? session.user?.name}</span>
        </div>
      </div>

      <div className="rounded border border-dashed border-neutral-300 bg-white p-3 text-xs dark:border-neutral-800 dark:bg-neutral-900">
        <span className="font-medium">Dev correlation ID:</span>{" "}
        <code className="break-all">{responseCorrelationId}</code>
      </div>

      {error && (
        <p className="rounded border border-red-300 bg-red-50 p-3 text-sm text-red-800 dark:border-red-800 dark:bg-red-950 dark:text-red-200">
          Failed to load workspaces: {error}
        </p>
      )}

      <ul className="grid gap-3 md:grid-cols-2 lg:grid-cols-3">
        {workspaces.map((w) => (
          <li key={w.id} className="rounded border border-neutral-200 p-4 dark:border-neutral-800">
            <h3 className="font-medium">{w.name}</h3>
            {w.description && <p className="mt-1 text-sm opacity-70">{w.description}</p>}
            <p className="mt-2 text-xs opacity-50">id: {w.id}</p>
            <details className="mt-3 text-xs">
              <summary className="cursor-pointer opacity-70 hover:opacity-100">About this Workspace</summary>
              <div className="mt-2 space-y-1 opacity-80">
                <p>Tenant: <code>{w.tenant_id}</code></p>
                <p>Business Unit: <code>{w.business_unit_id}</code></p>
                <p>
                  Learn how Tenants, Business Units, and Workspaces relate in the{" "}
                  <a
                    className="underline"
                    href="https://github.com/forge-eng-fabric/forge-eng-fabric/blob/main/docs/concepts/tenancy-model.md"
                    target="_blank"
                    rel="noreferrer"
                  >
                    Tenancy Model
                  </a>
                  .
                </p>
              </div>
            </details>
          </li>
        ))}
        {workspaces.length === 0 && !error && (
          <li className="opacity-70">No workspaces yet. Create one with the API or CLI.</li>
        )}
      </ul>
    </section>
  );
}
