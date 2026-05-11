import { authOptions } from "@/auth";
import { getServerSession } from "next-auth";
import { redirect } from "next/navigation";

type SpecificationDocument = {
  openspec_id: string;
  workspace_id: string;
  title: string;
  business_intent: string;
  problem_statement: string;
  stakeholders: string[];
  success_metrics: string[];
  requirements: { functional: string[]; non_functional: string[] };
  constraints: string[];
  autonomy_policy: { default_mode: string; approvals_required: string[] };
  linked_artifacts: { kind: string; ref: string; namespace?: string; direction: string; metadata?: Record<string, unknown> }[];
  decision_log: { actor: string; decision: string; rationale: string; timestamp: string; correlation_id?: string }[];
  audit: { created_by: string; created_at: string; updated_by?: string; updated_at?: string };
  version: number;
  openspec_artifacts?: { change_id: string; root: string; files: string[] } | null;
};

type SearchParams = {
  workspace_id?: string;
  openspec_id?: string;
  base_version?: string;
  compare_version?: string;
  saved?: string;
};

const openspecUrl = () => process.env.OPENSPEC_URL ?? "http://localhost:8083";

async function getToken() {
  const session = await getServerSession(authOptions);
  if (!session) redirect("/api/auth/signin");
  return (session as { accessToken?: string }).accessToken;
}

async function fetchSpecifications(workspaceId: string, token?: string) {
  const response = await fetch(`${openspecUrl()}/v1/openspecs?workspace_id=${encodeURIComponent(workspaceId)}`, {
    headers: token ? { authorization: `Bearer ${token}` } : {},
    cache: "no-store",
  });
  if (!response.ok) throw new Error(`specification-service ${response.status}: ${await response.text()}`);
  return ((await response.json()) as { openspecs: SpecificationDocument[] }).openspecs;
}

async function fetchSpecification(openspecId: string, token?: string) {
  const response = await fetch(`${openspecUrl()}/v1/openspecs/${encodeURIComponent(openspecId)}`, {
    headers: token ? { authorization: `Bearer ${token}` } : {},
    cache: "no-store",
  });
  if (!response.ok) throw new Error(`specification-service ${response.status}: ${await response.text()}`);
  return (await response.json()) as SpecificationDocument;
}

async function fetchVersions(openspecId: string, token?: string) {
  const response = await fetch(`${openspecUrl()}/v1/openspecs/${encodeURIComponent(openspecId)}/versions`, {
    headers: token ? { authorization: `Bearer ${token}` } : {},
    cache: "no-store",
  });
  if (!response.ok) return [];
  return ((await response.json()) as { versions: number[] }).versions;
}

async function fetchVersion(openspecId: string, version: number, token?: string) {
  const response = await fetch(
    `${openspecUrl()}/v1/openspecs/${encodeURIComponent(openspecId)}/versions/${version}`,
    { headers: token ? { authorization: `Bearer ${token}` } : {}, cache: "no-store" },
  );
  if (!response.ok) return null;
  return (await response.json()) as SpecificationDocument;
}

async function createSpecification(formData: FormData) {
  "use server";
  const token = await getToken();
  const workspaceId = required(formData, "workspace_id");
  const payload = {
    openspec_id: optional(formData, "openspec_id") || undefined,
    workspace_id: workspaceId,
    title: required(formData, "title"),
    business_intent: required(formData, "business_intent"),
    problem_statement: required(formData, "problem_statement"),
    stakeholders: lines(formData, "stakeholders"),
    success_metrics: lines(formData, "success_metrics"),
    requirements: {
      functional: lines(formData, "requirements_functional"),
      non_functional: lines(formData, "requirements_non_functional"),
    },
    constraints: lines(formData, "constraints"),
    created_by: "portal",
  };
  const response = await fetch(`${openspecUrl()}/v1/openspecs`, {
    method: "POST",
    headers: { "content-type": "application/json", ...(token ? { authorization: `Bearer ${token}` } : {}) },
    body: JSON.stringify(payload),
  });
  if (!response.ok) throw new Error(`specification-service ${response.status}: ${await response.text()}`);
  const created = (await response.json()) as SpecificationDocument;
  redirect(`/openspecs?workspace_id=${workspaceId}&openspec_id=${created.openspec_id}&saved=1`);
}

