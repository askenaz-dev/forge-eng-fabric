import { redirect } from "next/navigation";

type PhaseStatus =
  | "not_started"
  | "in_progress"
  | "gate_pending"
  | "passed"
  | "failed"
  | "skipped"
  | "overridden"
  | "blocked";

type GateOutcome = "passed" | "failed" | "skipped";

type GateResult = {
  id: string;
  initiative_id: string;
  phase: string;
  gate: string;
  outcome: GateOutcome;
  reason?: string;
  evaluated_at: string;
  detail?: Record<string, unknown>;
};

type Blocker = {
  id: string;
  initiative_id: string;
  phase: string;
  gate: string;
  reason: string;
  created_at: string;
  resolved_at?: string;
};

type PhaseState = {
  initiative_id: string;
  phase: string;
  status: PhaseStatus;
  entered_at?: string;
  completed_at?: string;
  gates: GateResult[];
  blockers: Blocker[];
};

type Initiative = {
  id: string;
  workspace_id: string;
  openspec_root: string;
  jira_epic_key?: string;
  criticality: string;
  current_phase: string;
  phase_states: PhaseState[];
  created_at: string;
  updated_at: string;
};

type Specification = {
  openspec_id: string;
  title: string;
  version: number;
};

type TraceabilityNode = {
  id: string;
  type: string;
  external_id: string;
  workspace_id: string;
  metadata?: Record<string, unknown>;
};

type TraceabilityLink = {
  id: string;
  from_node: string;
  to_node: string;
  relation: string;
  source: string;
  source_event: string;
};

type TraceabilityGraph = {
  openspec_id: string;
  depth: number;
  nodes: TraceabilityNode[];
  links: TraceabilityLink[];
  materialized_at: string;
};

type Budget = {
  id: string;
  workspace_id: string;
  initiative_openspec: string;
  monthly_limit_usd: number;
  consumed_usd: number;
  thresholds: number[];
};

type BudgetAlert = {
  event_type: string;
  workspace_id: string;
  initiative_openspec: string;
  threshold: number;
  consumed_usd: number;
  monthly_limit_usd: number;
  budget_id: string;
};

type FinOpsDashboard = {
  cost_by_initiative: Record<string, number>;
  budgets: Budget[];
  events: BudgetAlert[];
};

type SearchParams = {
  workspace_id?: string;
  initiative_id?: string;
  saved?: string;
  error?: string;
};

const sdlcUrl = () => process.env.SDLC_ORCHESTRATOR_URL ?? "http://localhost:8089";
const traceabilityUrl = () => process.env.TRACEABILITY_URL ?? "http://localhost:8090";
const finopsUrl = () => process.env.FINOPS_URL ?? "http://localhost:8122";
const specificationUrl = () => process.env.OPENSPEC_URL ?? "http://localhost:8083";

async function createInitiative(formData: FormData) {
  "use server";
  const workspaceId = required(formData, "workspace_id");
  const openspecRoot = required(formData, "openspec_root");
  const payload = {
    workspace_id: workspaceId,
    openspec_root: openspecRoot,
    jira_epic_key: optional(formData, "jira_epic_key") || undefined,
    criticality: optional(formData, "criticality") || "medium",
    actor: "portal",
  };

  const response = await fetch(`${sdlcUrl()}/v1/initiatives`, {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify(payload),
  });
  if (!response.ok) {
    redirect(`/initiatives?workspace_id=${encodeURIComponent(workspaceId)}&error=${encodeURIComponent(await response.text())}`);
  }
  const created = (await response.json()) as Initiative;
  redirect(`/initiatives?workspace_id=${encodeURIComponent(workspaceId)}&initiative_id=${created.id}&saved=initiative`);
}

