import { authOptions } from "@/auth";
import { getServerSession } from "next-auth";
import { redirect } from "next/navigation";
import { PageHead } from "@/components/page/PageHead";

type Suggestion = { kind: string; title: string; detail: string; openspec_ref?: string };
type Proposal = {
  id: string;
  incident_id: string;
  tenant_id: string;
  workspace_id: string;
  asset_id: string;
  postmortem_url: string;
  source: string;
  skill_version: string;
  status: "draft" | "inbox" | "accepted" | "rejected" | "converted";
  title: string;
  why: string;
  suggestions: Suggestion[];
  openspec_change_id?: string;
  created_at: string;
  updated_at: string;
  synthetic: boolean;
};

type Stats = {
  total: number;
  inbox: number;
  accepted: number;
  rejected: number;
  converted: number;
};

const evolutionUrl = () => process.env.EVOLUTION_URL ?? "http://localhost:8103";

async function getToken() {
  const session = await getServerSession(authOptions);
  if (!session) redirect("/api/auth/signin");
  return (session as { accessToken?: string }).accessToken;
}

async function fetchProposals(token?: string): Promise<Proposal[]> {
  const response = await fetch(`${evolutionUrl()}/v1/evolution/proposals?status=inbox`, {
    headers: token ? { authorization: `Bearer ${token}` } : {},
    cache: "no-store",
  });
  if (!response.ok) return [];
  return (await response.json()) as Proposal[];
}

async function fetchStats(token?: string): Promise<Stats> {
  const response = await fetch(`${evolutionUrl()}/v1/evolution/stats`, {
    headers: token ? { authorization: `Bearer ${token}` } : {},
    cache: "no-store",
  });
  if (!response.ok) return { total: 0, inbox: 0, accepted: 0, rejected: 0, converted: 0 };
  return (await response.json()) as Stats;
}

async function reviewProposal(formData: FormData) {
  "use server";
  const token = await getToken();
  const id = String(formData.get("proposal_id"));
  const approved = formData.get("approved") === "true";
  const reviewer = String(formData.get("reviewer") ?? "portal-user");
  const comment = String(formData.get("comment") ?? "");
  await fetch(`${evolutionUrl()}/v1/evolution/proposals/${encodeURIComponent(id)}/review`, {
    method: "POST",
    headers: { "content-type": "application/json", ...(token ? { authorization: `Bearer ${token}` } : {}) },
    body: JSON.stringify({ approved, reviewer, comment }),
  });
  redirect("/evolution");
}

export default async function EvolutionInboxPage() {
  const token = await getToken();
  const [proposals, stats] = await Promise.all([fetchProposals(token), fetchStats(token)]);

  return (
    <>
      <PageHead
        eyebrow="Observability · Evolution"
        title="Evolution"
        titleEm="inbox"
        sub="OpenSpec change proposals derived from postmortems by the autonomous loop. Each requires human review before becoming a normal change."
      />

      <div className="grid gap-3 sm:grid-cols-4" data-testid="evolution-stats">
        <Stat label="Total" value={stats.total} />
        <Stat label="Inbox" value={stats.inbox} />
        <Stat label="Converted" value={stats.converted} />
        <Stat label="Rejected" value={stats.rejected} />
      </div>

      <ul className="space-y-4" data-testid="evolution-proposals">
        {proposals.map((p) => (
          <li key={p.id} className="space-y-3 rounded border border-neutral-200 bg-white p-5 dark:border-neutral-800 dark:bg-neutral-900" data-testid={`proposal-${p.id}`}>
            <header className="flex items-start justify-between">
              <div>
                <h3 className="font-medium">{p.title}</h3>
                <p className="text-xs opacity-60">incident: <code>{p.incident_id}</code> · asset: <code>{p.asset_id}</code></p>
              </div>
              <div className="flex gap-2">
                <Badge color="purple">autonomous-loop</Badge>
                {p.synthetic && <Badge color="amber">synthetic</Badge>}
                <Badge color="neutral">{p.skill_version}</Badge>
              </div>
            </header>
            <p className="text-sm">{p.why}</p>
            <details className="text-sm">
              <summary className="cursor-pointer">Suggestions ({p.suggestions.length})</summary>
              <ul className="mt-2 space-y-2">
                {p.suggestions.map((s, i) => (
                  <li key={i} className="rounded border border-neutral-200 p-3 dark:border-neutral-800">
                    <p className="font-medium">{s.title} <code className="ml-2 text-xs opacity-60">{s.kind}</code></p>
                    <p className="text-xs opacity-80">{s.detail}</p>
                  </li>
                ))}
              </ul>
            </details>
            <div className="flex gap-2 text-sm">
              <form action={reviewProposal} className="flex flex-1 gap-2">
                <input type="hidden" name="proposal_id" value={p.id} />
                <input type="hidden" name="approved" value="true" />
                <input name="comment" placeholder="approval comment (optional)" className="flex-1 rounded border border-neutral-300 bg-transparent px-3 py-2 text-xs dark:border-neutral-700" />
                <button className="rounded bg-emerald-600 px-3 py-2 text-xs font-medium text-white">Approve & convert</button>
              </form>
              <form action={reviewProposal}>
                <input type="hidden" name="proposal_id" value={p.id} />
                <input type="hidden" name="approved" value="false" />
                <button className="rounded border border-red-400 px-3 py-2 text-xs font-medium text-red-700">Reject</button>
              </form>
            </div>
          </li>
        ))}
        {proposals.length === 0 && <p className="rounded border border-dashed border-neutral-300 p-6 text-sm opacity-70 dark:border-neutral-800">No proposals waiting for review.</p>}
      </ul>
    </>
  );
}

function Stat({ label, value }: { label: string; value: number }) {
  return (
    <div className="rounded border border-neutral-200 bg-white p-4 dark:border-neutral-800 dark:bg-neutral-900" data-testid={`stat-${label.toLowerCase()}`}>
      <p className="text-xs uppercase opacity-60">{label}</p>
      <p className="text-2xl font-semibold">{value}</p>
    </div>
  );
}

function Badge({ color, children }: { color: "purple" | "amber" | "neutral"; children: React.ReactNode }) {
  const cls = color === "purple"
    ? "bg-purple-200 text-purple-900"
    : color === "amber"
      ? "bg-amber-200 text-amber-900"
      : "bg-neutral-200 text-neutral-900";
  return <span className={`inline-flex items-center rounded px-1.5 py-0.5 text-[10px] font-semibold uppercase ${cls}`}>{children}</span>;
}
