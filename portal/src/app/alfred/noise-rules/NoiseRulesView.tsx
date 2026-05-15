"use client";

import { useCallback, useEffect, useState } from "react";

interface NoiseRule {
  id: string;
  fingerprint: string;
  description: string;
  proposed_by: string;
  proposed_at: string;
  approved_by?: string;
  status: string;
  pr_url?: string;
  evidence_sample_ids?: string[];
  expires_at?: string;
}

type Filter = "draft" | "active" | "promoted" | "revoked";

export function NoiseRulesView() {
  const [rules, setRules] = useState<NoiseRule[]>([]);
  const [filter, setFilter] = useState<Filter>("draft");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [acting, setActing] = useState<string | null>(null);

  const load = useCallback((status: Filter) => {
    setLoading(true);
    setError(null);
    fetch(`/api/alfred/noise-rules?status=${status}`)
      .then((r) => r.json())
      .then((d) => setRules(d.rules ?? []))
      .catch((e) => setError(e.message))
      .finally(() => setLoading(false));
  }, []);

  useEffect(() => { load(filter); }, [filter, load]);

  async function approve(id: string) {
    setActing(id);
    try {
      const res = await fetch(`/api/alfred/noise-rules/${id}/approve`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({}),
      });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error ?? "approve failed");
      load(filter);
    } catch (e) {
      alert((e as Error).message);
    } finally {
      setActing(null);
    }
  }

  async function revoke(id: string) {
    const reason = prompt("Revocation reason (optional):");
    if (reason === null) return; // cancelled
    setActing(id);
    try {
      const res = await fetch(`/api/alfred/noise-rules/${id}/revoke`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ reason }),
      });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error ?? "revoke failed");
      load(filter);
    } catch (e) {
      alert((e as Error).message);
    } finally {
      setActing(null);
    }
  }

  return (
    <div className="p-6 space-y-4">
      <div className="flex items-center justify-between gap-4 flex-wrap">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">Noise Rules</h1>
          <p className="text-sm text-muted-foreground mt-1">
            Proposed by Alfred to suppress known-noisy symptom fingerprints.
            Approve to open a policy PR; revoke to remove an active rule.
          </p>
        </div>
        <div className="flex gap-1">
          {(["draft", "active", "promoted", "revoked"] as Filter[]).map((f) => (
            <button
              key={f}
              onClick={() => setFilter(f)}
              className={`px-3 py-1 rounded text-xs font-medium border transition-colors ${
                filter === f
                  ? "bg-primary text-primary-foreground border-primary"
                  : "bg-background border-border text-muted-foreground hover:text-foreground"
              }`}
            >
              {f}
            </button>
          ))}
        </div>
      </div>

      {loading && (
        <div className="text-sm text-muted-foreground">Loading noise rules…</div>
      )}
      {error && (
        <div className="text-sm text-destructive">Error: {error}</div>
      )}

      {!loading && rules.length === 0 && !error && (
        <div className="rounded-md border border-dashed p-8 text-center text-muted-foreground text-sm">
          No <strong>{filter}</strong> noise rules.
          {filter === "draft" && " Alfred will propose rules here once symptoms repeat above the noise threshold."}
        </div>
      )}

      <div className="space-y-3">
        {rules.map((rule) => (
          <div key={rule.id} className="rounded-md border bg-card overflow-hidden">
            <div className="px-4 py-3 flex items-start gap-3">
              <div className="flex-1 min-w-0">
                <div className="flex items-center gap-2 flex-wrap">
                  <span className="font-mono text-xs text-muted-foreground">{rule.id.slice(0, 8)}…</span>
                  <StatusBadge status={rule.status} />
                  {rule.pr_url && (
                    <a
                      href={rule.pr_url}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="text-xs text-primary hover:underline font-mono"
                    >
                      PR ↗
                    </a>
                  )}
                </div>
                <p className="font-mono text-sm mt-1 break-all">{rule.fingerprint}</p>
                <p className="text-sm text-muted-foreground mt-0.5">{rule.description}</p>
              </div>
              {rule.status === "draft" && (
                <div className="flex gap-2 shrink-0">
                  <button
                    onClick={() => approve(rule.id)}
                    disabled={acting === rule.id}
                    className="px-3 py-1 rounded text-xs font-medium bg-green-600 text-white hover:bg-green-700 disabled:opacity-50"
                  >
                    Approve
                  </button>
                  <button
                    onClick={() => revoke(rule.id)}
                    disabled={acting === rule.id}
                    className="px-3 py-1 rounded text-xs font-medium bg-destructive text-destructive-foreground hover:opacity-90 disabled:opacity-50"
                  >
                    Revoke
                  </button>
                </div>
              )}
              {rule.status === "active" && (
                <button
                  onClick={() => revoke(rule.id)}
                  disabled={acting === rule.id}
                  className="px-3 py-1 rounded text-xs font-medium border border-destructive text-destructive hover:bg-destructive hover:text-destructive-foreground disabled:opacity-50 shrink-0"
                >
                  Revoke
                </button>
              )}
            </div>

            <div className="border-t px-4 py-2 bg-muted/30 flex flex-wrap gap-4 text-xs text-muted-foreground">
              <span>
                <span className="font-medium">Proposed by</span>{" "}
                <span className="font-mono">{rule.proposed_by}</span>
              </span>
              <span>
                <span className="font-medium">At</span>{" "}
                {new Date(rule.proposed_at).toLocaleString()}
              </span>
              {rule.approved_by && (
                <span>
                  <span className="font-medium">Approved by</span>{" "}
                  <span className="font-mono">{rule.approved_by}</span>
                </span>
              )}
              {rule.expires_at && (
                <span>
                  <span className="font-medium">Expires</span>{" "}
                  {new Date(rule.expires_at).toLocaleDateString()}
                </span>
              )}
              {rule.evidence_sample_ids && rule.evidence_sample_ids.length > 0 && (
                <EvidenceSamples ids={rule.evidence_sample_ids} />
              )}
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

function StatusBadge({ status }: { status: string }) {
  const colours: Record<string, string> = {
    draft: "bg-yellow-100 text-yellow-800",
    active: "bg-blue-100 text-blue-800",
    promoted: "bg-green-100 text-green-800",
    revoked: "bg-gray-100 text-gray-600",
  };
  return (
    <span
      className={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ${
        colours[status] ?? "bg-gray-100 text-gray-600"
      }`}
    >
      {status}
    </span>
  );
}

function EvidenceSamples({ ids }: { ids: string[] }) {
  const shown = ids.slice(0, 10);
  return (
    <span>
      <span className="font-medium">Evidence samples</span>{" "}
      <span className="font-mono">
        {shown.map((id) => id.slice(0, 8)).join(", ")}
        {ids.length > 10 && ` +${ids.length - 10} more`}
      </span>
    </span>
  );
}
