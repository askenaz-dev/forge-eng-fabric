"use client";

import type { OnboardingRequest, RepoTemplate } from "@/lib/onboarding-types";
import { useState } from "react";

type Props = { templates: RepoTemplate[] };

const steps = ["template", "params", "preview", "confirm"] as const;

type FormState = {
  workspace_id: string;
  tenant_id: string;
  repo_org: string;
  repo_name: string;
  owners: string;
  criticality: string;
  data_classification: string;
  runtime: string;
};

const defaults: FormState = {
  workspace_id: "",
  tenant_id: "",
  repo_org: "",
  repo_name: "",
  owners: "@platform-engineering",
  criticality: "medium",
  data_classification: "internal",
  runtime: "go1.22",
};

export function NewAppWizard({ templates }: Props) {
  const [stepIndex, setStepIndex] = useState(0);
  const [templateKey, setTemplateKey] = useState(templates[0] ? keyFor(templates[0]) : "");
  const [form, setForm] = useState(defaults);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const selected = (templates.find((template) => keyFor(template) === templateKey) ?? templates[0]) as RepoTemplate;
  const step = steps[stepIndex];
  const owners = parseOwners(form.owners);
  const checks = ["forge/lint", "forge/test-with-coverage", "forge/sast", "forge/sca", "forge/sbom", "forge/container-scan", "forge/cosign-sign-attest", "forge/openspec-link"];

  function setField(name: keyof FormState, value: string) {
    setForm((current) => ({ ...current, [name]: value }));
  }

  async function submit() {
    setSubmitting(true);
    setError(null);
    try {
      const response = await fetch("/api/onboarding", {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({
          workspace_id: form.workspace_id,
          tenant_id: form.tenant_id,
          repo_org: form.repo_org,
          repo_name: form.repo_name,
          template_id: selected?.id,
          template_version: selected?.version,
          owners,
          criticality: form.criticality,
          data_classification: form.data_classification,
          parameters: {
            name: form.repo_name,
            owner: owners[0],
            runtime: form.runtime,
            criticality: form.criticality,
            data_classification: form.data_classification,
          },
        }),
      });
      const payload = (await response.json()) as OnboardingRequest | { error?: string };
      if (!response.ok) throw new Error("error" in payload && payload.error ? payload.error : "onboarding failed");
      window.location.href = `/onboarding/${(payload as OnboardingRequest).id}`;
    } catch (e) {
      setError(e instanceof Error ? e.message : "failed to submit onboarding");
    } finally {
      setSubmitting(false);
    }
  }

  if (templates.length === 0) {
    return <div className="rounded border border-dashed border-neutral-300 p-6 text-sm opacity-70 dark:border-neutral-800">No approved templates are available.</div>;
  }

  return (
    <div className="grid gap-5 lg:grid-cols-[280px_1fr]">
      <aside className="rounded-3xl border border-neutral-200 bg-white p-4 dark:border-neutral-800 dark:bg-neutral-900">
        <p className="text-xs font-semibold uppercase tracking-wide text-neutral-500">Wizard steps</p>
        <ol className="mt-4 space-y-2">
          {steps.map((item, index) => (
            <li key={item} className={`rounded-2xl px-3 py-2 text-sm ${index === stepIndex ? "bg-neutral-900 text-white dark:bg-neutral-100 dark:text-neutral-900" : "bg-neutral-50 dark:bg-neutral-800"}`}>
              <span className="font-medium">{index + 1}. {title(item)}</span>
            </li>
          ))}
        </ol>
      </aside>

      <div className="rounded-3xl border border-neutral-200 bg-white p-5 dark:border-neutral-800 dark:bg-neutral-900" data-testid="new-app-wizard">
        {step === "template" && (
          <div className="space-y-4">
            <h3 className="text-xl font-semibold">Choose a governed template</h3>
            <div className="grid gap-3 md:grid-cols-2">
              {templates.map((template) => (
                <label key={keyFor(template)} className={`cursor-pointer rounded-2xl border p-4 ${keyFor(template) === templateKey ? "border-neutral-900 ring-2 ring-neutral-900 dark:border-neutral-100 dark:ring-neutral-100" : "border-neutral-200 dark:border-neutral-800"}`}>
                  <input className="sr-only" type="radio" name="template" checked={keyFor(template) === templateKey} onChange={() => setTemplateKey(keyFor(template))} />
                  <span className="text-xs uppercase tracking-wide opacity-60">{template.category}</span>
                  <span className="mt-1 block font-semibold">{template.id}@{template.version}</span>
                  <span className="mt-2 block text-sm opacity-70">{template.description}</span>
                  <span className="mt-3 inline-flex gap-2 text-xs"><b>{template.lifecycle_state}</b><b>{template.trust_level}</b></span>
                </label>
              ))}
            </div>
          </div>
        )}

        {step === "params" && (
          <div className="space-y-4">
            <h3 className="text-xl font-semibold">Workspace and repository parameters</h3>
            <div className="grid gap-3 md:grid-cols-2">
              <Field label="Workspace ID" value={form.workspace_id} onChange={(value) => setField("workspace_id", value)} required />
              <Field label="Tenant ID" value={form.tenant_id} onChange={(value) => setField("tenant_id", value)} required />
              <Field label="GitHub org" value={form.repo_org} onChange={(value) => setField("repo_org", value)} required />
              <Field label="Repository name" value={form.repo_name} onChange={(value) => setField("repo_name", value)} required />
              <Field label="Owners" value={form.owners} onChange={(value) => setField("owners", value)} hint="Comma-separated GitHub teams or users" />
              <Field label="Runtime" value={form.runtime} onChange={(value) => setField("runtime", value)} />
              <Select label="Criticality" value={form.criticality} values={["low", "medium", "high", "critical"]} onChange={(value) => setField("criticality", value)} />
              <Select label="Data classification" value={form.data_classification} values={["public", "internal", "confidential", "restricted"]} onChange={(value) => setField("data_classification", value)} />
            </div>
          </div>
        )}

        {step === "preview" && (
          <div className="space-y-4">
            <h3 className="text-xl font-semibold">Preview repository contract</h3>
            <div className="grid gap-3 md:grid-cols-3">
              <PreviewCard title="Repository" lines={[`${form.repo_org}/${form.repo_name}`, `main protected`, `${owners.length} CODEOWNERS entries`]} />
              <PreviewCard title="Metadata" lines={[`template ${selected.id}@${selected.version}`, `criticality ${form.criticality}`, `data ${form.data_classification}`]} />
              <PreviewCard title="Registry asset" lines={["type application", "lifecycle proposed", `image artifact-registry.local/${form.workspace_id}/${form.repo_name}`]} />
            </div>
            <div className="rounded-2xl bg-neutral-950 p-4 text-xs text-neutral-100">
              <p className="font-semibold">Required PR checks</p>
              <p className="mt-2 leading-6 opacity-80">{checks.join(" · ")}</p>
            </div>
          </div>
        )}

        {step === "confirm" && (
          <div className="space-y-4">
            <h3 className="text-xl font-semibold">Confirm onboarding</h3>
            <p className="text-sm opacity-70">Forge will validate policy, render the template, create the GitHub repository, apply branch protections, publish CI, and register the application asset.</p>
            <dl className="grid gap-3 text-sm md:grid-cols-2">
              <Summary label="Repo" value={`${form.repo_org}/${form.repo_name}`} />
              <Summary label="Template" value={`${selected.id}@${selected.version}`} />
              <Summary label="Owners" value={owners.join(", ")} />
              <Summary label="Criticality" value={form.criticality} />
            </dl>
            {error && <p className="rounded border border-red-300 bg-red-50 p-3 text-sm text-red-800 dark:border-red-800 dark:bg-red-950 dark:text-red-200">{error}</p>}
          </div>
        )}

        <div className="mt-6 flex items-center justify-between border-t border-neutral-200 pt-4 dark:border-neutral-800">
          <button className="rounded border border-neutral-300 px-4 py-2 text-sm disabled:opacity-40 dark:border-neutral-700" disabled={stepIndex === 0 || submitting} onClick={() => setStepIndex((value) => Math.max(0, value - 1))}>Back</button>
          {stepIndex < steps.length - 1 ? (
            <button className="rounded bg-neutral-900 px-4 py-2 text-sm text-white dark:bg-neutral-100 dark:text-neutral-900" onClick={() => setStepIndex((value) => Math.min(steps.length - 1, value + 1))}>Next</button>
          ) : (
            <button data-testid="confirm-onboarding" className="rounded bg-green-700 px-4 py-2 text-sm text-white disabled:opacity-50" disabled={submitting || !form.workspace_id || !form.repo_org || !form.repo_name} onClick={submit}>{submitting ? "Submitting..." : "Create app"}</button>
          )}
        </div>
      </div>
    </div>
  );
}

