"use client";

import { useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import { Button, Sheet } from "@/components/primitives";
import { useToast } from "@/components/providers/ToastProvider";

type LifecycleState = "proposed" | "in_review" | "approved" | "deprecated" | "retired";
type TrustLevel = "T0" | "T1" | "T2" | "T3" | "T4" | "T5";

const TRUST_LEVELS: TrustLevel[] = ["T0", "T1", "T2", "T3", "T4", "T5"];
const EVAL_KEYS = ["quality", "safety", "cost", "latency"] as const;

type Props = {
  assetId: string;
  version: string;
  lifecycleState: LifecycleState;
  trustLevel: TrustLevel;
  evalScores: Record<string, number>;
};

export function LifecycleActions({ assetId, version, lifecycleState, trustLevel, evalScores }: Props) {
  const router = useRouter();
  const toast = useToast();
  const [reviewOpen, setReviewOpen] = useState(false);
  const [approveOpen, setApproveOpen] = useState(false);
  const [busy, setBusy] = useState(false);

  async function transition(next: LifecycleState) {
    if (!confirm(`Transition to ${next}?`)) return;
    setBusy(true);
    try {
      const response = await fetch(
        `/api/assets/${encodeURIComponent(assetId)}/versions/${encodeURIComponent(version)}/transition`,
        {
          method: "POST",
          headers: { "content-type": "application/json" },
          body: JSON.stringify({ lifecycle_state: next, trust_level: trustLevel, eval_scores: evalScores }),
        },
      );
      const payload = (await response.json().catch(() => ({}))) as { message?: string; error?: string };
      if (!response.ok) throw new Error(payload.error || payload.message || `transition ${response.status}`);
      toast.success(`Lifecycle → ${next}`);
      router.refresh();
    } catch (e) {
      toast.err(e instanceof Error ? e.message : "transition failed");
    } finally {
      setBusy(false);
    }
  }

  const canSubmitReview = lifecycleState === "proposed";
  const canApprove = lifecycleState === "in_review";
  const canDeprecate = lifecycleState === "approved";
  const canRetire = lifecycleState !== "retired";
  const canSendBack = lifecycleState === "in_review";

  return (
    <>
      <div className="flex flex-wrap gap-2">
        {canSubmitReview && (
          <Button variant="primary" onClick={() => setReviewOpen(true)} disabled={busy}>
            Submit for review
          </Button>
        )}
        {canApprove && (
          <Button variant="primary" onClick={() => setApproveOpen(true)} disabled={busy}>
            Approve
          </Button>
        )}
        {canSendBack && (
          <Button variant="secondary" onClick={() => transition("proposed")} disabled={busy}>
            Send back to proposed
          </Button>
        )}
        {canDeprecate && (
          <Button variant="secondary" onClick={() => transition("deprecated")} disabled={busy}>
            Deprecate
          </Button>
        )}
        {canRetire && (
          <Button variant="danger" onClick={() => transition("retired")} disabled={busy}>
            Retire
          </Button>
        )}
        {lifecycleState === "retired" && (
          <p className="text-sm opacity-60">Retired — no further transitions allowed.</p>
        )}
      </div>

      <ReviewDrawer
        open={reviewOpen}
        onOpenChange={setReviewOpen}
        assetId={assetId}
        version={version}
        defaultTrustLevel={trustLevel === "T0" ? "T1" : trustLevel}
        defaultScores={evalScores}
        onDone={() => router.refresh()}
      />
      <ApproveDrawer
        open={approveOpen}
        onOpenChange={setApproveOpen}
        assetId={assetId}
        version={version}
        defaultTrustLevel={trustLevel === "T0" || trustLevel === "T1" ? "T3" : trustLevel}
        defaultScores={evalScores}
        onDone={() => router.refresh()}
      />
    </>
  );
}

type GateRow = { stage: string; outcome: "pass" | "warn" | "fail"; report_url: string };

const DEFAULT_GATES: GateRow[] = [
  { stage: "lint", outcome: "pass", report_url: "" },
  { stage: "test", outcome: "pass", report_url: "" },
  { stage: "sast", outcome: "pass", report_url: "" },
  { stage: "sca", outcome: "pass", report_url: "" },
  { stage: "sbom", outcome: "pass", report_url: "" },
  { stage: "sign", outcome: "pass", report_url: "" },
];

function ReviewDrawer({
  open,
  onOpenChange,
  assetId,
  version,
  defaultTrustLevel,
  defaultScores,
  onDone,
}: {
  open: boolean;
  onOpenChange: (next: boolean) => void;
  assetId: string;
  version: string;
  defaultTrustLevel: TrustLevel;
  defaultScores: Record<string, number>;
  onDone: () => void;
}) {
  const toast = useToast();
  const [pipelineRunId, setPipelineRunId] = useState("");
  const [commitSha, setCommitSha] = useState("");
  const [imageDigest, setImageDigest] = useState("");
  const [verified, setVerified] = useState({ image_signed: true, signature_verified: true, attestation_verified: true, sbom_published: true });
  const [gates, setGates] = useState<GateRow[]>(DEFAULT_GATES);
  const [trustLevel, setTrustLevel] = useState<TrustLevel>(defaultTrustLevel);
  const [scores, setScores] = useState<Record<string, string>>(() =>
    Object.fromEntries(EVAL_KEYS.map((k) => [k, String(defaultScores[k] ?? 0.9)])),
  );
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const canSubmit = useMemo(() => {
    if (!pipelineRunId.trim() || !commitSha.trim() || !imageDigest.trim()) return false;
    if (!verified.image_signed || !verified.signature_verified || !verified.attestation_verified || !verified.sbom_published) return false;
    return gates.every((g) => g.outcome !== "fail");
  }, [pipelineRunId, commitSha, imageDigest, verified, gates]);

  async function submit() {
    setSubmitting(true);
    setError(null);
    try {
      const evalScores: Record<string, number> = {};
      for (const key of EVAL_KEYS) {
        const value = Number.parseFloat(scores[key] ?? "0");
        if (Number.isNaN(value) || value < 0 || value > 1) {
          throw new Error(`${key} score must be a number between 0 and 1`);
        }
        evalScores[key] = value;
      }
      const response = await fetch(
        `/api/assets/${encodeURIComponent(assetId)}/versions/${encodeURIComponent(version)}/pipeline-green`,
        {
          method: "POST",
          headers: { "content-type": "application/json" },
          body: JSON.stringify({
            pipeline_run_id: pipelineRunId.trim(),
            commit_sha: commitSha.trim(),
            image_digest: imageDigest.trim(),
            ...verified,
            gate_results: gates.filter((g) => g.stage.trim()).map((g) => ({ stage: g.stage.trim(), outcome: g.outcome, report_url: g.report_url.trim() })),
            trust_level: trustLevel,
            eval_scores: evalScores,
          }),
        },
      );
      const payload = (await response.json().catch(() => ({}))) as {
        decision?: string;
        reason?: string;
        error?: string;
        message?: string;
      };
      if (!response.ok) throw new Error(payload.error || payload.message || `pipeline-green ${response.status}`);
      if (payload.decision === "waiting") {
        throw new Error(payload.reason || "registry is still waiting for upstream checks");
      }
      toast.success(`Lifecycle → in_review (${trustLevel})`);
      onOpenChange(false);
      onDone();
    } catch (e) {
      setError(e instanceof Error ? e.message : "submit for review failed");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <Sheet
      open={open}
      onOpenChange={onOpenChange}
      title={<>Submit for <em>review</em></>}
      subtitle={`${assetId}@${version} · proposed → in_review`}
      footer={
        <div style={{ display: "flex", gap: 8, justifyContent: "flex-end" }}>
          <Button variant="ghost" onClick={() => onOpenChange(false)} disabled={submitting}>Cancel</Button>
          <Button variant="primary" onClick={submit} disabled={!canSubmit || submitting}>
            {submitting ? "Submitting…" : "Submit"}
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
        <Row label="Pipeline run ID" required>
          <input value={pipelineRunId} onChange={(e) => setPipelineRunId(e.target.value)} className={inputCls} placeholder="github-run-12345" />
        </Row>
        <Row label="Commit SHA" required>
          <input value={commitSha} onChange={(e) => setCommitSha(e.target.value)} className={monoCls} placeholder="abc123def…" />
        </Row>
        <Row label="Image digest" required>
          <input value={imageDigest} onChange={(e) => setImageDigest(e.target.value)} className={monoCls} placeholder="sha256:…" />
        </Row>
        <fieldset className="rounded border border-neutral-200 p-3 dark:border-neutral-800">
          <legend className="px-1 text-xs uppercase tracking-wide opacity-70">Supply chain</legend>
          <div className="grid gap-1 text-sm">
            {(Object.keys(verified) as (keyof typeof verified)[]).map((key) => (
              <label key={key} className="flex items-center gap-2">
                <input type="checkbox" checked={verified[key]} onChange={(e) => setVerified((v) => ({ ...v, [key]: e.target.checked }))} />
                <span>{key.replace(/_/g, " ")}</span>
              </label>
            ))}
          </div>
        </fieldset>
        <Row label="Pipeline gates" hint="All must be pass or warn">
          <div className="space-y-2">
            {gates.map((gate, i) => (
              <div key={i} className="flex items-center gap-2">
                <input value={gate.stage} onChange={(e) => updateGate(setGates, i, { stage: e.target.value })} className={inputCls} placeholder="stage" />
                <select value={gate.outcome} onChange={(e) => updateGate(setGates, i, { outcome: e.target.value as GateRow["outcome"] })} className={selectCls} style={{ width: 100 }}>
                  <option value="pass">pass</option>
                  <option value="warn">warn</option>
                  <option value="fail">fail</option>
                </select>
                <input value={gate.report_url} onChange={(e) => updateGate(setGates, i, { report_url: e.target.value })} className={inputCls} placeholder="report URL" />
                <button type="button" onClick={() => setGates((g) => g.filter((_, j) => j !== i))} className="opacity-60 hover:opacity-100" aria-label="remove">×</button>
              </div>
            ))}
            <button type="button" onClick={() => setGates((g) => [...g, { stage: "", outcome: "pass", report_url: "" }])} className="text-xs opacity-70 hover:opacity-100">
              + add gate
            </button>
          </div>
        </Row>
        <div className="grid gap-4 md:grid-cols-2">
          <Row label="Trust level">
            <select value={trustLevel} onChange={(e) => setTrustLevel(e.target.value as TrustLevel)} className={selectCls}>
              {TRUST_LEVELS.filter((tl) => tl !== "T0").map((tl) => (
                <option key={tl} value={tl}>{tl}</option>
              ))}
            </select>
          </Row>
        </div>
        <Row label="Eval scores (0..1)" hint="quality / safety / cost / latency">
          <div className="grid grid-cols-2 gap-2">
            {EVAL_KEYS.map((key) => (
              <label key={key} className="flex items-center gap-2 text-xs">
                <span className="w-16 opacity-70">{key}</span>
                <input value={scores[key]} onChange={(e) => setScores((s) => ({ ...s, [key]: e.target.value }))} className={monoCls} />
              </label>
            ))}
          </div>
        </Row>
      </div>
    </Sheet>
  );
}

function ApproveDrawer({
  open,
  onOpenChange,
  assetId,
  version,
  defaultTrustLevel,
  defaultScores,
  onDone,
}: {
  open: boolean;
  onOpenChange: (next: boolean) => void;
  assetId: string;
  version: string;
  defaultTrustLevel: TrustLevel;
  defaultScores: Record<string, number>;
  onDone: () => void;
}) {
  const toast = useToast();
  const [comment, setComment] = useState("");
  const [trustLevel, setTrustLevel] = useState<TrustLevel>(defaultTrustLevel);
  const [scores, setScores] = useState<Record<string, string>>(() =>
    Object.fromEntries(EVAL_KEYS.map((k) => [k, String(defaultScores[k] ?? 0.95)])),
  );
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function submit() {
    setSubmitting(true);
    setError(null);
    try {
      const evalScores: Record<string, number> = {};
      for (const key of EVAL_KEYS) {
        const value = Number.parseFloat(scores[key] ?? "0");
        if (Number.isNaN(value) || value < 0 || value > 1) {
          throw new Error(`${key} score must be a number between 0 and 1`);
        }
        evalScores[key] = value;
      }
      const response = await fetch(
        `/api/assets/${encodeURIComponent(assetId)}/versions/${encodeURIComponent(version)}/approve`,
        {
          method: "POST",
          headers: { "content-type": "application/json" },
          body: JSON.stringify({
            comment,
            trust_level: trustLevel,
            eval_scores: evalScores,
          }),
        },
      );
      const payload = (await response.json().catch(() => ({}))) as {
        decision?: string;
        code?: string;
        failing?: Record<string, unknown>;
        error?: string;
        message?: string;
      };
      if (!response.ok) {
        if (payload.code === "eval_threshold_failed" && payload.failing) {
          throw new Error(`eval thresholds for ${trustLevel} failed: ${Object.keys(payload.failing).join(", ")}`);
        }
        throw new Error(payload.error || payload.message || `approve ${response.status}`);
      }
      toast.success(`Lifecycle → approved (${trustLevel})`);
      onOpenChange(false);
      onDone();
    } catch (e) {
      setError(e instanceof Error ? e.message : "approval failed");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <Sheet
      open={open}
      onOpenChange={onOpenChange}
      title={<>Approve <em>asset</em></>}
      subtitle={`${assetId}@${version} · in_review → approved`}
      footer={
        <div style={{ display: "flex", gap: 8, justifyContent: "flex-end" }}>
          <Button variant="ghost" onClick={() => onOpenChange(false)} disabled={submitting}>Cancel</Button>
          <Button variant="primary" onClick={submit} disabled={submitting}>
            {submitting ? "Approving…" : "Approve"}
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
        <Row label="Comment" hint="Will be recorded on the lifecycle event">
          <textarea value={comment} onChange={(e) => setComment(e.target.value)} rows={3} className={inputCls} placeholder="Approved after reviewing evals and sign-off." />
        </Row>
        <Row label="Trust level" hint="Sets the gate the eval scores must clear (T5 requires SDLC sign-off)">
          <select value={trustLevel} onChange={(e) => setTrustLevel(e.target.value as TrustLevel)} className={selectCls}>
            {TRUST_LEVELS.map((tl) => (
              <option key={tl} value={tl}>{tl}</option>
            ))}
          </select>
        </Row>
        <Row label="Eval scores (0..1)" hint="Each score must clear the trust-level threshold">
          <div className="grid grid-cols-2 gap-2">
            {EVAL_KEYS.map((key) => (
              <label key={key} className="flex items-center gap-2 text-xs">
                <span className="w-16 opacity-70">{key}</span>
                <input value={scores[key]} onChange={(e) => setScores((s) => ({ ...s, [key]: e.target.value }))} className={monoCls} />
              </label>
            ))}
          </div>
        </Row>
      </div>
    </Sheet>
  );
}

function updateGate(set: (updater: (prev: GateRow[]) => GateRow[]) => void, index: number, patch: Partial<GateRow>) {
  set((prev) => prev.map((g, i) => (i === index ? { ...g, ...patch } : g)));
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
const selectCls = inputCls;
const monoCls =
  "w-full rounded border border-neutral-300 bg-transparent px-2 py-1.5 font-mono text-xs outline-none focus:border-neutral-500 dark:border-neutral-700 dark:focus:border-neutral-400";