async function updateSpecification(formData: FormData) {
  "use server";
  const token = await getToken();
  const workspaceId = required(formData, "workspace_id");
  const openspecId = required(formData, "openspec_id");
  const payload = {
    title: required(formData, "title"),
    business_intent: required(formData, "business_intent"),
    problem_statement: required(formData, "problem_statement"),
    stakeholders: lines(formData, "stakeholders"),
    success_metrics: lines(formData, "success_metrics"),
    requirements: {
      functional: lines(formData, "requirements_functional"),
      non_functional: lines(formData, "requirements_non_functional"),
    },
    constraints: lines(formData, "constraints"),
    updated_by: "portal",
  };
  const response = await fetch(`${openspecUrl()}/v1/openspecs/${encodeURIComponent(openspecId)}`, {
    method: "PATCH",
    headers: { "content-type": "application/json", ...(token ? { authorization: `Bearer ${token}` } : {}) },
    body: JSON.stringify(payload),
  });
  if (!response.ok) throw new Error(`specification-service ${response.status}: ${await response.text()}`);
  redirect(`/openspecs?workspace_id=${workspaceId}&openspec_id=${openspecId}&saved=1`);
}

export default async function SpecificationsPage({ searchParams }: { searchParams: SearchParams }) {
  const token = await getToken();
  const workspaceId = searchParams.workspace_id?.trim() ?? "";
  let specs: SpecificationDocument[] = [];
  let selected: SpecificationDocument | null = null;
  let versions: number[] = [];
  let base: SpecificationDocument | null = null;
  let compare: SpecificationDocument | null = null;
  let error: string | null = null;

  if (workspaceId) {
    try {
      specs = await fetchSpecifications(workspaceId, token);
      const selectedId = searchParams.openspec_id ?? specs[0]?.openspec_id;
      if (selectedId) {
        selected = await fetchSpecification(selectedId, token);
        versions = await fetchVersions(selectedId, token);
        const compareVersion = Number(searchParams.compare_version ?? selected.version);
        const baseVersion = Number(searchParams.base_version ?? Math.max(1, compareVersion - 1));
        base = await fetchVersion(selectedId, baseVersion, token);
        compare = await fetchVersion(selectedId, compareVersion, token);
      }
    } catch (e) {
      error = e instanceof Error ? e.message : "failed to load specifications";
    }
  }

  return (
    <section className="space-y-6">
      <div className="flex flex-col gap-3 md:flex-row md:items-end md:justify-between">
        <div>
          <h2 className="text-2xl font-semibold">Specifications</h2>
          <p className="mt-1 text-sm opacity-70">Structured delivery specifications backed by OpenSpec artifacts, links, and version history.</p>
        </div>
        <form className="flex gap-2" method="get">
          <input name="workspace_id" defaultValue={workspaceId} placeholder="Workspace ID" className="min-w-0 rounded border border-neutral-300 bg-transparent px-3 py-2 text-sm dark:border-neutral-700" />
          <button className="rounded bg-neutral-900 px-4 py-2 text-sm text-white dark:bg-neutral-100 dark:text-neutral-900">Load</button>
        </form>
      </div>

      {searchParams.saved && <p className="rounded border border-green-300 bg-green-50 p-3 text-sm text-green-800 dark:border-green-800 dark:bg-green-950 dark:text-green-200">Specification saved.</p>}
      {error && <p className="rounded border border-red-300 bg-red-50 p-3 text-sm text-red-800 dark:border-red-800 dark:bg-red-950 dark:text-red-200">{error}</p>}

      <div className="grid gap-5 xl:grid-cols-[320px_1fr]">
        <aside className="space-y-4 rounded border border-neutral-200 bg-white p-4 dark:border-neutral-800 dark:bg-neutral-900">
          <h3 className="font-medium">Workspace specifications</h3>
          <div className="grid gap-2 text-sm">
            {specs.map((spec) => (
              <a key={spec.openspec_id} href={`/openspecs?workspace_id=${workspaceId}&openspec_id=${spec.openspec_id}`} className="rounded border border-neutral-200 px-3 py-2 hover:bg-neutral-50 dark:border-neutral-800 dark:hover:bg-neutral-800">
                <span className="block font-medium">{spec.title}</span>
                <span className="text-xs opacity-60">{spec.openspec_id} · v{spec.version}</span>
              </a>
            ))}
            {workspaceId && specs.length === 0 && !error && <p className="opacity-70">No specifications indexed for this Workspace.</p>}
          </div>
        </aside>

        <div className="grid gap-5 lg:grid-cols-2">
          <SpecificationForm workspaceId={workspaceId} selected={selected} />
          <div className="space-y-5">
            {selected ? (
              <>
                <MarkdownPreview spec={selected} />
                <OpenSpecArtifacts spec={selected} />
                <LinkedArtifacts spec={selected} />
                <VersionDiff spec={selected} versions={versions} base={base} compare={compare} workspaceId={workspaceId} />
              </>
            ) : (
              <div className="rounded border border-dashed border-neutral-300 p-6 text-sm opacity-70 dark:border-neutral-800">Load a Workspace and select a specification to preview versions and links.</div>
            )}
          </div>
        </div>
      </div>
    </section>
  );
}

