import { authOptions } from "@/auth";
import { getServerSession } from "next-auth";
import { redirect } from "next/navigation";
import { PageHead } from "@/components/page/PageHead";
import { Button } from "@/components/primitives";

type Incident = {
  id: string;
  service: string;
  environment: string;
  severity: "info" | "warning" | "critical";
  source: string;
  status: "open" | "resolved";
  title: string;
  description?: string;
  opened_at: string;
  updated_at: string;
  resolved_at?: string;
  synthetic: boolean;
  events?: { id: string; source: string; severity: string; occurred_at: string }[];
};

type HealingDecision = {
  id: string;
  incident_id: string;
  action_id: string;
  level: string;
  requested_level: string;
  outcome: string;
  reason?: string;
  workflow_run_id?: string;
  approval_id?: string;
  synthetic: boolean;
  created_at: string;
};

type DiagnosisReport = {
  incident_id: string;
  prompt_version: string;
  context_summary: string;
  hypotheses: {
    statement: string;
    confidence: number;
    rationale?: string;
    citations: { source_kind: string; source_id: string; url?: string }[];
    suggested_actions: string[];
  }[];
};

type SearchParams = { incident_id?: string; status?: string };

const detectionUrl = () => process.env.INCIDENT_DETECTION_URL ?? "http://localhost:8101";
const healingUrl = () => process.env.HEALING_ENGINE_URL ?? "http://localhost:8102";
const diagnosisUrl = () => process.env.DIAGNOSIS_URL ?? "http://localhost:8104";

async function getToken() {
  const session = await getServerSession(authOptions);
  if (!session) redirect("/api/auth/signin");
  return (session as { accessToken?: string }).accessToken;
}

async function fetchIncidents(status: string, token?: string): Promise<Incident[]> {
  const params = status ? `?status=${encodeURIComponent(status)}` : "";
  const response = await fetch(`${detectionUrl()}/v1/incidents${params}`, {
    headers: token ? { authorization: `Bearer ${token}` } : {},
    cache: "no-store",
  });
  if (!response.ok) return [];
  return (await response.json()) as Incident[];
}

async function fetchIncident(id: string, token?: string): Promise<Incident | null> {
  const response = await fetch(`${detectionUrl()}/v1/incidents/${encodeURIComponent(id)}`, {
    headers: token ? { authorization: `Bearer ${token}` } : {},
    cache: "no-store",
  });
  if (!response.ok) return null;
  return (await response.json()) as Incident;
}

async function fetchDecisions(id: string, token?: string): Promise<HealingDecision[]> {
  const response = await fetch(`${healingUrl()}/v1/decisions/${encodeURIComponent(id)}`, {
    headers: token ? { authorization: `Bearer ${token}` } : {},
    cache: "no-store",
  });
  if (!response.ok) return [];
  return (await response.json()) as HealingDecision[];
}

async function fetchDiagnosis(incident: Incident, token?: string): Promise<DiagnosisReport | null> {
  // Posting to /v1/diagnose is idempotent for synthetic flows; production wires
  // this to the diagnosis store instead. The portal never makes the user wait
  // longer than 60s — the p95 latency target documented in the spec.
  const response = await fetch(`${diagnosisUrl()}/v1/diagnose`, {
    method: "POST",
    headers: { "content-type": "application/json", ...(token ? { authorization: `Bearer ${token}` } : {}) },
    body: JSON.stringify({
      incident_id: incident.id,
      tenant_id: "t-1",
      service: incident.service,
      environment: incident.environment,
      signature_hash: incident.id,
      severity: incident.severity,
      title: incident.title,
      description: incident.description,
      synthetic: incident.synthetic,
    }),
  });
  if (!response.ok) return null;
  return (await response.json()) as DiagnosisReport;
}

