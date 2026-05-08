import { fetchPipelineGates, requirePortalIdentity } from "@/lib/onboarding";
import type { PipelineGateResult } from "@/lib/onboarding-types";

type SearchParams = { workspace_id?: string; repo?: string; pr?: string };

export default async function PRGatesPage({ searchParams }: { searchParams: SearchParams }) {
  const identity = await requirePortalIdentity();
  let results: PipelineGateResult[] = [];
  let error: string | null = null;
  try {
    results = await fetchPipelineGates(searchParams, identity.token);
  } catch (e) {
    error = e instanceof Error ? e.message : "failed to load PR gates";
  }
  const stages = groupByStage(results);

  return (
    <section className="space-y-5">
      <div className="flex flex-col gap-3 md:flex-row md:items-end md:justify-between">
        <div>
          <h2 className="text-2xl font-semibold">PR gates</h2>
          <p className="mt-1 text-sm opacity-70">Quality, security, SBOM and signing gate results by stage with links to raw logs.</p>
        </div>
        <form className="flex flex-wrap gap-2 text-sm" method="get">
          <input name="workspace_id" defaultValue={searchParams.workspace_id ?? ""} placeholder="Workspace ID" className="rounded border border-neutral-300 bg-transparent px-3 py-2 dark:border-neutral-700" />
          <input name="repo" defaultValue={searchParams.repo ?? ""} placeholder="org/repo" className="rounded border border-neutral-300 bg-transparent px-3 py-2 dark:border-neutral-700" />
          <input name="pr" defaultValue={searchParams.pr ?? ""} placeholder="PR" className="w-24 rounded border border-neutral-300 bg-transparent px-3 py-2 dark:border-neutral-700" />
          <button className="rounded bg-neutral-900 px-4 py-2 text-white dark:bg-neutral-100 dark:text-neutral-900">Load</button>
        </form>
      </div>
      {error && <p className="rounded border border-red-300 bg-red-50 p-3 text-sm text-red-800 dark:border-red-800 dark:bg-red-950 dark:text-red-200">{error}</p>}
      <div className="grid gap-4">
        {Object.entries(stages).map(([stage, gates]) => <StageCard key={stage} stage={stage} gates={gates} />)}
        {results.length === 0 && !error && <p className="rounded border border-dashed border-neutral-300 p-6 text-sm opacity-70 dark:border-neutral-800">No gate results match this PR filter.</p>}
      </div>
    </section>
  );
}

function StageCard({ stage, gates }: { stage: string; gates: PipelineGateResult[] }) {
  const worst = gates.some((gate) => gate.outcome === "fail") ? "fail" : gates.some((gate) => gate.outcome === "warn") ? "warn" : "pass";
  return (
    <article className="rounded-3xl border border-neutral-200 bg-white p-5 dark:border-neutral-800 dark:bg-neutral-900">
      <div className="flex items-center justify-between">
        <h3 className="text-lg font-semibold">{stage}</h3>
        <span className={`rounded px-2 py-1 text-xs font-semibold ${worst === "fail" ? "bg-red-100 text-red-800 dark:bg-red-950 dark:text-red-200" : worst === "warn" ? "bg-yellow-100 text-yellow-900 dark:bg-yellow-950 dark:text-yellow-100" : "bg-green-100 text-green-800 dark:bg-green-950 dark:text-green-200"}`}>{worst}</span>
      </div>
      <div className="mt-4 overflow-x-auto">
        <table className="w-full text-left text-sm">
          <thead className="text-xs uppercase tracking-wide opacity-60"><tr><th className="py-2">Tool</th><th>Repo / PR</th><th>Commit</th><th>Outcome</th><th>Logs</th></tr></thead>
          <tbody>
            {gates.map((gate) => (
              <tr key={`${gate.repo_full_name}-${gate.pr_number}-${gate.stage}-${gate.tool}-${gate.commit_sha}`} className="border-t border-neutral-200 dark:border-neutral-800">
                <td className="py-2 font-medium">{gate.tool}</td>
                <td>{gate.repo_full_name}{gate.pr_number ? ` #${gate.pr_number}` : ""}</td>
                <td><code>{gate.commit_sha.slice(0, 12)}</code></td>
                <td>{gate.outcome}</td>
                <td>{gate.report_url ? <a className="underline" href={gate.report_url}>open logs</a> : <span className="opacity-50">none</span>}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </article>
  );
}

function groupByStage(results: PipelineGateResult[]) {
  return results.reduce<Record<string, PipelineGateResult[]>>((acc, result) => {
    acc[result.stage] ??= [];
    acc[result.stage].push(result);
    return acc;
  }, {});
}