function SpecificationForm({ workspaceId, selected }: { workspaceId: string; selected: SpecificationDocument | null }) {
  const action = selected ? updateSpecification : createSpecification;
  return (
    <form action={action} className="space-y-4 rounded border border-neutral-200 bg-white p-5 dark:border-neutral-800 dark:bg-neutral-900">
      <div>
        <h3 className="font-medium">{selected ? "Edit specification" : "Create specification"}</h3>
        <p className="mt-1 text-xs opacity-60">One item per line for lists. Functional requirements are required.</p>
      </div>
      <input type="hidden" name="workspace_id" value={workspaceId} />
      <label className="grid gap-1 text-sm">
        <span className="font-medium">Specification ID</span>
        <input name="openspec_id" defaultValue={selected?.openspec_id ?? ""} readOnly={Boolean(selected)} className="rounded border border-neutral-300 bg-transparent px-3 py-2 dark:border-neutral-700" />
      </label>
      <Field name="title" label="Title" defaultValue={selected?.title} required />
      <TextArea name="business_intent" label="Business intent" defaultValue={selected?.business_intent} required />
      <TextArea name="problem_statement" label="Problem statement" defaultValue={selected?.problem_statement} required />
      <TextArea name="requirements_functional" label="Functional requirements" defaultValue={selected?.requirements.functional.join("\n")} required />
      <TextArea name="requirements_non_functional" label="Non-functional requirements" defaultValue={selected?.requirements.non_functional.join("\n")} />
      <TextArea name="stakeholders" label="Stakeholders" defaultValue={selected?.stakeholders.join("\n")} />
      <TextArea name="success_metrics" label="Success metrics" defaultValue={selected?.success_metrics.join("\n")} />
      <TextArea name="constraints" label="Constraints" defaultValue={selected?.constraints.join("\n")} />
      <button disabled={!workspaceId} className="rounded bg-neutral-900 px-4 py-2 text-sm font-medium text-white disabled:opacity-40 dark:bg-neutral-100 dark:text-neutral-900">{selected ? "Save changes" : "Create specification"}</button>
    </form>
  );
}

function Field({ name, label, defaultValue, required = false }: { name: string; label: string; defaultValue?: string; required?: boolean }) {
  return (
    <label className="grid gap-1 text-sm">
      <span className="font-medium">{label}</span>
      <input name={name} required={required} defaultValue={defaultValue} className="rounded border border-neutral-300 bg-transparent px-3 py-2 dark:border-neutral-700" />
    </label>
  );
}

function TextArea({ name, label, defaultValue, required = false }: { name: string; label: string; defaultValue?: string; required?: boolean }) {
  return (
    <label className="grid gap-1 text-sm">
      <span className="font-medium">{label}</span>
      <textarea name={name} required={required} defaultValue={defaultValue} rows={3} className="rounded border border-neutral-300 bg-transparent px-3 py-2 dark:border-neutral-700" />
    </label>
  );
}

function MarkdownPreview({ spec }: { spec: SpecificationDocument }) {
  return (
    <div className="rounded border border-neutral-200 bg-white p-5 dark:border-neutral-800 dark:bg-neutral-900">
      <h3 className="font-medium">Markdown preview</h3>
      <pre className="mt-3 max-h-[520px] overflow-auto whitespace-pre-wrap rounded bg-neutral-950 p-4 text-xs text-neutral-100">{toMarkdown(spec)}</pre>
    </div>
  );
}

function OpenSpecArtifacts({ spec }: { spec: SpecificationDocument }) {
  const artifacts = spec.openspec_artifacts;
  return (
    <div className="rounded border border-neutral-200 bg-white p-5 dark:border-neutral-800 dark:bg-neutral-900">
      <h3 className="font-medium">OpenSpec backing artifacts</h3>
      {artifacts ? (
        <div className="mt-3 space-y-2 text-sm">
          <p>
            Change ID <code>{artifacts.change_id}</code>
          </p>
          <p className="break-all text-xs opacity-70">{artifacts.root}</p>
          <div className="grid gap-2">
            {artifacts.files.map((file) => (
              <code key={file} className="rounded bg-neutral-100 px-3 py-2 text-xs dark:bg-neutral-950">{file}</code>
            ))}
          </div>
        </div>
      ) : (
        <p className="mt-3 rounded border border-dashed border-neutral-300 p-4 text-sm opacity-70 dark:border-neutral-800">
          The specification service did not return OpenSpec artifacts for this record.
        </p>
      )}
    </div>
  );
}

