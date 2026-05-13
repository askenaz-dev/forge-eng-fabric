import { fetchTemplates, requirePortalIdentity } from "@/lib/onboarding";
import type { RepoTemplate } from "@/lib/onboarding-types";
import { PageHead } from "@/components/page/PageHead";
import { Button } from "@/components/primitives";

type SearchParams = { lifecycle_state?: string; trust_level?: string; category?: string };

export default async function TemplatesPage({ searchParams }: { searchParams: SearchParams }) {
  const identity = await requirePortalIdentity();
  let templates: RepoTemplate[] = [];
  let error: string | null = null;
  try {
    templates = await fetchTemplates(identity.token);
  } catch (e) {
    error = e instanceof Error ? e.message : "failed to load templates";
  }
  const filtered = templates.filter((template) => {
    if (searchParams.lifecycle_state && template.lifecycle_state !== searchParams.lifecycle_state) return false;
    if (searchParams.trust_level && template.trust_level !== searchParams.trust_level) return false;
    if (searchParams.category && template.category !== searchParams.category) return false;
    return true;
  });

  return (
    <>
      <PageHead
        eyebrow="Platform · Templates"
        title="Repository"
        titleEm="templates"
        sub="Approved repository templates with lifecycle, trust level and required capabilities."
        actions={
          <form method="get" style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
            <input name="category" defaultValue={searchParams.category ?? ""} placeholder="category" className="top-search" style={{ height: 32, width: 140 }} />
            <select name="lifecycle_state" defaultValue={searchParams.lifecycle_state ?? ""} style={{ height: 32, background: "var(--bg-card)", border: "1px solid var(--border)", borderRadius: "var(--r-2)", padding: "0 10px", color: "var(--fg)", fontFamily: "var(--f-sans)", fontSize: 13 }}>
            <option value="">any lifecycle</option>
            <option value="approved">approved</option>
            <option value="in_review">in_review</option>
            <option value="proposed">proposed</option>
          </select>
            <select name="trust_level" defaultValue={searchParams.trust_level ?? ""} style={{ height: 32, background: "var(--bg-card)", border: "1px solid var(--border)", borderRadius: "var(--r-2)", padding: "0 10px", color: "var(--fg)", fontFamily: "var(--f-sans)", fontSize: 13 }}>
              <option value="">any trust</option>
              {['T0', 'T1', 'T2', 'T3', 'T4', 'T5'].map((trust) => <option key={trust} value={trust}>{trust}</option>)}
            </select>
            <Button variant="primary" type="submit">Filter</Button>
          </form>
        }
      />
      {error && <p className="rounded border border-red-300 bg-red-50 p-3 text-sm text-red-800 dark:border-red-800 dark:bg-red-950 dark:text-red-200">{error}</p>}
      <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
        {filtered.map((template) => <TemplateCard key={`${template.id}@${template.version}`} template={template} />)}
        {filtered.length === 0 && !error && <p className="rounded border border-dashed border-neutral-300 p-6 text-sm opacity-70 dark:border-neutral-800">No templates match these filters.</p>}
      </div>
    </>
  );
}

function TemplateCard({ template }: { template: RepoTemplate }) {
  return (
    <article className="rounded-3xl border border-neutral-200 bg-white p-5 dark:border-neutral-800 dark:bg-neutral-900">
      <div className="flex items-start justify-between gap-3">
        <div>
          <p className="text-xs uppercase tracking-wide opacity-60">{template.category ?? "template"}</p>
          <h3 className="mt-1 text-lg font-semibold">{template.id}</h3>
          <p className="text-sm opacity-60">v{template.version}</p>
        </div>
        <div className="flex gap-2 text-xs font-semibold">
          <span className="rounded bg-green-100 px-2 py-1 text-green-800 dark:bg-green-950 dark:text-green-200">{template.lifecycle_state}</span>
          <span className="rounded bg-neutral-100 px-2 py-1 dark:bg-neutral-800">{template.trust_level}</span>
        </div>
      </div>
      <p className="mt-3 text-sm opacity-75">{template.description}</p>
      <div className="mt-4">
        <p className="text-xs font-semibold uppercase tracking-wide opacity-60">Parameters</p>
        <div className="mt-2 flex flex-wrap gap-2 text-xs">
          {Object.entries(template.parameters ?? {}).map(([name, spec]) => <span key={name} className="rounded bg-neutral-100 px-2 py-1 dark:bg-neutral-800">{name}{spec.required ? " *" : ""}</span>)}
        </div>
      </div>
      <div className="mt-4">
        <p className="text-xs font-semibold uppercase tracking-wide opacity-60">Capabilities</p>
        <p className="mt-2 text-xs leading-5 opacity-70">{(template.required_capabilities ?? []).join(" · ") || "none declared"}</p>
      </div>
    </article>
  );
}
