type Phase = {
  name: string;
  status: "passed" | "in_progress" | "blocked" | "pending";
  gates: { name: string; outcome: "passed" | "failed" | "pending" }[];
  blockers: string[];
};

const initiatives = [
  {
    id: "init-204",
    title: "Checkout reliability uplift",
    openspec: "phase-4-sdlc-orchestration",
    jira: "KAN-1",
    workspace: "payments-platform",
    criticality: "high",
    phase: "security",
    cost: "$842 / $1,200",
  },
  {
    id: "init-188",
    title: "Internal developer portal search",
    openspec: "spec-dev-portal-search",
    jira: "KAN-8",
    workspace: "platform-experience",
    criticality: "medium",
    phase: "development",
    cost: "$210 / $800",
  },
];

const phases: Phase[] = [
  { name: "product", status: "passed", gates: [{ name: "acceptance_criteria_present", outcome: "passed" }, { name: "story_size_estimated", outcome: "passed" }], blockers: [] },
  { name: "architecture", status: "passed", gates: [{ name: "adrs_published", outcome: "passed" }, { name: "security_review_passed", outcome: "passed" }], blockers: [] },
  { name: "design", status: "passed", gates: [{ name: "api_contracts_defined", outcome: "passed" }, { name: "threat_model_present", outcome: "passed" }], blockers: [] },
  { name: "development", status: "passed", gates: [{ name: "unit_tests_passing", outcome: "passed" }, { name: "coverage", outcome: "passed" }], blockers: [] },
  { name: "qa", status: "passed", gates: [{ name: "e2e_tests_passing", outcome: "passed" }, { name: "perf_budget_met", outcome: "passed" }], blockers: [] },
  { name: "security", status: "blocked", gates: [{ name: "sast_clean", outcome: "failed" }, { name: "secrets_clean", outcome: "passed" }], blockers: ["SAST finding SEC-77 requires triage"] },
  { name: "devops", status: "pending", gates: [{ name: "pipelines_green", outcome: "pending" }], blockers: [] },
  { name: "sre", status: "pending", gates: [{ name: "slos_defined", outcome: "pending" }], blockers: [] },
  { name: "finops", status: "pending", gates: [{ name: "cost_estimate_within_budget", outcome: "pending" }], blockers: [] },
];

const graphNodes = ["OpenSpec", "Jira Epic", "ADR", "API Contract", "PR #42", "Stage Deploy", "SLO", "Cost Record"];

export default function InitiativesPage() {
  return (
    <section className="space-y-6">
      <div className="flex flex-col gap-2 md:flex-row md:items-end md:justify-between">
        <div>
          <p className="text-sm font-medium uppercase tracking-wide text-neutral-500">Initiatives</p>
          <h2 className="text-2xl font-semibold">SDLC phase progression</h2>
          <p className="mt-1 text-sm text-neutral-600 dark:text-neutral-300">Track cross-phase gates, blockers, traceability, and costs for each OpenSpec-led initiative.</p>
        </div>
        <a href="/openspecs" className="rounded bg-neutral-900 px-4 py-2 text-sm text-white dark:bg-neutral-100 dark:text-neutral-900">OpenSpec viewer</a>
      </div>

      <div className="grid gap-4 lg:grid-cols-[360px_1fr]">
        <aside className="space-y-3">
          {initiatives.map((initiative) => (
            <article key={initiative.id} className="rounded border border-neutral-200 bg-white p-4 dark:border-neutral-800 dark:bg-neutral-900">
              <div className="flex items-start justify-between gap-3">
                <div>
                  <h3 className="font-semibold">{initiative.title}</h3>
                  <p className="mt-1 text-xs text-neutral-500">{initiative.id} · {initiative.workspace}</p>
                </div>
                <span className="rounded-full bg-amber-50 px-2 py-1 text-xs text-amber-700 dark:bg-amber-950 dark:text-amber-200">{initiative.phase}</span>
              </div>
              <dl className="mt-4 grid grid-cols-2 gap-2 text-xs">
                <div><dt className="text-neutral-500">OpenSpec</dt><dd className="font-medium">{initiative.openspec}</dd></div>
                <div><dt className="text-neutral-500">Jira</dt><dd className="font-medium">{initiative.jira}</dd></div>
                <div><dt className="text-neutral-500">Criticality</dt><dd className="font-medium">{initiative.criticality}</dd></div>
                <div><dt className="text-neutral-500">Cost</dt><dd className="font-medium">{initiative.cost}</dd></div>
              </dl>
            </article>
          ))}
        </aside>

        <div className="space-y-5">
          <PhaseProgression phases={phases} />
          <TraceabilityGraph />
          <CostsPanel />
        </div>
      </div>
    </section>
  );
}

