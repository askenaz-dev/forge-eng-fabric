"use client";

import { useEffect, useState } from "react";

interface AutonomousSession {
  session_id: string;
  symptom_id: string | null;
  fingerprint: string;
  trigger_source: string;
  actor: string;
  status: string;
  action: string | null;
  verification: string | null;
  audit_event_id: string | null;
  started_at: string;
}

export function AutonomousActivityView() {
  const [sessions, setSessions] = useState<AutonomousSession[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    fetch("/api/alfred/autonomous-sessions")
      .then((r) => {
        if (!r.ok) throw new Error(`HTTP ${r.status}`);
        return r.json();
      })
      .then((data) => setSessions(data.sessions ?? []))
      .catch((e) => setError(e.message))
      .finally(() => setLoading(false));
  }, []);

  if (loading) return <div className="p-6 text-muted-foreground">Loading autonomous activity…</div>;
  if (error) return <div className="p-6 text-destructive">Error: {error}</div>;

  return (
    <div className="p-6 space-y-4">
      <h1 className="text-2xl font-semibold tracking-tight">Autonomous Activity</h1>
      <p className="text-sm text-muted-foreground">
        Sessions spawned by Alfred autonomously in response to platform symptoms.
      </p>

      {sessions.length === 0 ? (
        <div className="rounded-md border border-dashed p-8 text-center text-muted-foreground text-sm">
          No autonomous sessions yet. Alfred will appear here once it starts acting on symptoms.
        </div>
      ) : (
        <div className="rounded-md border overflow-hidden">
          <table className="w-full text-sm">
            <thead className="bg-muted/50 text-muted-foreground">
              <tr>
                <th className="px-4 py-2 text-left font-medium">Session</th>
                <th className="px-4 py-2 text-left font-medium">Fingerprint</th>
                <th className="px-4 py-2 text-left font-medium">Status</th>
                <th className="px-4 py-2 text-left font-medium">Action</th>
                <th className="px-4 py-2 text-left font-medium">Verification</th>
                <th className="px-4 py-2 text-left font-medium">Audit</th>
                <th className="px-4 py-2 text-left font-medium">Started</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-border">
              {sessions.map((s) => (
                <tr key={s.session_id} className="hover:bg-muted/30 transition-colors">
                  <td className="px-4 py-2 font-mono text-xs truncate max-w-[120px]">
                    <a
                      href={`/alfred/sessions/${s.session_id}`}
                      className="text-primary hover:underline"
                    >
                      {s.session_id.slice(0, 8)}…
                    </a>
                  </td>
                  <td className="px-4 py-2 font-mono text-xs truncate max-w-[200px]" title={s.fingerprint}>
                    {s.fingerprint}
                  </td>
                  <td className="px-4 py-2">
                    <StatusBadge status={s.status} />
                  </td>
                  <td className="px-4 py-2 text-xs">{s.action ?? "—"}</td>
                  <td className="px-4 py-2 text-xs">{s.verification ?? "—"}</td>
                  <td className="px-4 py-2 text-xs">
                    {s.audit_event_id ? (
                      <a
                        href={`/admin/audit?id=${s.audit_event_id}`}
                        className="text-primary hover:underline font-mono"
                      >
                        {s.audit_event_id.slice(0, 8)}…
                      </a>
                    ) : (
                      "—"
                    )}
                  </td>
                  <td className="px-4 py-2 text-xs text-muted-foreground">
                    {new Date(s.started_at).toLocaleString()}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}

function StatusBadge({ status }: { status: string }) {
  const colours: Record<string, string> = {
    planning: "bg-blue-100 text-blue-800",
    running: "bg-yellow-100 text-yellow-800",
    completed: "bg-green-100 text-green-800",
    failed: "bg-red-100 text-red-800",
    paused_for_approval: "bg-purple-100 text-purple-800",
    aborted: "bg-gray-100 text-gray-600",
  };
  const cls = colours[status] ?? "bg-gray-100 text-gray-600";
  return (
    <span className={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ${cls}`}>
      {status}
    </span>
  );
}