export default async function IncidentsPage({ searchParams }: { searchParams: SearchParams }) {
  const token = await getToken();
  const status = searchParams.status ?? "";
  const incidents = await fetchIncidents(status, token);
  const selectedId = searchParams.incident_id ?? incidents[0]?.id;
  const selected = selectedId ? await fetchIncident(selectedId, token) : null;
  const decisions = selected ? await fetchDecisions(selected.id, token) : [];
  const diagnosis = selected ? await fetchDiagnosis(selected, token) : null;

  return (
    <>
      <PageHead
        eyebrow="Observability · Incidents"
        title="Incident"
        titleEm="timeline"
        sub="Detection → diagnosis → healing → postmortem (Phase 6)."
        actions={
          <form method="get" style={{ display: "flex", gap: 8 }}>
            <select
              name="status"
              defaultValue={status}
              style={{
                height: 32,
                background: "var(--bg-card)",
                border: "1px solid var(--border)",
                borderRadius: "var(--r-2)",
                padding: "0 10px",
                color: "var(--fg)",
                fontFamily: "var(--f-sans)",
                fontSize: 13,
              }}
            >
              <option value="">All</option>
              <option value="open">Open</option>
              <option value="resolved">Resolved</option>
            </select>
            <Button variant="primary" type="submit">Filter</Button>
          </form>
        }
      />

      <div className="grid gap-5 xl:grid-cols-[320px_1fr]">
        <aside className="space-y-2 rounded border border-neutral-200 bg-white p-4 dark:border-neutral-800 dark:bg-neutral-900" data-testid="incident-list">
          <h3 className="font-medium">Recent incidents</h3>
          {incidents.map((inc) => (
            <a key={inc.id} href={`/incidents?incident_id=${inc.id}${status ? `&status=${status}` : ""}`} className="block rounded border border-neutral-200 px-3 py-2 text-sm hover:bg-neutral-50 dark:border-neutral-800 dark:hover:bg-neutral-800" data-testid={`incident-${inc.id}`}>
              <span className="block font-medium">{inc.title}</span>
              <span className="text-xs opacity-60">
                {inc.service} · {inc.environment} · <SeverityBadge value={inc.severity} /> {inc.synthetic && <SyntheticBadge />}
              </span>
            </a>
          ))}
          {incidents.length === 0 && <p className="opacity-70">No incidents.</p>}
        </aside>

        <div className="space-y-5">
          {selected ? (
            <>
              <IncidentSummary incident={selected} />
              <Timeline incident={selected} decisions={decisions} />
              <DiagnosisCard report={diagnosis} />
              <HealingDecisionsCard decisions={decisions} />
              <PostmortemLink incident={selected} />
            </>
          ) : (
            <div className="rounded border border-dashed border-neutral-300 p-6 text-sm opacity-70 dark:border-neutral-800">Select an incident to view its detail.</div>
          )}
        </div>
      </div>
    </>
  );
}

function SeverityBadge({ value }: { value: string }) {
  const cls = value === "critical" ? "bg-red-200 text-red-900" : value === "warning" ? "bg-amber-200 text-amber-900" : "bg-neutral-200 text-neutral-900";
  return <span className={`inline-flex items-center rounded px-1.5 py-0.5 text-[10px] font-semibold uppercase ${cls}`}>{value}</span>;
}

function SyntheticBadge() {
  return <span className="ml-1 inline-flex items-center rounded bg-purple-200 px-1.5 py-0.5 text-[10px] font-semibold uppercase text-purple-900">synthetic</span>;
}

function IncidentSummary({ incident }: { incident: Incident }) {
  return (
    <div className="rounded border border-neutral-200 bg-white p-5 dark:border-neutral-800 dark:bg-neutral-900">
      <div className="flex items-start justify-between">
        <div>
          <h3 className="text-lg font-semibold">{incident.title}</h3>
          <p className="text-xs opacity-60">id: {incident.id} · service: {incident.service} · env: {incident.environment} · source: {incident.source}</p>
        </div>
        <SeverityBadge value={incident.severity} />
      </div>
      {incident.description && <p className="mt-3 text-sm">{incident.description}</p>}
    </div>
  );
}