async function evaluateCurrentPhase(formData: FormData) {
  "use server";
  const workspaceId = required(formData, "workspace_id");
  const initiativeId = required(formData, "initiative_id");
  const phase = required(formData, "phase");
  const evidenceRaw = optional(formData, "evidence_json");
  let evidence: Record<string, unknown> = {};
  if (evidenceRaw) {
    try {
      evidence = JSON.parse(evidenceRaw) as Record<string, unknown>;
    } catch {
      redirect(`/initiatives?workspace_id=${encodeURIComponent(workspaceId)}&initiative_id=${initiativeId}&error=${encodeURIComponent("Evidence must be valid JSON")}`);
    }
  }

  const response = await fetch(`${sdlcUrl()}/v1/initiatives/${encodeURIComponent(initiativeId)}/phase/${encodeURIComponent(phase)}/complete`, {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify({ actor: "portal", evidence }),
  });
  if (!response.ok) {
    redirect(`/initiatives?workspace_id=${encodeURIComponent(workspaceId)}&initiative_id=${initiativeId}&error=${encodeURIComponent(await response.text())}`);
  }
  redirect(`/initiatives?workspace_id=${encodeURIComponent(workspaceId)}&initiative_id=${initiativeId}&saved=phase`);
}

export default async function InitiativesPage({ searchParams }: { searchParams: SearchParams }) {
  const workspaceId = searchParams.workspace_id?.trim() ?? "";
  const errors: string[] = [];
  let initiatives: Initiative[] = [];
  let specifications: Specification[] = [];

  if (workspaceId) {
    const [initiativeResult, specificationResult] = await Promise.all([
      fetchInitiatives(workspaceId),
      fetchSpecifications(workspaceId),
    ]);
    initiatives = initiativeResult.data ?? [];
    specifications = specificationResult.data ?? [];
    if (initiativeResult.error) errors.push(initiativeResult.error);
    if (specificationResult.error) errors.push(specificationResult.error);
  }

  const selected = initiatives.find((initiative) => initiative.id === searchParams.initiative_id) ?? initiatives[0] ?? null;
  const specById = new Map(specifications.map((spec) => [spec.openspec_id, spec]));
  const selectedSpec = selected ? specById.get(selected.openspec_root) ?? null : null;
  const [traceability, finops] = selected
    ? await Promise.all([fetchTraceability(selected.openspec_root), fetchFinOps()])
    : [{ data: null }, { data: null }];
  if (traceability.error) errors.push(traceability.error);
  if (finops.error) errors.push(finops.error);

  return (
    <section className="space-y-6">
      <div className="flex flex-col gap-3 md:flex-row md:items-end md:justify-between">
        <div>
          <p className="text-sm font-medium uppercase tracking-wide text-neutral-500">Initiatives</p>
          <h2 className="text-2xl font-semibold">SDLC phase progression</h2>
          <p className="mt-1 text-sm text-neutral-600 dark:text-neutral-300">
            Live initiative state from the SDLC orchestrator, traceability graph, and FinOps attribution.
          </p>
        </div>
        <form className="flex gap-2" method="get">
          <input name="workspace_id" defaultValue={workspaceId} placeholder="Workspace ID" className="min-w-0 rounded border border-neutral-300 bg-transparent px-3 py-2 text-sm dark:border-neutral-700" />
          <button className="rounded bg-neutral-900 px-4 py-2 text-sm text-white dark:bg-neutral-100 dark:text-neutral-900">Load</button>
        </form>
      </div>

      {searchParams.saved && <p className="rounded border border-green-300 bg-green-50 p-3 text-sm text-green-800 dark:border-green-800 dark:bg-green-950 dark:text-green-200">Initiative state saved.</p>}
      {searchParams.error && <p className="rounded border border-red-300 bg-red-50 p-3 text-sm text-red-800 dark:border-red-800 dark:bg-red-950 dark:text-red-200">{searchParams.error}</p>}
      {errors.map((error) => (
        <p key={error} className="rounded border border-amber-300 bg-amber-50 p-3 text-sm text-amber-900 dark:border-amber-800 dark:bg-amber-950 dark:text-amber-200">{error}</p>
      ))}

      {!workspaceId ? (
        <EmptyPanel title="Load a Workspace" body="Enter a Workspace ID to read initiatives from the SDLC orchestrator." />
      ) : (
        <div className="grid gap-4 lg:grid-cols-[360px_1fr]">
          <aside className="space-y-4">
            <InitiativeList workspaceId={workspaceId} initiatives={initiatives} specById={specById} selectedId={selected?.id} />
            <CreateInitiativeForm workspaceId={workspaceId} specifications={specifications} />
          </aside>

          <div className="space-y-5">
            {selected ? (
              <>
                <InitiativeSummary initiative={selected} specification={selectedSpec} />
                <PhaseProgression initiative={selected} />
                <PhaseEvaluationForm initiative={selected} />
                <TraceabilityPanel graph={traceability.data ?? null} />
                <CostsPanel dashboard={finops.data ?? null} workspaceId={workspaceId} openspecRoot={selected.openspec_root} />
              </>
            ) : (
              <EmptyPanel title="No Initiatives" body="The SDLC orchestrator returned no initiatives for this Workspace. Create one from a committed specification to start real phase tracking." />
            )}
          </div>
        </div>
      )}
    </section>
  );
}

