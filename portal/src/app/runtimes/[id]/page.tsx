// Runtime detail page. Shows the runtime metadata plus the latest verification
// report (per task 2.13 of platform-gaps-closure). When the runtime-registry
// API is unreachable this falls back to a placeholder so the route still renders.

type VerifyCheck = {
  name: string;
  status: "pass" | "fail" | "warn" | "skip";
  evidence?: string;
  remediation?: string;
};

type VerifyReport = {
  id: string;
  workspace_id: string;
  runtime_id: string;
  type: string;
  mode: string;
  principal?: string;
  started_at: string;
  ended_at: string;
  status: VerifyCheck["status"];
  checks: VerifyCheck[];
};

type Runtime = {
  id: string;
  name: string;
  type: string;
  mode: string;
  status: string;
};

async function fetchRuntime(id: string): Promise<{ runtime: Runtime | null; latest: VerifyReport | null; error?: string }> {
  const base = process.env.RUNTIME_REGISTRY_URL ?? "http://localhost:8110";
  try {
    const rRes = await fetch(`${base}/v1/runtimes/${id}`, { cache: "no-store" });
    if (!rRes.ok) return { runtime: null, latest: null, error: `runtime-registry ${rRes.status}` };
    const runtime = (await rRes.json()) as Runtime;
    const vRes = await fetch(`${base}/v1/runtimes/${id}/verifications`, { cache: "no-store" });
    let latest: VerifyReport | null = null;
    if (vRes.ok) {
      const body = (await vRes.json()) as { verifications: VerifyReport[] };
      if (body.verifications && body.verifications.length > 0) {
        latest = body.verifications[body.verifications.length - 1];
      }
    }
    return { runtime, latest };
  } catch (e: any) {
    return { runtime: null, latest: null, error: e.message };
  }
}

const STATUS_BADGE: Record<VerifyCheck["status"], string> = {
  pass: "bg-emerald-100 text-emerald-800 dark:bg-emerald-950 dark:text-emerald-300",
  fail: "bg-red-100 text-red-800 dark:bg-red-950 dark:text-red-300",
  warn: "bg-amber-100 text-amber-800 dark:bg-amber-950 dark:text-amber-300",
  skip: "bg-neutral-100 text-neutral-700 dark:bg-neutral-900 dark:text-neutral-300",
};

export default async function RuntimeDetailPage({ params }: { params: { id: string } }) {
  const { runtime, latest, error } = await fetchRuntime(params.id);

  return (
    <section className="space-y-6">
      <div>
        <p className="text-sm font-medium uppercase tracking-wide text-neutral-500">Runtime</p>
        <h2 className="text-2xl font-semibold">{runtime?.name ?? params.id}</h2>
        <p className="text-xs text-neutral-500">{params.id}</p>
      </div>

      {error && (
        <div className="rounded border border-amber-300 bg-amber-50 p-3 text-sm text-amber-900 dark:border-amber-800 dark:bg-amber-950 dark:text-amber-200">
          Could not reach runtime-registry: {error}
        </div>
      )}

      {runtime && (
        <dl className="grid grid-cols-2 gap-3 rounded border border-neutral-200 bg-white p-4 text-sm md:grid-cols-4 dark:border-neutral-800 dark:bg-neutral-900">
          <div><dt className="text-xs text-neutral-500">Type</dt><dd>{runtime.type}</dd></div>
          <div><dt className="text-xs text-neutral-500">Mode</dt><dd>{runtime.mode}</dd></div>
          <div><dt className="text-xs text-neutral-500">Status</dt><dd>{runtime.status}</dd></div>
        </dl>
      )}

      <div>
        <div className="mb-2 flex items-baseline justify-between">
          <h3 className="text-lg font-semibold">Latest verification</h3>
          {latest && (
            <span className={`rounded-full px-2 py-1 text-xs ${STATUS_BADGE[latest.status]}`}>
              {latest.status.toUpperCase()}
            </span>
          )}
        </div>

        {!latest && (
          <p className="text-sm text-neutral-600 dark:text-neutral-400">
            No verification report yet. Run <code className="rounded bg-neutral-100 px-1 dark:bg-neutral-800">make verify-runtime RUNTIME={params.id}</code> to produce one.
          </p>
        )}

        {latest && (
          <div className="overflow-hidden rounded border border-neutral-200 dark:border-neutral-800">
            <table className="w-full text-sm">
              <thead className="bg-neutral-100 text-left text-xs uppercase tracking-wide text-neutral-500 dark:bg-neutral-900">
                <tr>
                  <th className="px-3 py-2">Check</th>
                  <th className="px-3 py-2">Status</th>
                  <th className="px-3 py-2">Evidence</th>
                  <th className="px-3 py-2">Remediation</th>
                </tr>
              </thead>
              <tbody>
                {latest.checks.map((c) => (
                  <tr key={c.name} className="border-t border-neutral-100 dark:border-neutral-800">
                    <td className="px-3 py-2 font-mono text-xs">{c.name}</td>
                    <td className="px-3 py-2">
                      <span className={`rounded-full px-2 py-1 text-xs ${STATUS_BADGE[c.status]}`}>{c.status}</span>
                    </td>
                    <td className="px-3 py-2 text-neutral-700 dark:text-neutral-300">{c.evidence ?? "—"}</td>
                    <td className="px-3 py-2 text-neutral-700 dark:text-neutral-300">{c.remediation ?? "—"}</td>
                  </tr>
                ))}
              </tbody>
            </table>
            <div className="border-t border-neutral-100 bg-neutral-50 px-3 py-2 text-xs text-neutral-600 dark:border-neutral-800 dark:bg-neutral-900 dark:text-neutral-400">
              Verified by <code>{latest.principal ?? "(unknown)"}</code> at {new Date(latest.ended_at).toISOString()}
            </div>
          </div>
        )}
      </div>
    </section>
  );
}
