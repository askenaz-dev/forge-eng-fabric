"use client";

import { useEffect, useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import { Button, Sheet } from "@/components/primitives";
import { ScopeSelect } from "@/components/scope/ScopeSelect";
import { useToast } from "@/components/providers/ToastProvider";

const TRUST_LEVELS = ["T0", "T1", "T2", "T3", "T4", "T5"] as const;
const CHANNELS = ["stable", "beta", "alpha"] as const;
const SEMVER_RE = /^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$/;
const KEBAB_RE = /^[a-z][a-z0-9-]*[a-z0-9]$/;

type Props = {
  open: boolean;
  onOpenChange: (next: boolean) => void;
  workspaceId: string;
};

type FormState = {
  workspace_id: string;
  name: string;
  version: string;
  description: string;
  owner_team: string;
  visibility: "workspace" | "tenant";
  trust_level: (typeof TRUST_LEVELS)[number];
  owners: string;
  channel: (typeof CHANNELS)[number];
  repo: string;
  spec_path: string;
};

const EMPTY: FormState = {
  workspace_id: "",
  name: "",
  version: "0.1.0",
  description: "",
  owner_team: "",
  visibility: "workspace",
  trust_level: "T0",
  owners: "",
  channel: "stable",
  repo: "",
  spec_path: "",
};

type Published = {
  asset_id: string;
  name: string;
  version: string;
  channel: string;
  repo: string;
  spec_path: string;
};

export function RegisterSkillDrawer({ open, onOpenChange, workspaceId }: Props) {
  const router = useRouter();
  const toast = useToast();
  const [form, setForm] = useState<FormState>(() => ({ ...EMPTY, workspace_id: workspaceId }));
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [published, setPublished] = useState<Published | null>(null);

  useEffect(() => {
    if (open) {
      setForm({ ...EMPTY, workspace_id: workspaceId });
      setError(null);
      setPublished(null);
    }
  }, [open, workspaceId]);

  const nameInvalid = form.name.trim() !== "" && !KEBAB_RE.test(form.name.trim());

  const canSubmit = useMemo(() => {
    if (!form.workspace_id.trim()) return false;
    if (!KEBAB_RE.test(form.name.trim())) return false;
    if (!SEMVER_RE.test(form.version.trim())) return false;
    if (!form.owner_team.trim()) return false;
    return true;
  }, [form]);

  function set<K extends keyof FormState>(key: K, value: FormState[K]) {
    setForm((current) => ({ ...current, [key]: value }));
  }

  async function submit() {
    setSubmitting(true);
    setError(null);
    try {
      const owners = form.owners
        .split(",")
        .map((s) => s.trim())
        .filter(Boolean);
      const metadata: Record<string, unknown> = {
        channel: form.channel,
      };
      if (form.repo.trim() || form.spec_path.trim()) {
        metadata.source = {
          ...(form.repo.trim() ? { repo: form.repo.trim() } : {}),
          ...(form.spec_path.trim() ? { spec_path: form.spec_path.trim() } : {}),
        };
      }
      const response = await fetch("/api/assets", {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({
          workspace_id: form.workspace_id.trim(),
          type: "skill",
          name: form.name.trim(),
          version: form.version.trim(),
          description: form.description.trim() || undefined,
          owner_team: form.owner_team.trim(),
          visibility: form.visibility,
          trust_level: form.trust_level,
          owners,
          inputs_schema: { type: "object" },
          outputs_schema: { type: "object" },
          metadata,
        }),
      });
      const payload = (await response.json().catch(() => ({}))) as {
        id?: string;
        asset_id?: string;
        error?: string;
        message?: string;
      };
      if (!response.ok) {
        throw new Error(payload.error || payload.message || `registry ${response.status}`);
      }
      const assetId = payload.asset_id ?? payload.id ?? `skill:${form.workspace_id.trim()}:${form.name.trim()}`;
      toast.success(`Registered skill metadata ${form.name} ${form.version}`);
      setPublished({
        asset_id: assetId,
        name: form.name.trim(),
        version: form.version.trim(),
        channel: form.channel,
        repo: form.repo.trim() || "<your-repo>",
        spec_path: form.spec_path.trim() || `./skills/${form.name.trim()}.json`,
      });
      router.refresh();
    } catch (e) {
      setError(e instanceof Error ? e.message : "failed to register skill");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <Sheet
      open={open}
      onOpenChange={onOpenChange}
      title={<>Register <em>skill</em></>}
      subtitle={
        published
          ? `published metadata · asset_id=${published.asset_id} · lifecycle_state=proposed`
          : `workspace ${form.workspace_id || "—"} · channel=${form.channel} · lifecycle_state=proposed`
      }
      footer={
        published ? (
          <div style={{ display: "flex", gap: 8, justifyContent: "flex-end" }}>
            <Button variant="primary" onClick={() => onOpenChange(false)}>
              Done
            </Button>
          </div>
        ) : (
          <div style={{ display: "flex", gap: 8, justifyContent: "flex-end" }}>
            <Button variant="ghost" onClick={() => onOpenChange(false)} disabled={submitting}>
              Cancel
            </Button>
            <Button variant="primary" onClick={submit} disabled={!canSubmit || submitting}>
              {submitting ? "Registering…" : "Register metadata"}
            </Button>
          </div>
        )
      }
    >
      {published ? <NextStepsView pub={published} /> : (
        <SkillForm
          form={form}
          set={set}
          error={error}
          nameInvalid={nameInvalid}
        />
      )}
    </Sheet>
  );
}

function SkillForm({
  form,
  set,
  error,
  nameInvalid,
}: {
  form: FormState;
  set: <K extends keyof FormState>(key: K, value: FormState[K]) => void;
  error: string | null;
  nameInvalid: boolean;
}) {
  return (
    <>
      {error && (
        <p className="mb-3 rounded border border-red-300 bg-red-50 p-3 text-sm text-red-800 dark:border-red-800 dark:bg-red-950 dark:text-red-200">
          {error}
        </p>
      )}
      <p className="mb-4 rounded border border-neutral-200 bg-neutral-50 p-3 text-xs leading-relaxed text-neutral-700 dark:border-neutral-800 dark:bg-neutral-900 dark:text-neutral-300">
        Two-step publish. <strong>This drawer registers metadata only</strong> — name, version,
        ownership, trust target. The actual signed bundle (<code>SKILL.md</code> + scripts/, packaged
        as <code>.tar.zst</code> with cosign + in-toto attestation) is published from your CI using
        the snippet shown after registration. Read the{" "}
        <a className="underline" href="https://agentskills.io/home" target="_blank" rel="noreferrer">
          Agent Skills layout
        </a>{" "}
        spec if you need it.
      </p>
      <div className="grid gap-4">
        <Row label="Workspace" required hint="Where the skill is registered">
          <ScopeSelect
            kind="workspace"
            name="workspace_id"
            value={form.workspace_id}
            onChange={(next) => set("workspace_id", next)}
            required
            className={inputCls}
          />
        </Row>
        <Row
          label="Skill name"
          required
          hint={nameInvalid ? "kebab-case only — lowercase, digits, hyphens" : "kebab-case — matches the SKILL.md folder name"}
        >
          <input
            value={form.name}
            onChange={(e) => set("name", e.target.value)}
            placeholder="generate-test-cases"
            className={inputCls}
            aria-invalid={nameInvalid}
          />
        </Row>
        <div className="grid gap-4 md:grid-cols-2">
          <Row label="Version" required hint="SemVer MAJOR.MINOR.PATCH">
            <input value={form.version} onChange={(e) => set("version", e.target.value)} placeholder="0.1.0" className={inputCls} />
          </Row>
          <Row label="Channel" hint="stable rolls out to all clients; beta/alpha are opt-in">
            <select
              value={form.channel}
              onChange={(e) => set("channel", e.target.value as FormState["channel"])}
              className={inputCls}
            >
              {CHANNELS.map((c) => (
                <option key={c} value={c}>{c}</option>
              ))}
            </select>
          </Row>
        </div>
        <Row label="Description" hint="Shown in `forge skills list` and client UIs">
          <textarea
            value={form.description}
            onChange={(e) => set("description", e.target.value)}
            rows={2}
            className={inputCls}
            placeholder="Generate property-based test cases from an OpenSpec spec."
          />
        </Row>
        <Row label="Owner team" required>
          <input value={form.owner_team} onChange={(e) => set("owner_team", e.target.value)} placeholder="engineering" className={inputCls} />
        </Row>
        <Row label="Owners" hint="Comma-separated emails (optional)">
          <input value={form.owners} onChange={(e) => set("owners", e.target.value)} placeholder="ana@acme.io, ben@acme.io" className={inputCls} />
        </Row>
        <div className="grid gap-4 md:grid-cols-2">
          <Row label="Visibility">
            <select
              value={form.visibility}
              onChange={(e) => set("visibility", e.target.value as FormState["visibility"])}
              className={inputCls}
            >
              <option value="workspace">workspace</option>
              <option value="tenant">tenant</option>
            </select>
          </Row>
          <Row label="Trust level" hint="T0 minimum; T1+ unlocks gateway install">
            <select
              value={form.trust_level}
              onChange={(e) => set("trust_level", e.target.value as FormState["trust_level"])}
              className={inputCls}
            >
              {TRUST_LEVELS.map((tl) => (
                <option key={tl} value={tl}>{tl}</option>
              ))}
            </select>
          </Row>
        </div>
        <fieldset className="rounded border border-neutral-200 p-3 dark:border-neutral-800">
          <legend className="px-1 text-xs uppercase tracking-wide opacity-70">Source (for CI snippet)</legend>
          <p className="mb-3 text-xs opacity-70">Optional — used only to pre-fill the GitHub Actions step shown after submit. Not stored as a runtime field.</p>
          <div className="grid gap-3">
            <Row label="Repository" hint="org/repo or full URL">
              <input value={form.repo} onChange={(e) => set("repo", e.target.value)} placeholder="acme/skills" className={monoCls} spellCheck={false} />
            </Row>
            <Row label="Skill spec path" hint="Path to the skill folder or spec inside the repo">
              <input value={form.spec_path} onChange={(e) => set("spec_path", e.target.value)} placeholder={`./skills/${form.name || "<skill-name>"}.json`} className={monoCls} spellCheck={false} />
            </Row>
          </div>
        </fieldset>
      </div>
    </>
  );
}

function NextStepsView({ pub }: { pub: Published }) {
  const yaml = buildCISnippet(pub);
  const curl = buildCurlSnippet(pub);
  return (
    <div className="space-y-4">
      <p className="rounded border border-green-300 bg-green-50 p-3 text-sm text-green-800 dark:border-green-800 dark:bg-green-950 dark:text-green-200">
        Skill metadata registered. <strong>Now publish the bundle from CI</strong> so it becomes installable.
      </p>
      <Section title="1. Add the publish job to your repo">
        <p className="mb-2 text-xs opacity-70">
          Paste this into <code>.github/workflows/skill-publish.yml</code>. The reusable workflow packages, signs, attests, uploads and calls the gateway-publish hook.
        </p>
        <CopyBlock content={yaml} lang="yaml" />
      </Section>
      <Section title="2. Or publish from your laptop with the CLI">
        <p className="mb-2 text-xs opacity-70">
          Useful for the first publish or for local testing. Requires <code>FORGE_PUBLISH_TOKEN</code> in the environment.
        </p>
        <CopyBlock content={curl} lang="bash" />
      </Section>
      <Section title="3. Confirm">
        <p className="text-xs opacity-70">
          Once CI succeeds, the asset will flip to <code>distribution.gateway_published=true</code>. Run{" "}
          <code className="rounded bg-neutral-200 px-1 dark:bg-neutral-800">forge skills list</code> to see it, or refresh the registry page.
        </p>
      </Section>
    </div>
  );
}

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div className="rounded border border-neutral-200 p-3 dark:border-neutral-800">
      <h4 className="mb-2 text-xs font-medium uppercase tracking-wide opacity-80">{title}</h4>
      {children}
    </div>
  );
}