function InitiativeList({
  workspaceId,
  initiatives,
  specById,
  selectedId,
}: {
  workspaceId: string;
  initiatives: Initiative[];
  specById: Map<string, Specification>;
  selectedId?: string;
}) {
  return (
    <div className="space-y-3">
      {initiatives.map((initiative) => {
        const spec = specById.get(initiative.openspec_root);
        const selected = initiative.id === selectedId;
        return (
          <a
            key={initiative.id}
            href={`/initiatives?workspace_id=${encodeURIComponent(workspaceId)}&initiative_id=${initiative.id}`}
            className={`block rounded border p-4 ${selected ? "border-neutral-900 bg-neutral-100 dark:border-neutral-100 dark:bg-neutral-800" : "border-neutral-200 bg-white dark:border-neutral-800 dark:bg-neutral-900"}`}
          >
            <div className="flex items-start justify-between gap-3">
              <div>
                <h3 className="font-semibold">{spec?.title ?? initiative.openspec_root}</h3>
                <p className="mt-1 text-xs text-neutral-500">{initiative.id}</p>
              </div>
              <span className="rounded-full bg-neutral-100 px-2 py-1 text-xs text-neutral-700 dark:bg-neutral-950 dark:text-neutral-200">{initiative.current_phase}</span>
            </div>
            <dl className="mt-4 grid grid-cols-2 gap-2 text-xs">
              <div><dt className="text-neutral-500">Specification</dt><dd className="font-medium">{initiative.openspec_root}</dd></div>
              <div><dt className="text-neutral-500">Jira</dt><dd className="font-medium">{initiative.jira_epic_key || "none"}</dd></div>
              <div><dt className="text-neutral-500">Criticality</dt><dd className="font-medium">{initiative.criticality}</dd></div>
              <div><dt className="text-neutral-500">Updated</dt><dd className="font-medium">{formatDate(initiative.updated_at)}</dd></div>
            </dl>
          </a>
        );
      })}
      {initiatives.length === 0 && <EmptyPanel title="No Initiative Records" body="No records were returned by the SDLC orchestrator for this Workspace." />}
    </div>
  );
}

