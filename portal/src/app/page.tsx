import { getServerSession } from "next-auth";
import { authOptions } from "./api/auth/[...nextauth]/route";
import { redirect } from "next/navigation";

type Workspace = {
  id: string;
  tenant_id: string;
  business_unit_id: string;
  name: string;
  description?: string;
  owners: string[];
  created_at: string;
};

async function fetchWorkspaces(token: string): Promise<Workspace[]> {
  const cp = process.env.CONTROL_PLANE_URL ?? "http://localhost:8081";
  const r = await fetch(`${cp}/v1/workspaces`, {
    headers: { authorization: `Bearer ${token}` },
    cache: "no-store",
  });
  if (!r.ok) throw new Error(`control-plane ${r.status}`);
  return r.json();
}

export default async function HomePage() {
  const session = await getServerSession(authOptions);
  if (!session) redirect("/api/auth/signin");

  const token = (session as any).accessToken as string | undefined;
  let workspaces: Workspace[] = [];
  let error: string | null = null;
  if (token) {
    try {
      workspaces = await fetchWorkspaces(token);
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
        <span className="text-sm opacity-70">signed in as {session.user?.email ?? session.user?.name}</span>
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
          </li>
        ))}
        {workspaces.length === 0 && !error && (
          <li className="opacity-70">No workspaces yet. Create one with the API or CLI.</li>
        )}
      </ul>
    </section>
  );
}