function CopyBlock({ content, lang }: { content: string; lang: "yaml" | "bash" }) {
  const [copied, setCopied] = useState(false);
  return (
    <div className="relative">
      <pre className="overflow-auto rounded bg-neutral-950 p-3 text-xs leading-relaxed text-neutral-100">
        <code data-lang={lang}>{content}</code>
      </pre>
      <button
        type="button"
        onClick={() => {
          void navigator.clipboard.writeText(content).then(() => {
            setCopied(true);
            setTimeout(() => setCopied(false), 1500);
          });
        }}
        className="absolute right-2 top-2 rounded border border-neutral-700 bg-neutral-900 px-2 py-1 text-[11px] text-neutral-100 hover:bg-neutral-800"
      >
        {copied ? "Copied" : "Copy"}
      </button>
    </div>
  );
}

function buildCISnippet(pub: Published): string {
  return `name: skill-publish
on:
  push:
    tags: ['skill/${pub.name}/v*']
jobs:
  publish:
    uses: forge-eng-fabric/.github/.github/workflows/skill-publish.yml@main
    with:
      skill_spec: ${pub.spec_path}
      asset_id: ${pub.asset_id}
      asset_version: ${pub.version}
      channel: ${pub.channel}
    secrets:
      forge_publish_token: \${{ secrets.FORGE_PUBLISH_TOKEN }}`;
}