function CreateInitiativeForm({ workspaceId, specifications }: { workspaceId: string; specifications: Specification[] }) {
  return (
    <form action={createInitiative} className="space-y-3 rounded border border-neutral-200 bg-white p-4 dark:border-neutral-800 dark:bg-neutral-900">
      <h3 className="font-medium">Create initiative</h3>
      <p className="text-xs opacity-70">Creates a real SDLC orchestrator record from an existing committed specification.</p>
      <input type="hidden" name="workspace_id" value={workspaceId} />
      <label className="grid gap-1 text-sm">
        <span className="font-medium">Specification ID</span>
        <input name="openspec_root" required list="specification-options" className="rounded border border-neutral-300 bg-transparent px-3 py-2 dark:border-neutral-700" />
        <datalist id="specification-options">
          {specifications.map((spec) => <option key={spec.openspec_id} value={spec.openspec_id}>{spec.title}</option>)}
        </datalist>
      </label>
      <label className="grid gap-1 text-sm">
        <span className="font-medium">Jira epic key</span>
        <input name="jira_epic_key" className="rounded border border-neutral-300 bg-transparent px-3 py-2 dark:border-neutral-700" />
      </label>
      <label className="grid gap-1 text-sm">
        <span className="font-medium">Criticality</span>
        <select name="criticality" defaultValue="medium" className="rounded border border-neutral-300 bg-transparent px-3 py-2 dark:border-neutral-700">
          <option value="low">low</option>
          <option value="medium">medium</option>
          <option value="high">high</option>
          <option value="critical">critical</option>
        </select>
      </label>
      <button className="rounded bg-neutral-900 px-4 py-2 text-sm font-medium text-white dark:bg-neutral-100 dark:text-neutral-900">Create initiative</button>
    </form>
  );
}

function InitiativeSummary({ initiative, specification }: { initiative: Initiative; specification: Specification | null }) {
  return (
    <div className="rounded border border-neutral-200 bg-white p-5 dark:border-neutral-800 dark:bg-neutral-900">
      <div className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
        <div>
          <h3 className="font-semibold">{specification?.title ?? initiative.openspec_root}</h3>
          <p className="mt-1 text-sm text-neutral-600 dark:text-neutral-300">
            Current phase: <span className="font-medium">{initiative.current_phase}</span>
          </p>
        </div>
        <a href={`/openspecs?workspace_id=${encodeURIComponent(initiative.workspace_id)}&openspec_id=${encodeURIComponent(initiative.openspec_root)}`} className="rounded border border-neutral-300 px-4 py-2 text-sm dark:border-neutral-700">
          Open specification
        </a>
      </div>
    </div>
  );
}

function PhaseProgression({ initiative }: { initiative: Initiative }) {
  return (
    <div className="rounded border border-neutral-200 bg-white p-5 dark:border-neutral-800 dark:bg-neutral-900">
      <div className="flex items-center justify-between">
        <h3 className="font-semibold">Phase progression</h3>
        <span className="text-xs text-neutral-500">source: sdlc-orchestrator</span>
      </div>
      <div className="mt-4 grid gap-3 md:grid-cols-3 xl:grid-cols-5">
        {initiative.phase_states.map((phase) => (
          <details key={phase.phase} className={`rounded border p-3 ${phaseClass(phase.status)}`} open={phase.status === "blocked" || phase.status === "in_progress" || phase.status === "gate_pending"}>
            <summary className="cursor-pointer text-sm font-medium">{phase.phase}</summary>
            <div className="mt-3 space-y-2 text-xs">
              <p><span className="font-medium">status</span> · {phase.status}</p>
              {phase.entered_at && <p><span className="font-medium">entered</span> · {formatDate(phase.entered_at)}</p>}
              {phase.completed_at && <p><span className="font-medium">completed</span> · {formatDate(phase.completed_at)}</p>}
              {phase.gates.map((gate) => (
                <p key={gate.id}><span className="font-medium">{gate.outcome}</span> · {gate.gate}{gate.reason ? ` · ${gate.reason}` : ""}</p>
              ))}
              {phase.blockers.filter((blocker) => !blocker.resolved_at).map((blocker) => (
                <p key={blocker.id} className="rounded bg-white p-2 text-red-700 dark:bg-neutral-900 dark:text-red-200">{blocker.reason}</p>
              ))}
              {phase.gates.length === 0 && phase.blockers.length === 0 && <p className="text-neutral-500">No gate evaluations recorded.</p>}
            </div>
          </details>
        ))}
      </div>
    </div>
  );
}

