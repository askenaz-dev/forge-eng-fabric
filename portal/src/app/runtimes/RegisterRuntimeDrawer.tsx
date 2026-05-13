"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { Button, Sheet } from "@/components/primitives";
import { ScopeSelect } from "@/components/scope/ScopeSelect";
import { useToast } from "@/components/providers/ToastProvider";

// Type values MUST match what services/runtime-registry validates via
// `validType` (gke | cloudrun | minikube).
type RuntimeType = "gke" | "cloudrun" | "minikube";

type Props = {
  open: boolean;
  onOpenChange: (next: boolean) => void;
};

type FormState = {
  name: string;
  type: RuntimeType;
  workspace_id: string;
  tenant_id: string;
  region: string;
  visibility: "workspace" | "tenant";
  endpoint: string;
  namespace: string;
  cluster_name: string;
  project_id: string;
  service_account_email: string;
  kubeconfig: string;
  sa_key: string;
};

const EMPTY: FormState = {
  name: "",
  type: "gke",
  workspace_id: "",
  tenant_id: "",
  region: "",
  visibility: "workspace",
  endpoint: "",
  namespace: "default",
  cluster_name: "",
  project_id: "",
  service_account_email: "",
  kubeconfig: "",
  sa_key: "",
};

const TYPE_OPTIONS: { value: RuntimeType; label: string; hint: string }[] = [
  { value: "gke",       label: "GKE",      hint: "Kubernetes — Google Kubernetes Engine" },
  { value: "cloudrun",  label: "Cloud Run", hint: "Serverless containers on GCP" },
  { value: "minikube",  label: "Minikube",  hint: "Local Kubernetes for dev / demos" },
];