function buildCurlSnippet(pub: Published): string {
  return `# 1. Package locally (deterministic .tar.zst + sha256 digest)
make package-skill SPEC=${pub.spec_path} OUT=./out/${pub.name}-${pub.version}.tar.zst
DIGEST=$(cat ./out/${pub.name}-${pub.version}.tar.zst.digest)

# 2. Sign + attest
cosign sign-blob --yes ./out/${pub.name}-${pub.version}.tar.zst

# 3. Upload to the object store
aws s3 cp ./out/${pub.name}-${pub.version}.tar.zst \\
  s3://forge-packages/${pub.asset_id}/${pub.version}.tar.zst

# 4. Tell the registry it's ready
curl --fail-with-body \\
  -H "Authorization: Bearer $FORGE_PUBLISH_TOKEN" \\
  -H "content-type: application/json" \\
  https://registry.forge.internal/v1/assets/${pub.asset_id}/versions/${pub.version}/lifecycle-hooks/gateway-publish \\
  -d "{
    \\"channel\\": \\"${pub.channel}\\",
    \\"package_digest\\": \\"$DIGEST\\",
    \\"bytes_uri\\": \\"s3://forge-packages/${pub.asset_id}/${pub.version}.tar.zst\\"
  }"`;
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
  "w-full rounded border border-neutral-300 bg-transparent px-2 py-1.5 text-sm outline-none focus:border-neutral-500 dark:border-neutral-700 dark:focus:border-neutral-400 aria-invalid:border-red-500";
const monoCls =
  "w-full rounded border border-neutral-300 bg-transparent px-2 py-1.5 font-mono text-xs outline-none focus:border-neutral-500 dark:border-neutral-700 dark:focus:border-neutral-400";
