// App creation page (app-first-class-entity 10.5). Used by the wizard's
// "create a new App" branch and from the App picker. Uses a server action
// against the application service so we don't have to expose secrets to the
// client.

import { authOptions } from "@/auth";
import { getServerSession } from "next-auth";
import { redirect } from "next/navigation";

const applicationUrl = () => process.env.APPLICATION_URL ?? "http://localhost:8095";

async function createApp(formData: FormData) {
  "use server";
  const session = await getServerSession(authOptions);
  if (!session) redirect("/api/auth/signin");
  const token = (session as { accessToken?: string }).accessToken;
  const wsSlug = formData.get("workspace_slug") as string;
  const slug = (formData.get("slug") as string)?.trim();
  const name = (formData.get("name") as string)?.trim();
  const description = (formData.get("description") as string) ?? "";
  if (!wsSlug || !slug || !name) {
    redirect(`/workspaces/${wsSlug}/apps/new?error=${encodeURIComponent("slug and name are required")}`);
  }
  const body = {
    slug,
    name,
    description,
    owners: [(session.user?.email as string) || "self"],
  };
  try {
    const r = await fetch(`${applicationUrl()}/v1/workspaces/${wsSlug}/apps`, {
      method: "POST",
      headers: {
        "content-type": "application/json",
        ...(token ? { authorization: `Bearer ${token}` } : {}),
      },
      body: JSON.stringify(body),
    });
    if (!r.ok) {
      const text = await r.text();
      redirect(`/workspaces/${wsSlug}/apps/new?error=${encodeURIComponent(`${r.status}: ${text}`)}`);
    }
    const created = await r.json();
    redirect(`/workspaces/${wsSlug}/apps/${created.slug}`);
  } catch (e: any) {
    redirect(`/workspaces/${wsSlug}/apps/new?error=${encodeURIComponent(e?.message ?? "fetch failed")}`);
  }
}

export default async function NewAppPage({
  params,
  searchParams,
}: {
  params: { ws_slug: string };
  searchParams: { error?: string };
}) {
  const session = await getServerSession(authOptions);
  if (!session) redirect("/api/auth/signin");
  return (
    <div style={{ maxWidth: 540 }}>
      <h1 className="page-title">Create a new App</h1>
      <p className="page-sub">
        Apps anchor every OpenSpec, deployment and runtime to a stable product identity.
        Slug must be lowercase kebab-case and unique within the workspace.
      </p>
      {searchParams.error && (
        <div className="rounded border border-red-300 bg-red-50 p-3 text-sm text-red-800 dark:border-red-800 dark:bg-red-950 dark:text-red-200" style={{ marginBottom: 16 }}>
          {searchParams.error}
        </div>
      )}
      <form action={createApp} className="space-y-3 rounded border border-neutral-200 bg-white p-4 dark:border-neutral-800 dark:bg-neutral-900">
        <input type="hidden" name="workspace_slug" value={params.ws_slug} />
        <label className="block text-sm">
          <span className="mb-1 block font-medium">Slug</span>
          <input
            name="slug"
            required
            pattern="^[a-z0-9][a-z0-9_-]{0,62}$"
            placeholder="hr-portal"
            className="w-full rounded border border-neutral-300 px-3 py-2 text-sm dark:border-neutral-700 dark:bg-neutral-800"
          />
        </label>
        <label className="block text-sm">
          <span className="mb-1 block font-medium">Display name</span>
          <input
            name="name"
            required
            className="w-full rounded border border-neutral-300 px-3 py-2 text-sm dark:border-neutral-700 dark:bg-neutral-800"
          />
        </label>
        <label className="block text-sm">
          <span className="mb-1 block font-medium">Description</span>
          <textarea
            name="description"
            rows={3}
            className="w-full rounded border border-neutral-300 px-3 py-2 text-sm dark:border-neutral-700 dark:bg-neutral-800"
          />
        </label>
        <button className="rounded bg-neutral-900 px-4 py-2 text-sm font-medium text-white dark:bg-neutral-100 dark:text-neutral-900">
          Create App
        </button>
      </form>
    </div>
  );
}
