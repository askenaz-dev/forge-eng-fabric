import { getServerSession } from "next-auth";
import { redirect } from "next/navigation";
import { authOptions } from "@/auth";

async function createWorkspace(formData: FormData) {
  "use server";

  const session = await getServerSession(authOptions);
  if (!session) redirect("/api/auth/signin");
  const token = (session as any).accessToken as string | undefined;
  if (!token) throw new Error("missing access token");

  const businessUnitId = String(formData.get("business_unit_id") ?? "").trim();
  const name = String(formData.get("name") ?? "").trim();
  const description = String(formData.get("description") ?? "").trim();
  const owners = String(formData.get("owners") ?? "")
    .split(",")
    .map((owner) => owner.trim())
    .filter(Boolean);

  const cp = process.env.CONTROL_PLANE_URL ?? "http://localhost:8081";
  const response = await fetch(`${cp}/v1/business-units/${businessUnitId}/workspaces`, {
    method: "POST",
    headers: { authorization: `Bearer ${token}`, "content-type": "application/json" },
    body: JSON.stringify({ name, description, owners }),
  });
  if (!response.ok) {
    throw new Error(`control-plane ${response.status}: ${await response.text()}`);
  }
  redirect("/");
}

export default async function NewWorkspacePage() {
  const session = await getServerSession(authOptions);
  if (!session) redirect("/api/auth/signin");

  return (
    <section className="max-w-2xl space-y-5">
      <div>
        <h2 className="text-2xl font-semibold">New workspace</h2>
        <p className="mt-1 text-sm opacity-70">Create a workspace in an existing Business Unit.</p>
      </div>

      <form action={createWorkspace} className="space-y-4 rounded border border-neutral-200 bg-white p-5 dark:border-neutral-800 dark:bg-neutral-900">
        <label className="grid gap-1 text-sm">
          <span className="font-medium">Business Unit ID</span>
          <input name="business_unit_id" required className="rounded border border-neutral-300 bg-transparent px-3 py-2 dark:border-neutral-700" />
        </label>
        <label className="grid gap-1 text-sm">
          <span className="font-medium">Workspace name</span>
          <input name="name" required className="rounded border border-neutral-300 bg-transparent px-3 py-2 dark:border-neutral-700" />
        </label>
        <label className="grid gap-1 text-sm">
          <span className="font-medium">Description</span>
          <textarea name="description" rows={3} className="rounded border border-neutral-300 bg-transparent px-3 py-2 dark:border-neutral-700" />
        </label>
        <label className="grid gap-1 text-sm">
          <span className="font-medium">Owners</span>
          <input name="owners" required placeholder="alice,bob" className="rounded border border-neutral-300 bg-transparent px-3 py-2 dark:border-neutral-700" />
          <span className="text-xs opacity-60">Comma-separated local usernames or subject identifiers.</span>
        </label>
        <div className="flex items-center gap-3">
          <button type="submit" className="rounded bg-neutral-900 px-4 py-2 text-sm font-medium text-white dark:bg-neutral-100 dark:text-neutral-900">
            Create workspace
          </button>
          <a href="/" className="text-sm underline opacity-70">Cancel</a>
        </div>
      </form>
    </section>
  );
}