function PhaseEvaluationForm({ initiative }: { initiative: Initiative }) {
  if (initiative.current_phase === "done") return null;
  return (
    <form action={evaluateCurrentPhase} className="space-y-3 rounded border border-neutral-200 bg-white p-5 dark:border-neutral-800 dark:bg-neutral-900">
      <div>
        <h3 className="font-semibold">Evaluate current phase</h3>
        <p className="mt-1 text-sm text-neutral-600 dark:text-neutral-300">Submit evidence JSON to the SDLC orchestrator for the current phase.</p>
      </div>
      <input type="hidden" name="workspace_id" value={initiative.workspace_id} />
      <input type="hidden" name="initiative_id" value={initiative.id} />
      <input type="hidden" name="phase" value={initiative.current_phase} />
      <textarea
        name="evidence_json"
        rows={5}
        placeholder='{"acceptance_criteria_present": true, "story_size_estimated": true}'
        className="w-full rounded border border-neutral-300 bg-transparent px-3 py-2 font-mono text-xs dark:border-neutral-700"
      />
      <button className="rounded bg-neutral-900 px-4 py-2 text-sm font-medium text-white dark:bg-neutral-100 dark:text-neutral-900">Evaluate phase</button>
    </form>
  );
}

function TraceabilityPanel({ graph }: { graph: TraceabilityGraph | null }) {
  return (
    <div className="rounded border border-neutral-200 bg-white p-5 dark:border-neutral-800 dark:bg-neutral-900">
      <h3 className="font-semibold">Traceability graph</h3>
      <p className="mt-1 text-sm text-neutral-600 dark:text-neutral-300">Materialized artifact graph from the traceability service.</p>
      {graph && graph.nodes.length > 0 ? (
        <div className="mt-4 grid gap-2 md:grid-cols-3">
          {graph.nodes.map((node) => (
            <details key={node.id} className="rounded border border-neutral-200 bg-neutral-50 p-3 dark:border-neutral-800 dark:bg-neutral-950">
              <summary className="cursor-pointer text-sm font-medium">{node.type}</summary>
              <p className="mt-2 break-all text-xs text-neutral-500">{node.external_id}</p>
            </details>
          ))}
        </div>
      ) : (
        <p className="mt-4 rounded border border-dashed border-neutral-300 p-4 text-sm opacity-70 dark:border-neutral-800">
          No traceability nodes returned for this specification.
        </p>
      )}
      {graph && graph.links.length > 0 && <p className="mt-3 text-xs text-neutral-500">{graph.links.length} links · materialized {formatDate(graph.materialized_at)}</p>}
    </div>
  );
}

function CostsPanel({ dashboard, workspaceId, openspecRoot }: { dashboard: FinOpsDashboard | null; workspaceId: string; openspecRoot: string }) {
  const cost = dashboard?.cost_by_initiative?.[openspecRoot] ?? 0;
  const budgets = dashboard?.budgets.filter((budget) => budget.workspace_id === workspaceId && budget.initiative_openspec === openspecRoot) ?? [];
  const events = dashboard?.events.filter((event) => event.workspace_id === workspaceId && event.initiative_openspec === openspecRoot) ?? [];
  return (
    <div className="rounded border border-neutral-200 bg-white p-5 dark:border-neutral-800 dark:bg-neutral-900">
      <h3 className="font-semibold">Costs by initiative</h3>
      <div className="mt-4 grid gap-3 md:grid-cols-3">
        <Metric label="Attributed spend" value={formatUsd(cost)} />
        <Metric label="Budgets" value={String(budgets.length)} />
        <Metric label="Budget alerts" value={String(events.length)} />
      </div>
      {dashboard && cost === 0 && budgets.length === 0 && events.length === 0 && (
        <p className="mt-4 rounded border border-dashed border-neutral-300 p-4 text-sm opacity-70 dark:border-neutral-800">
          No FinOps records returned for this specification.
        </p>
      )}
    </div>
  );
}

