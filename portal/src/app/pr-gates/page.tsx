import { fetchPipelineGates, requirePortalIdentity } from "@/lib/onboarding";
import type { PipelineGateResult } from "@/lib/onboarding-types";
import { PageHead } from "@/components/page/PageHead";
import { Button, Card } from "@/components/primitives";
import { ScopeSelect } from "@/components/scope/ScopeSelect";

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
    <>
      <PageHead
        eyebrow="Governance · PR gates"
        title="PR"
        titleEm="gates"
        sub="Quality, security, SBOM and signing gate results by stage with links to raw logs."
        actions={
          <form method="get" style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
            <ScopeSelect kind="workspace" name="workspace_id" defaultValue={searchParams.workspace_id ?? ""} className="top-search" style={{ height: 32, width: 180 }} />
            <input name="repo" defaultValue={searchParams.repo ?? ""} placeholder="org/repo" className="top-search" style={{ height: 32, width: 180 }} />
            <input name="pr" defaultValue={searchParams.pr ?? ""} placeholder="PR" className="top-search" style={{ height: 32, width: 80 }} />
            <Button variant="primary" type="submit">Load</Button>
          </form>
        }
      />
      {error && <Card style={{ marginBottom: 16 }}><div style={{ padding: 14, color: "var(--rust)" }}>{error}</div></Card>}
      <div className="grid gap-4">
        {Object.entries(stages).map(([stage, gates]) => <StageCard key={stage} stage={stage} gates={gates} />)}
        {results.length === 0 && !error && <p className="rounded border border-dashed border-neutral-300 p-6 text-sm opacity-70 dark:border-neutral-800">No gate results match this PR filter.</p>}
      </div>
    </>
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
