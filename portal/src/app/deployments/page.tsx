const deployments = [
  { id: "dep-1007", asset: "app-foo", env: "prod", status: "verified", revision: "rev-1007", runtime: "rt-prod-gke", image: "sha256:abc", stage: "notify" },
  { id: "dep-1006", asset: "app-foo", env: "prod", status: "rolled_back", revision: "rev-1006", runtime: "rt-prod-gke", image: "sha256:def", stage: "rollback" },
  { id: "dep-42", asset: "worker-bar", env: "dev", status: "running", revision: "rev-42", runtime: "rt-dev-1", image: "sha256:999", stage: "verify" },
];

const stages = ["preflight", "policy", "image-verify", "render", "apply", "verify", "notify"];

export default function DeploymentsPage() {
  return (
    <section className="space-y-5">
      <div>
        <p className="text-sm font-medium uppercase tracking-wide text-neutral-500">Deployments</p>
        <h2 className="text-2xl font-semibold">Release history and live status</h2>
        <p className="mt-1 text-sm text-neutral-600 dark:text-neutral-300">Inspect stages, verify image status, and rollback to the previous revision with an audited reason.</p>
      </div>

      <div className="overflow-hidden rounded border border-neutral-200 bg-white dark:border-neutral-800 dark:bg-neutral-900">
        {deployments.map((deployment) => (
          <div key={deployment.id} className="border-b border-neutral-100 p-4 last:border-b-0 dark:border-neutral-800">
            <div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
              <div>
                <h3 className="font-semibold">{deployment.asset} <span className="text-sm font-normal text-neutral-500">{deployment.env}</span></h3>
                <p className="text-xs text-neutral-500">{deployment.id} · {deployment.revision} · {deployment.runtime} · {deployment.image}</p>
              </div>
              <div className="flex items-center gap-2">
                <span className="rounded-full bg-emerald-50 px-2 py-1 text-xs text-emerald-700 dark:bg-emerald-950 dark:text-emerald-200">{deployment.status}</span>
                <button className="rounded border border-neutral-300 px-3 py-1.5 text-sm dark:border-neutral-700">Rollback to previous</button>
              </div>
            </div>
            <div className="mt-4 grid gap-2 md:grid-cols-7">
              {stages.map((stage) => (
                <div key={stage} className={`rounded px-2 py-2 text-xs ${stage === deployment.stage ? "bg-neutral-900 text-white dark:bg-neutral-100 dark:text-neutral-900" : "bg-neutral-100 dark:bg-neutral-800"}`}>
                  {stage}
                </div>
              ))}
            </div>
            <details className="mt-3 text-sm">
              <summary className="cursor-pointer text-neutral-600 dark:text-neutral-300">Rollback confirmation</summary>
              <div className="mt-2 rounded bg-neutral-50 p-3 dark:bg-neutral-950">
                <label className="block text-xs font-medium text-neutral-500">Reason</label>
                <textarea className="mt-1 min-h-20 w-full rounded border border-neutral-300 bg-white p-2 text-sm dark:border-neutral-700 dark:bg-neutral-900" placeholder="Explain customer impact and recovery intent" />
                <button className="mt-2 rounded bg-red-700 px-3 py-1.5 text-sm text-white">Confirm rollback</button>
              </div>
            </details>
          </div>
        ))}
      </div>
    </section>
  );
}
