"use client";

// PropertyPanel is the right-rail editor for the currently selected
// canvas node. It dispatches to per-type sub-forms; LlmNodeProperties
// (task 8.1) is the most substantial.

import type { CanonicalStep, CanonicalTrigger } from "@/lib/ast-canvas-adapter";
import type { FlowNodeData } from "./nodes/FlowNode";
import { LlmNodeProperties } from "./LlmNodeProperties";

export interface PropertyPanelProps {
  selected: SelectedNode | null;
  onChangeStep: (id: string, patch: Partial<CanonicalStep>) => void;
  onChangeTrigger: (id: string, patch: Partial<CanonicalTrigger>) => void;
  /** Asset catalogs and registry data piped through from the editor shell. */
  catalogs?: PropertyPanelCatalogs;
}

export interface PropertyPanelCatalogs {
  promptTemplates: { ref: string; label: string }[];
  models: { ref: string; label: string; provider?: string; pricingPerToken?: number }[];
  mcps: { ref: string; label: string }[];
}

export type SelectedNode =
  | { kind: "step"; step: CanonicalStep; data: FlowNodeData }
  | { kind: "trigger"; trigger: CanonicalTrigger; data: FlowNodeData };

export function PropertyPanel({ selected, onChangeStep, onChangeTrigger, catalogs }: PropertyPanelProps) {
  if (!selected) {
    return (
      <aside className="w-[320px] shrink-0 border-l border-neutral-200 dark:border-neutral-800 p-4 text-sm opacity-60">
        Select a node to edit its properties.
      </aside>
    );
  }
  return (
    <aside className="w-[320px] shrink-0 border-l border-neutral-200 dark:border-neutral-800 p-4 overflow-y-auto">
      <h3 className="text-xs font-semibold uppercase tracking-wide opacity-60 mb-3">
        {selected.kind === "trigger" ? "Trigger" : "Step"} · {selected.data.label}
      </h3>
      {selected.kind === "step" && selected.step.type === "llm" ? (
        <LlmNodeProperties
          step={selected.step}
          onChange={(patch) => onChangeStep(selected.step.id, patch)}
          catalogs={catalogs}
        />
      ) : selected.kind === "step" ? (
        <StepGenericProperties step={selected.step} onChange={(patch) => onChangeStep(selected.step.id, patch)} />
      ) : (
        <TriggerGenericProperties
          trigger={selected.trigger}
          onChange={(patch) => onChangeTrigger(selected.trigger.id, patch)}
        />
      )}
    </aside>
  );
}

function StepGenericProperties({
  step,
  onChange,
}: {
  step: CanonicalStep;
  onChange: (patch: Partial<CanonicalStep>) => void;
}) {
  return (
    <div className="space-y-3 text-xs">
      <Field label="ID"><code>{step.id}</code></Field>
      <Field label="Type"><code>{step.type}</code></Field>
      <Field label="Ref">
        <input
          type="text"
          value={step.ref ?? ""}
          onChange={(e) => onChange({ ref: e.target.value })}
          className="w-full rounded border border-neutral-300 dark:border-neutral-700 px-2 py-1 bg-transparent"
          placeholder="registry:..."
        />
      </Field>
      {(step.type === "mcp" || step.type === "webhook") && (
        <Field label="Tool">
          <input
            type="text"
            value={step.tool ?? ""}
            onChange={(e) => onChange({ tool: e.target.value })}
            className="w-full rounded border border-neutral-300 dark:border-neutral-700 px-2 py-1 bg-transparent"
          />
        </Field>
      )}
      <Field label="Depends on">
        <code className="opacity-60">{step.depends_on?.join(", ") || "(none)"}</code>
      </Field>
    </div>
  );
}

function TriggerGenericProperties({
  trigger,
  onChange,
}: {
  trigger: CanonicalTrigger;
  onChange: (patch: Partial<CanonicalTrigger>) => void;
}) {
  return (
    <div className="space-y-3 text-xs">
      <Field label="ID"><code>{trigger.id}</code></Field>
      <Field label="Type"><code>{trigger.type}</code></Field>
      {trigger.type === "cron" && (
        <Field label="Expression">
          <input
            type="text"
            value={(trigger.config?.expression as string) ?? ""}
            onChange={(e) =>
              onChange({ config: { ...trigger.config, expression: e.target.value } })
            }
            placeholder="0 */6 * * *"
            className="w-full rounded border border-neutral-300 dark:border-neutral-700 px-2 py-1 font-mono bg-transparent"
          />
        </Field>
      )}
      {trigger.type === "email-inbound" && (
        <Field label="Mailbox ref">
          <input
            type="text"
            value={(trigger.config?.mailbox_ref as string) ?? ""}
            onChange={(e) =>
              onChange({ config: { ...trigger.config, mailbox_ref: e.target.value } })
            }
            placeholder="ws:mailbox:support"
            className="w-full rounded border border-neutral-300 dark:border-neutral-700 px-2 py-1 bg-transparent"
          />
        </Field>
      )}
      <Field label="Concurrency">
        <select
          value={trigger.concurrency ?? "queue"}
          onChange={(e) => onChange({ concurrency: e.target.value as "queue" | "drop" | "overlap" })}
          className="w-full rounded border border-neutral-300 dark:border-neutral-700 px-2 py-1 bg-transparent"
        >
          <option value="queue">queue (default)</option>
          <option value="drop">drop</option>
          <option value="overlap">overlap</option>
        </select>
      </Field>
    </div>
  );
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <label className="block">
      <span className="block text-[10px] uppercase tracking-wider opacity-60 mb-1">{label}</span>
      {children}
    </label>
  );
}