function LinkedArtifacts({ spec }: { spec: SpecificationDocument }) {
  const namespaces = ["jira", "confluence", "test", "slo", "cost", "incident"];
  return (
    <div className="rounded border border-neutral-200 bg-white p-5 dark:border-neutral-800 dark:bg-neutral-900">
      <h3 className="font-medium">Linked artifacts</h3>
      <div className="mt-3 grid gap-3 text-sm md:grid-cols-2">
        {namespaces.map((namespace) => {
          const links = spec.linked_artifacts.filter((link) => (link.namespace ?? link.kind).replace(":", "") === namespace);
          return (
            <section key={namespace} className="rounded border border-neutral-200 p-3 dark:border-neutral-800">
              <h4 className="font-medium capitalize">{namespace === "slo" ? "SLOs" : namespace}</h4>
              <div className="mt-2 grid gap-2">
                {links.map((link) => (
                  <a key={`${link.kind}:${link.ref}`} href={link.ref.startsWith("http") ? link.ref : "#"} className="rounded bg-neutral-50 px-3 py-2 dark:bg-neutral-950">
                    <span className="font-medium">{link.kind}</span> <code>{link.ref}</code>
                    <span className="ml-2 text-xs opacity-60">{link.direction}</span>
                  </a>
                ))}
                {links.length === 0 && <p className="text-xs opacity-60">No {namespace} links.</p>}
              </div>
            </section>
          );
        })}
        {spec.linked_artifacts.length === 0 && <p className="opacity-70">No linked artifacts yet.</p>}
      </div>
    </div>
  );
}

function VersionDiff({ spec, versions, base, compare, workspaceId }: { spec: SpecificationDocument; versions: number[]; base: SpecificationDocument | null; compare: SpecificationDocument | null; workspaceId: string }) {
  const rows = base && compare ? diffLines(toMarkdown(base), toMarkdown(compare)) : [];
  return (
    <div className="rounded border border-neutral-200 bg-white p-5 dark:border-neutral-800 dark:bg-neutral-900">
      <h3 className="font-medium">Version diff</h3>
      <form className="mt-3 flex flex-wrap gap-2 text-sm" method="get">
        <input type="hidden" name="workspace_id" value={workspaceId} />
        <input type="hidden" name="openspec_id" value={spec.openspec_id} />
        <select name="base_version" defaultValue={base?.version} className="rounded border border-neutral-300 bg-transparent px-2 py-1 dark:border-neutral-700">
          {versions.map((version) => <option key={version} value={version}>base v{version}</option>)}
        </select>
        <select name="compare_version" defaultValue={compare?.version ?? spec.version} className="rounded border border-neutral-300 bg-transparent px-2 py-1 dark:border-neutral-700">
          {versions.map((version) => <option key={version} value={version}>compare v{version}</option>)}
        </select>
        <button className="rounded border border-neutral-300 px-3 py-1 dark:border-neutral-700">Diff</button>
      </form>
      <div className="mt-3 max-h-80 overflow-auto rounded bg-neutral-950 p-3 font-mono text-xs text-neutral-100">
        {rows.map((row, index) => <div key={index} className={row.kind === "add" ? "text-green-300" : row.kind === "remove" ? "text-red-300" : "text-neutral-400"}>{row.prefix} {row.text}</div>)}
        {rows.length === 0 && <p className="text-neutral-400">No diff available yet.</p>}
      </div>
    </div>
  );
}

function toMarkdown(spec: SpecificationDocument) {
  return [
    `# ${spec.title}`,
    "",
    `Specification: ${spec.openspec_id} · Workspace: ${spec.workspace_id} · Version: ${spec.version}`,
    "",
    "## Business Intent",
    spec.business_intent,
    "",
    "## Problem Statement",
    spec.problem_statement,
    "",
    "## Functional Requirements",
    ...spec.requirements.functional.map((item) => `- ${item}`),
    "",
    "## Non-Functional Requirements",
    ...spec.requirements.non_functional.map((item) => `- ${item}`),
    "",
    "## Constraints",
    ...spec.constraints.map((item) => `- ${item}`),
  ].join("\n");
}

function diffLines(left: string, right: string) {
  const leftLines = left.split("\n");
  const rightLines = right.split("\n");
  const rows: { kind: "same" | "add" | "remove"; prefix: string; text: string }[] = [];
  const max = Math.max(leftLines.length, rightLines.length);
  for (let i = 0; i < max; i += 1) {
    if (leftLines[i] === rightLines[i]) rows.push({ kind: "same", prefix: " ", text: leftLines[i] ?? "" });
    else {
      if (leftLines[i] !== undefined) rows.push({ kind: "remove", prefix: "-", text: leftLines[i] });
      if (rightLines[i] !== undefined) rows.push({ kind: "add", prefix: "+", text: rightLines[i] });
    }
  }
  return rows;
}

function required(formData: FormData, key: string) {
  const value = optional(formData, key);
  if (!value) throw new Error(`${key} is required`);
  return value;
}

function optional(formData: FormData, key: string) {
  return String(formData.get(key) ?? "").trim();
}

function lines(formData: FormData, key: string) {
  return optional(formData, key).split("\n").map((line) => line.trim()).filter(Boolean);
}
