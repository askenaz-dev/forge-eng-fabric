const findings = [
  { id: "drift-1", resource: "google_container_node_pool.main", field: "node_count", severity: "medium", status: "open" },
  { id: "drift-2", resource: "google_project_iam_binding.deploy", field: "members", severity: "high", status: "remediation proposed" },
];

export default function DriftPage() {
  return (
    <section className="space-y-5">
      <div>
        <p className="text-sm font-medium uppercase tracking-wide text-neutral-500">IaC Drift</p>
        <h2 className="text-2xl font-semibold">Terraform drift findings</h2>
        <p className="mt-1 text-sm text-neutral-600 dark:text-neutral-300">Review hourly findings, route high-severity changes, and ask Alfred to propose remediation PRs.</p>
      </div>

      <div className="grid gap-3">
        {findings.map((finding) => (
          <article key={finding.id} className="rounded border border-neutral-200 bg-white p-4 dark:border-neutral-800 dark:bg-neutral-900">
            <div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
              <div>
                <h3 className="font-semibold">{finding.resource}</h3>
                <p className="text-xs text-neutral-500">{finding.id} · field {finding.field}</p>
              </div>
              <div className="flex items-center gap-2">
                <span className="rounded-full bg-amber-50 px-2 py-1 text-xs text-amber-700 dark:bg-amber-950 dark:text-amber-200">{finding.severity}</span>
                <span className="text-sm text-neutral-500">{finding.status}</span>
                <button className="rounded bg-neutral-900 px-3 py-1.5 text-sm text-white dark:bg-neutral-100 dark:text-neutral-900">Propose PR</button>
              </div>
            </div>
          </article>
        ))}
      </div>
    </section>
  );
}
