import { getServerSession } from "next-auth";
import { redirect } from "next/navigation";
import { authOptions } from "@/auth";
import { PageHead } from "@/components/page/PageHead";

type GitHubRepository = {
  name: string;
  full_name: string;
  private: boolean;
  html_url?: string;
  default_branch?: string;
};

type GitHubRepositoriesResponse = {
  installation_id: string;
  github_account: string;
  cache_hit: boolean;
  repositories: GitHubRepository[];
};

async function recordInstallation(formData: FormData) {
  "use server";

  const session = await getServerSession(authOptions);
  if (!session) redirect("/api/auth/signin");
  const token = (session as any).accessToken as string | undefined;
  if (!token) throw new Error("missing access token");

  const workspaceId = String(formData.get("workspace_id") ?? "").trim();
  const installationId = String(formData.get("installation_id") ?? "").trim();
  const githubAccount = String(formData.get("github_account") ?? "").trim();
  const scopes = String(formData.get("scopes") ?? "")
    .split(",")
    .map((scope) => scope.trim())
    .filter(Boolean);

  const cp = process.env.CONTROL_PLANE_URL ?? "http://localhost:8081";
  const response = await fetch(`${cp}/v1/workspaces/${workspaceId}/github/installations`, {
    method: "POST",
    headers: { authorization: `Bearer ${token}`, "content-type": "application/json" },
    body: JSON.stringify({ installation_id: installationId, github_account: githubAccount, scopes }),
  });
  if (!response.ok) {
    throw new Error(`control-plane ${response.status}: ${await response.text()}`);
  }
  redirect(`/settings/github?connected=1&workspace_id=${encodeURIComponent(workspaceId)}`);
}

async function fetchRepositories(workspaceId: string, token: string): Promise<{ data?: GitHubRepositoriesResponse; error?: string }> {
  const cp = process.env.CONTROL_PLANE_URL ?? "http://localhost:8081";
  const response = await fetch(`${cp}/v1/workspaces/${workspaceId}/github/repositories`, {
    headers: { authorization: `Bearer ${token}` },
    cache: "no-store",
  });
  if (!response.ok) {
    return { error: `control-plane ${response.status}: ${await response.text()}` };
  }
  return { data: (await response.json()) as GitHubRepositoriesResponse };
}

export default async function GitHubSettingsPage({ searchParams }: { searchParams: { connected?: string; workspace_id?: string } }) {
  const session = await getServerSession(authOptions);
  if (!session) redirect("/api/auth/signin");

  const installUrl = process.env.GITHUB_APP_INSTALL_URL;
  const token = (session as any).accessToken as string | undefined;
  const workspaceId = searchParams.workspace_id?.trim() ?? "";
  const repoResult = workspaceId && token ? await fetchRepositories(workspaceId, token) : null;

  return (
    <div style={{ maxWidth: 720 }}>
      <PageHead
        eyebrow="Account · Settings"
        title="Connect"
        titleEm="GitHub"
        sub="Install the Forge GitHub App or record a local fixture installation for Phase 0 smoke testing."
      />

      {searchParams.connected && (
        <p className="rounded border border-green-300 bg-green-50 p-3 text-sm text-green-800 dark:border-green-800 dark:bg-green-950 dark:text-green-200">
          GitHub installation recorded. Local repository listing is available below; real GitHub data requires an installation token.
        </p>
      )}

      <div className="rounded border border-neutral-200 bg-white p-5 dark:border-neutral-800 dark:bg-neutral-900">
        <h3 className="font-medium">Install flow</h3>
        <p className="mt-1 text-sm opacity-70">Set <code>GITHUB_APP_INSTALL_URL</code> to enable the real GitHub installation flow.</p>
        {installUrl ? (
          <a className="mt-4 inline-block rounded bg-neutral-900 px-4 py-2 text-sm font-medium text-white dark:bg-neutral-100 dark:text-neutral-900" href={installUrl}>
            Install Forge GitHub App
          </a>
        ) : (
          <p className="mt-4 text-sm opacity-70">No install URL configured for this local environment.</p>
        )}
      </div>

      <form action={recordInstallation} className="space-y-4 rounded border border-neutral-200 bg-white p-5 dark:border-neutral-800 dark:bg-neutral-900">
        <h3 className="font-medium">Record local installation</h3>
        <label className="grid gap-1 text-sm">
          <span className="font-medium">Workspace ID</span>
          <input name="workspace_id" required className="rounded border border-neutral-300 bg-transparent px-3 py-2 dark:border-neutral-700" />
        </label>
        <label className="grid gap-1 text-sm">
          <span className="font-medium">Installation ID</span>
          <input name="installation_id" required placeholder="local-installation" className="rounded border border-neutral-300 bg-transparent px-3 py-2 dark:border-neutral-700" />
        </label>
        <label className="grid gap-1 text-sm">
          <span className="font-medium">GitHub account</span>
          <input name="github_account" required placeholder="forge-local" className="rounded border border-neutral-300 bg-transparent px-3 py-2 dark:border-neutral-700" />
        </label>
        <label className="grid gap-1 text-sm">
          <span className="font-medium">Scopes</span>
          <input name="scopes" placeholder="metadata:read,contents:read" className="rounded border border-neutral-300 bg-transparent px-3 py-2 dark:border-neutral-700" />
        </label>
        <button type="submit" className="rounded bg-neutral-900 px-4 py-2 text-sm font-medium text-white dark:bg-neutral-100 dark:text-neutral-900">
          Record installation
        </button>
      </form>

      <div className="space-y-4 rounded border border-neutral-200 bg-white p-5 dark:border-neutral-800 dark:bg-neutral-900">
        <div>
          <h3 className="font-medium">Accessible repositories</h3>
          <p className="mt-1 text-sm opacity-70">List repositories for the latest installation recorded on a workspace. Results are cached by the Control Plane.</p>
        </div>
        <form className="flex flex-col gap-3 sm:flex-row" method="get">
          <input name="workspace_id" defaultValue={workspaceId} required placeholder="Workspace ID" className="min-w-0 flex-1 rounded border border-neutral-300 bg-transparent px-3 py-2 text-sm dark:border-neutral-700" />
          <button type="submit" className="rounded border border-neutral-300 px-4 py-2 text-sm font-medium dark:border-neutral-700">
            List repositories
          </button>
        </form>
        {repoResult?.error && (
          <p className="rounded border border-red-300 bg-red-50 p-3 text-sm text-red-800 dark:border-red-800 dark:bg-red-950 dark:text-red-200">{repoResult.error}</p>
        )}
        {repoResult?.data && (
          <div className="space-y-3 text-sm">
            <p className="opacity-70">
              Installation <code>{repoResult.data.installation_id}</code> for <code>{repoResult.data.github_account}</code> ({repoResult.data.cache_hit ? "cached" : "fresh"})
            </p>
            <div className="divide-y divide-neutral-200 rounded border border-neutral-200 dark:divide-neutral-800 dark:border-neutral-800">
              {repoResult.data.repositories.map((repo) => (
                <a key={repo.full_name} href={repo.html_url ?? "#"} className="block px-3 py-2 hover:bg-neutral-50 dark:hover:bg-neutral-800">
                  <span className="font-medium">{repo.full_name}</span>
                  <span className="ml-2 opacity-60">{repo.private ? "private" : "public"}</span>
                  {repo.default_branch && <span className="ml-2 opacity-60">branch {repo.default_branch}</span>}
                </a>
              ))}
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