function Metric({ label, value }: { label: string; value: string }) {
  return <div className="rounded bg-neutral-100 p-4 dark:bg-neutral-950"><p className="text-xs text-neutral-500">{label}</p><p className="mt-1 text-xl font-semibold">{value}</p></div>;
}

function EmptyPanel({ title, body }: { title: string; body: string }) {
  return (
    <div className="rounded border border-dashed border-neutral-300 p-6 text-sm dark:border-neutral-800">
      <h3 className="font-medium">{title}</h3>
      <p className="mt-1 opacity-70">{body}</p>
    </div>
  );
}

async function fetchInitiatives(workspaceId: string): Promise<{ data?: Initiative[]; error?: string }> {
  try {
    const response = await fetch(`${sdlcUrl()}/v1/initiatives?workspace_id=${encodeURIComponent(workspaceId)}`, { cache: "no-store" });
    if (!response.ok) return { error: `sdlc-orchestrator ${response.status}: ${await response.text()}` };
    return { data: ((await response.json()) as { initiatives: Initiative[] }).initiatives };
  } catch (error) {
    return { error: `sdlc-orchestrator unavailable: ${errorMessage(error)}` };
  }
}

async function fetchSpecifications(workspaceId: string): Promise<{ data?: Specification[]; error?: string }> {
  try {
    const response = await fetch(`${specificationUrl()}/v1/openspecs?workspace_id=${encodeURIComponent(workspaceId)}`, { cache: "no-store" });
    if (!response.ok) return { error: `specification service ${response.status}: ${await response.text()}` };
    return { data: ((await response.json()) as { openspecs: Specification[] }).openspecs };
  } catch (error) {
    return { error: `specification service unavailable: ${errorMessage(error)}` };
  }
}

async function fetchTraceability(openspecRoot: string): Promise<{ data?: TraceabilityGraph | null; error?: string }> {
  try {
    const response = await fetch(`${traceabilityUrl()}/v1/traceability/${encodeURIComponent(openspecRoot)}?depth=4`, { cache: "no-store" });
    if (!response.ok) return { error: `traceability ${response.status}: ${await response.text()}` };
    return { data: (await response.json()) as TraceabilityGraph };
  } catch (error) {
    return { error: `traceability unavailable: ${errorMessage(error)}` };
  }
}

async function fetchFinOps(): Promise<{ data?: FinOpsDashboard | null; error?: string }> {
  try {
    const response = await fetch(`${finopsUrl()}/v1/dashboard`, { cache: "no-store" });
    if (!response.ok) return { error: `finops ${response.status}: ${await response.text()}` };
    return { data: (await response.json()) as FinOpsDashboard };
  } catch (error) {
    return { error: `finops unavailable: ${errorMessage(error)}` };
  }
}

function phaseClass(status: PhaseStatus) {
  if (status === "blocked" || status === "failed") return "border-red-300 bg-red-50 dark:border-red-900 dark:bg-red-950";
  if (status === "passed" || status === "overridden") return "border-emerald-300 bg-emerald-50 dark:border-emerald-900 dark:bg-emerald-950";
  if (status === "in_progress" || status === "gate_pending") return "border-amber-300 bg-amber-50 dark:border-amber-900 dark:bg-amber-950";
  return "border-neutral-200 bg-neutral-50 dark:border-neutral-800 dark:bg-neutral-950";
}

function formatDate(value: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return new Intl.DateTimeFormat("en", { dateStyle: "medium", timeStyle: "short" }).format(date);
}

function formatUsd(value: number) {
  return new Intl.NumberFormat("en-US", { style: "currency", currency: "USD" }).format(value);
}

function required(formData: FormData, key: string) {
  const value = optional(formData, key);
  if (!value) throw new Error(`${key} is required`);
  return value;
}

function optional(formData: FormData, key: string) {
  return String(formData.get(key) ?? "").trim();
}

function errorMessage(error: unknown) {
  return error instanceof Error ? error.message : "fetch failed";
}
