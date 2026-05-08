const runtimes = [
  { id: "rt-dev-1", name: "local-minikube", type: "minikube", mode: "byo", status: "ready", env: "dev" },
  { id: "rt-prod-gke", name: "pilot-prod", type: "gke", mode: "byo", status: "preflight_required", env: "prod" },
  { id: "rt-stage-cr", name: "stage-cloudrun", type: "cloudrun", mode: "provisioned", status: "ready", env: "stage" },
];

export default function RuntimesPage() {
  return (
    <section className="space-y-5">
      <div className="flex flex-col gap-3 md:flex-row md:items-end md:justify-between">
        <div>
          <p className="text-sm font-medium uppercase tracking-wide text-neutral-500">Runtimes</p>
          <h2 className="text-2xl font-semibold">Deploy targets</h2>
          <p className="mt-1 text-sm text-neutral-600 dark:text-neutral-300">Register BYO targets, run preflight, or track Forge-provisioned runtimes.</p>
        </div>
        <button className="rounded bg-neutral-900 px-4 py-2 text-sm font-medium text-white dark:bg-neutral-100 dark:text-neutral-900">Register BYO runtime</button>
      </div>

      <div className="grid gap-3 lg:grid-cols-3">
        {runtimes.map((runtime) => (
          <article key={runtime.id} className="rounded border border-neutral-200 bg-white p-4 shadow-sm dark:border-neutral-800 dark:bg-neutral-900">
            <div className="flex items-start justify-between gap-3">
              <div>
                <h3 className="font-semibold">{runtime.name}</h3>
                <p className="text-xs text-neutral-500">{runtime.id}</p>
              </div>
              <span className="rounded-full bg-neutral-100 px-2 py-1 text-xs dark:bg-neutral-800">{runtime.status}</span>
            </div>
            <dl className="mt-4 grid grid-cols-3 gap-2 text-sm">
              <div><dt className="text-xs text-neutral-500">Type</dt><dd>{runtime.type}</dd></div>
              <div><dt className="text-xs text-neutral-500">Mode</dt><dd>{runtime.mode}</dd></div>
              <div><dt className="text-xs text-neutral-500">Env</dt><dd>{runtime.env}</dd></div>
            </dl>
            <div className="mt-4 flex gap-2">
              <button className="rounded border border-neutral-300 px-3 py-1.5 text-sm dark:border-neutral-700">Run preflight</button>
              <button className="rounded border border-red-300 px-3 py-1.5 text-sm text-red-700 dark:border-red-800 dark:text-red-300">Delete</button>
            </div>
          </article>
        ))}
      </div>
    </section>
  );
}