function Field({ label, value, onChange, hint, required }: { label: string; value: string; onChange: (value: string) => void; hint?: string; required?: boolean }) {
  return (
    <label className="grid gap-1 text-sm">
      <span className="font-medium">{label}</span>
      <input required={required} value={value} onChange={(event) => onChange(event.target.value)} className="rounded border border-neutral-300 bg-transparent px-3 py-2 dark:border-neutral-700" />
      {hint && <span className="text-xs opacity-60">{hint}</span>}
    </label>
  );
}

function Select({ label, value, values, onChange }: { label: string; value: string; values: string[]; onChange: (value: string) => void }) {
  return (
    <label className="grid gap-1 text-sm">
      <span className="font-medium">{label}</span>
      <select value={value} onChange={(event) => onChange(event.target.value)} className="rounded border border-neutral-300 bg-transparent px-3 py-2 dark:border-neutral-700">
        {values.map((item) => <option key={item} value={item}>{item}</option>)}
      </select>
    </label>
  );
}

function PreviewCard({ title, lines }: { title: string; lines: string[] }) {
  return (
    <div className="rounded-2xl border border-neutral-200 p-4 dark:border-neutral-800">
      <p className="text-xs uppercase tracking-wide opacity-60">{title}</p>
      {lines.map((line) => <p key={line} className="mt-2 text-sm font-medium">{line}</p>)}
    </div>
  );
}

function Summary({ label, value }: { label: string; value: string }) {
  return <div className="rounded-2xl bg-neutral-50 p-3 dark:bg-neutral-800"><dt className="text-xs uppercase tracking-wide opacity-60">{label}</dt><dd className="mt-1 font-medium">{value}</dd></div>;
}

function parseOwners(value: string) {
  return value.split(",").map((item) => item.trim()).filter(Boolean);
}

function keyFor(template: RepoTemplate) {
  return `${template.id}@${template.version}`;
}

function title(value: string) {
  return value.slice(0, 1).toUpperCase() + value.slice(1);
}