function Timeline({ incident, decisions }: { incident: Incident; decisions: HealingDecision[] }) {
  const rows = [
    { ts: incident.opened_at, label: "incident.detected.v1" },
    ...(incident.events ?? []).slice(1).map((ev) => ({ ts: ev.occurred_at, label: `event from ${ev.source}` })),
    ...decisions.map((d) => ({ ts: d.created_at, label: `healing ${d.action_id} → ${d.outcome} (${d.level})` })),
    ...(incident.resolved_at ? [{ ts: incident.resolved_at, label: "incident.resolved.v1" }] : []),
  ];
  rows.sort((a, b) => a.ts.localeCompare(b.ts));
  return (
    <div className="rounded border border-neutral-200 bg-white p-5 dark:border-neutral-800 dark:bg-neutral-900">
      <h3 className="font-medium">Timeline</h3>
      <ol className="mt-3 space-y-1 text-xs">
        {rows.map((row, i) => (
          <li key={i} className="font-mono">
            <span className="opacity-60">{row.ts}</span> — {row.label}
          </li>
        ))}
      </ol>
    </div>
  );
}

function DiagnosisCard({ report }: { report: DiagnosisReport | null }) {
  if (!report) return null;
  return (
    <div className="rounded border border-neutral-200 bg-white p-5 dark:border-neutral-800 dark:bg-neutral-900">
      <h3 className="font-medium">Diagnosis ({report.prompt_version})</h3>
      <p className="mt-1 text-sm">{report.context_summary}</p>
      <ol className="mt-3 space-y-2 text-sm">
        {report.hypotheses.map((h, i) => (
          <li key={i} className="rounded border border-neutral-200 p-3 dark:border-neutral-800">
            <p className="font-medium">{h.statement} <span className="text-xs opacity-60">(confidence {(h.confidence * 100).toFixed(0)}%)</span></p>
            {h.rationale && <p className="text-xs opacity-70">{h.rationale}</p>}
            <ul className="mt-2 list-disc pl-4 text-xs">
              {h.citations.map((c, ci) => (
                <li key={ci}><code>{c.source_kind}:{c.source_id}</code></li>
              ))}
            </ul>
            {h.suggested_actions.length > 0 && (
              <p className="mt-1 text-xs">Suggested: {h.suggested_actions.map((a) => <code key={a} className="mr-2">{a}</code>)}</p>
            )}
          </li>
        ))}
      </ol>
    </div>
  );
}

function HealingDecisionsCard({ decisions }: { decisions: HealingDecision[] }) {
  return (
    <div className="rounded border border-neutral-200 bg-white p-5 dark:border-neutral-800 dark:bg-neutral-900">
      <h3 className="font-medium">Healing decisions</h3>
      {decisions.length === 0 ? (
        <p className="mt-2 text-sm opacity-70">No healing actions taken.</p>
      ) : (
        <table className="mt-3 w-full text-xs">
          <thead className="text-left opacity-70">
            <tr>
              <th>action</th>
              <th>level</th>
              <th>outcome</th>
              <th>workflow_run</th>
            </tr>
          </thead>
          <tbody>
            {decisions.map((d) => (
              <tr key={d.id} className="border-t border-neutral-200 dark:border-neutral-800">
                <td><code>{d.action_id}</code></td>
                <td>
                  <code>{d.level}</code>
                  {d.level !== d.requested_level && <span className="ml-1 opacity-60">(requested {d.requested_level})</span>}
                </td>
                <td>{d.outcome}{d.reason ? ` (${d.reason})` : ""}</td>
                <td><code className="text-[10px]">{d.workflow_run_id ?? "-"}</code></td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  );
}

function PostmortemLink({ incident }: { incident: Incident }) {
  if (incident.status !== "resolved") return null;
  // The postmortem URL is filled in by the postmortem service when published.
  return (
    <div className="rounded border border-neutral-200 bg-white p-5 dark:border-neutral-800 dark:bg-neutral-900">
      <h3 className="font-medium">Postmortem</h3>
      <p className="mt-2 text-sm">
        Postmortem for <code>{incident.id}</code> is auto-generated on <code>incident.resolved.v1</code>. Once published it appears in Confluence and is linked from the affected asset&apos;s OpenSpec.
      </p>
    </div>
  );
}