export function RegisterRuntimeDrawer({ open, onOpenChange }: Props) {
  const router = useRouter();
  const toast = useToast();
  const [form, setForm] = useState<FormState>(EMPTY);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  function set<K extends keyof FormState>(key: K, value: FormState[K]) {
    setForm((current) => ({ ...current, [key]: value }));
  }

  const isCloudRun = form.type === "cloudrun";
  const isMinikube = form.type === "minikube";
  const credentialMissing = isCloudRun ? !form.sa_key.trim() : !isMinikube && !form.kubeconfig.trim();
  const canSubmit =
    form.name.trim() &&
    form.workspace_id.trim() &&
    form.tenant_id.trim() &&
    !credentialMissing;

  async function submit() {
    setSubmitting(true);
    setError(null);
    try {
      const payload: Record<string, unknown> = {
        name: form.name.trim(),
        type: form.type,
        mode: "byo",
        workspace_id: form.workspace_id.trim(),
        tenant_id: form.tenant_id.trim(),
        visibility: form.visibility,
        region: form.region.trim() || undefined,
        endpoint: form.endpoint.trim() || undefined,
        namespace: form.namespace.trim() || undefined,
      };
      if (isCloudRun) {
        payload.project_id = form.project_id.trim() || undefined;
        payload.service_account_email = form.service_account_email.trim() || undefined;
        payload.sa_key = form.sa_key.trim();
      } else {
        payload.cluster_name = form.cluster_name.trim() || undefined;
        if (!isMinikube) payload.kubeconfig = form.kubeconfig.trim();
      }

      const resp = await fetch("/api/runtimes", {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify(payload),
      });
      const body = (await resp.json().catch(() => ({}))) as { runtime?: { id?: string; name?: string }; message?: string; error?: string };
      if (!resp.ok) {
        throw new Error(body.error || body.message || `runtime-registry ${resp.status}`);
      }
      toast.success(`Registered ${body.runtime?.name ?? form.name}`);
      setForm(EMPTY);
      onOpenChange(false);
      router.refresh();
    } catch (e) {
      setError(e instanceof Error ? e.message : "register failed");
    } finally {
      setSubmitting(false);
    }
  }

  const credentialLabel = isCloudRun ? "Service-account key (JSON)" : "Kubeconfig";
  const credentialHint = isCloudRun
    ? "Paste the SA JSON. Stored encrypted; never logged."
    : isMinikube
    ? "Optional — Minikube runtimes work without a kubeconfig in local dev."
    : "Paste the kubeconfig the platform should use. Stored encrypted.";

  return (
    <Sheet
      open={open}
      onOpenChange={onOpenChange}
      title={<>Register <em>BYO runtime</em></>}
      subtitle="Connects an existing cluster the platform can deploy to. Preflight runs automatically afterwards."
      footer={
        <div style={{ display: "flex", gap: 8, justifyContent: "flex-end" }}>
          <Button variant="ghost" onClick={() => onOpenChange(false)} disabled={submitting}>Cancel</Button>
          <Button variant="primary" onClick={submit} disabled={!canSubmit || submitting}>
            {submitting ? "Registering…" : "Register"}
          </Button>
        </div>
      }
    >
      {error && (
        <p className="mb-3 rounded border border-red-300 bg-red-50 p-3 text-sm text-red-800 dark:border-red-800 dark:bg-red-950 dark:text-red-200">
          {error}
        </p>
      )}
      <div className="grid gap-4">
        <Row label="Name" required>
          <input value={form.name} onChange={(e) => set("name", e.target.value)} placeholder="acme-prod-us-east" className={inputCls} />
        </Row>

        <Row label="Type" required hint={TYPE_OPTIONS.find((t) => t.value === form.type)?.hint}>
          <select value={form.type} onChange={(e) => set("type", e.target.value as RuntimeType)} className={inputCls}>
            {TYPE_OPTIONS.map((opt) => (
              <option key={opt.value} value={opt.value}>{opt.label}</option>
            ))}
          </select>
        </Row>

        <div className="grid gap-4 md:grid-cols-2">
          <Row label="Tenant" required>
            <ScopeSelect
              kind="tenant"
              name="tenant_id"
              value={form.tenant_id}
              onChange={(next) => set("tenant_id", next)}
              required
              className={monoCls}
            />
          </Row>
          <Row label="Workspace" required>
            <ScopeSelect
              kind="workspace"
              name="workspace_id"
              value={form.workspace_id}
              onChange={(next) => set("workspace_id", next)}
              required
              className={monoCls}
            />
          </Row>
        </div>

        <Row label="Visibility" hint={form.visibility === "tenant" ? "Visible to every workspace in the tenant." : "Visible only to this workspace."}>
          <select value={form.visibility} onChange={(e) => set("visibility", e.target.value as FormState["visibility"])} className={inputCls}>
            <option value="workspace">Workspace (private)</option>
            <option value="tenant">Tenant (shared)</option>
          </select>
        </Row>

        <Row label="Region" hint="e.g. us-east-1, europe-west4">
          <input value={form.region} onChange={(e) => set("region", e.target.value)} className={inputCls} />
        </Row>

        {!isCloudRun && (
          <>
            <Row label="Cluster name">
              <input value={form.cluster_name} onChange={(e) => set("cluster_name", e.target.value)} className={inputCls} />
            </Row>
            <div className="grid gap-4 md:grid-cols-2">
              <Row label="API endpoint">
                <input value={form.endpoint} onChange={(e) => set("endpoint", e.target.value)} placeholder="https://k8s.example.com" className={monoCls} />
              </Row>
              <Row label="Namespace">
                <input value={form.namespace} onChange={(e) => set("namespace", e.target.value)} className={monoCls} />
              </Row>
            </div>
          </>
        )}

        {isCloudRun && (
          <div className="grid gap-4 md:grid-cols-2">
            <Row label="Project ID" required>
              <input value={form.project_id} onChange={(e) => set("project_id", e.target.value)} placeholder="my-gcp-project" className={monoCls} />
            </Row>
            <Row label="Service-account email">
              <input value={form.service_account_email} onChange={(e) => set("service_account_email", e.target.value)} placeholder="deploy@my-gcp-project.iam.gserviceaccount.com" className={monoCls} />
            </Row>
          </div>
        )}

        <Row label={credentialLabel} required={!isMinikube} hint={credentialHint}>
          <textarea
            value={isCloudRun ? form.sa_key : form.kubeconfig}
            onChange={(e) => (isCloudRun ? set("sa_key", e.target.value) : set("kubeconfig", e.target.value))}
            rows={6}
            className={monoCls}
            spellCheck={false}
            placeholder={isCloudRun ? "{ \"type\": \"service_account\", ... }" : "apiVersion: v1\nclusters:\n  - cluster: ...\n"}
          />
        </Row>
      </div>
    </Sheet>
  );
}

function Row({ label, hint, required, children }: { label: string; hint?: string; required?: boolean; children: React.ReactNode }) {
  return (
    <label className="block">
      <span className="mb-1 flex items-baseline justify-between gap-3 text-xs uppercase tracking-wide opacity-70">
        <span>{label}{required && <span className="ml-1 text-red-600 dark:text-red-400">*</span>}</span>
        {hint && <span className="text-[10px] normal-case opacity-60">{hint}</span>}
      </span>
      {children}
    </label>
  );
}

const inputCls =
  "w-full rounded border border-neutral-300 bg-transparent px-2 py-1.5 text-sm outline-none focus:border-neutral-500 dark:border-neutral-700 dark:focus:border-neutral-400";
const monoCls =
  "w-full rounded border border-neutral-300 bg-transparent px-2 py-1.5 font-mono text-xs outline-none focus:border-neutral-500 dark:border-neutral-700 dark:focus:border-neutral-400";
