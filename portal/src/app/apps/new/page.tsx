import { fetchTemplates, requirePortalIdentity } from "@/lib/onboarding";
import type { RepoTemplate } from "@/lib/onboarding-types";
import { NewAppWizard } from "./wizard";

export default async function NewAppPage() {
  const identity = await requirePortalIdentity();
  let templates: RepoTemplate[] = [];
  let error: string | null = null;
  try {
    templates = await fetchTemplates(identity.token);
  } catch (e) {
    error = e instanceof Error ? e.message : "failed to load templates";
  }

  return (
    <section className="space-y-5">
      <div className="rounded-3xl border border-neutral-200 bg-white p-6 shadow-sm dark:border-neutral-800 dark:bg-neutral-900">
        <p className="text-xs font-semibold uppercase tracking-[0.2em] text-neutral-500">Golden path</p>
        <h2 className="mt-2 text-3xl font-semibold tracking-tight">New App</h2>
        <p className="mt-2 max-w-3xl text-sm text-neutral-600 dark:text-neutral-300">
          Select an approved template, supply Workspace parameters, inspect the generated repository contract, then submit onboarding.
        </p>
      </div>
      {error && <p className="rounded border border-red-300 bg-red-50 p-3 text-sm text-red-800 dark:border-red-800 dark:bg-red-950 dark:text-red-200">{error}</p>}
      <NewAppWizard templates={templates} />
    </section>
  );
}