function PhaseProgression({ phases }: { phases: Phase[] }) {
  return (
    <div className="rounded border border-neutral-200 bg-white p-5 dark:border-neutral-800 dark:bg-neutral-900">
      <div className="flex items-center justify-between">
        <h3 className="font-semibold">Phase progression</h3>
        <span className="text-xs text-neutral-500">security blocked · override available</span>
      </div>
      <div className="mt-4 grid gap-3 md:grid-cols-3 xl:grid-cols-5">
        {phases.map((phase) => (
          <details key={phase.name} className={`rounded border p-3 ${phase.status === "blocked" ? "border-red-300 bg-red-50 dark:border-red-900 dark:bg-red-950" : "border-neutral-200 bg-neutral-50 dark:border-neutral-800 dark:bg-neutral-950"}`} open={phase.status === "blocked"}>
            <summary className="cursor-pointer text-sm font-medium">{phase.name}</summary>
            <div className="mt-3 space-y-2 text-xs">
              {phase.gates.map((gate) => <p key={gate.name}><span className="font-medium">{gate.outcome}</span> · {gate.name}</p>)}
              {phase.blockers.map((blocker) => <p key={blocker} className="rounded bg-white p-2 text-red-700 dark:bg-neutral-900 dark:text-red-200">{blocker}</p>)}
            </div>
          </details>
        ))}
      </div>
    </div>
  );
}

function TraceabilityGraph() {
  return (
    <div className="rounded border border-neutral-200 bg-white p-5 dark:border-neutral-800 dark:bg-neutral-900">
      <h3 className="font-semibold">Traceability graph</h3>
      <p className="mt-1 text-sm text-neutral-600 dark:text-neutral-300">Drill into OpenSpec → Jira → ADR → PR → deployment → SLO → cost relationships.</p>
      <div className="mt-4 grid gap-2 md:grid-cols-4">
        {graphNodes.map((node, index) => (
          <details key={node} className="rounded border border-neutral-200 bg-neutral-50 p-3 dark:border-neutral-800 dark:bg-neutral-950" open={index === 0}>
            <summary className="cursor-pointer text-sm font-medium">{node}</summary>
            <p className="mt-2 text-xs text-neutral-500">{index === 0 ? "root" : `hop ${index}`} · bidirectional link</p>
          </details>
        ))}
      </div>
    </div>
  );
}

function CostsPanel() {
  return (
    <div className="rounded border border-neutral-200 bg-white p-5 dark:border-neutral-800 dark:bg-neutral-900">
      <h3 className="font-semibold">Costs by initiative</h3>
      <div className="mt-4 grid gap-3 md:grid-cols-3">
        <Metric label="Cloud" value="$620" />
        <Metric label="LLM" value="$222" />
        <Metric label="Budget" value="70%" />
      </div>
    </div>
  );
}

function Metric({ label, value }: { label: string; value: string }) {
  return <div className="rounded bg-neutral-100 p-4 dark:bg-neutral-950"><p className="text-xs text-neutral-500">{label}</p><p className="mt-1 text-xl font-semibold">{value}</p></div>;
}
