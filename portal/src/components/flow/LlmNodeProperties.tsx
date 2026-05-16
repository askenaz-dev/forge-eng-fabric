"use client";

// Task 8.1 — LLM node property panel.
// Picks: prompt template, model (filtered to workspace whitelist),
// per-override fields, tools multi-select (from in-scope MCPs), output
// schema, max_tool_calls. Surfaces estimated cost-per-execution.

import type { CanonicalStep } from "@/lib/ast-canvas-adapter";
import type { PropertyPanelCatalogs } from "./PropertyPanel";

export interface LlmNodePropertiesProps {
  step: CanonicalStep;
  onChange: (patch: Partial<CanonicalStep>) => void;
  catalogs?: PropertyPanelCatalogs;
}

export function LlmNodeProperties({ step, onChange, catalogs }: LlmNodePropertiesProps) {
  const promptTemplates = catalogs?.promptTemplates ?? [];
  const models = catalogs?.models ?? [];
  const mcps = catalogs?.mcps ?? [];
  const selectedTools = new Set(step.tools ?? []);

  const promptIsFloating = isFloatingRef(step.prompt_template);
  const toolsOutsidePin = (step.tools ?? []).filter((ref) => !mcps.some((m) => m.ref === ref));

  const model = models.find((m) => m.ref === step.model?.ref);
  const tokenEstimate = estimateTokens(step);
  const costEstimate = tokenEstimate * (model?.pricingPerToken ?? 0);

  return (
    <div className="space-y-3 text-xs" data-testid="llm-node-properties">
      <Field label="Prompt template">
        <select
          value={step.prompt_template ?? ""}
          onChange={(e) => onChange({ prompt_template: e.target.value })}
          className="w-full rounded border border-neutral-300 dark:border-neutral-700 px-2 py-1 bg-transparent"
        >
          <option value="">(none — pick one)</option>
          {promptTemplates.map((p) => (
            <option key={p.ref} value={p.ref}>
              {p.label || p.ref}
            </option>
          ))}
        </select>
        {promptIsFloating && (
          <p className="mt-1 text-rose-700">
            ⚠ prompt_template uses a floating tag — pin to exact SemVer.
          </p>
        )}
      </Field>

      <Field label="Model">
        <select
          value={step.model?.ref ?? ""}
          onChange={(e) =>
            onChange({ model: { ...step.model, ref: e.target.value } })
          }
          className="w-full rounded border border-neutral-300 dark:border-neutral-700 px-2 py-1 bg-transparent"
        >
          <option value="">(none — pick one)</option>
          {models.map((m) => (
            <option key={m.ref} value={m.ref}>
              {m.label || m.ref}
              {m.provider ? ` · ${m.provider}` : ""}
            </option>
          ))}
        </select>
      </Field>

      <details className="text-[11px]">
        <summary className="cursor-pointer opacity-70">Model overrides</summary>
        <div className="mt-2 grid grid-cols-2 gap-2">
          <NumberField
            label="temperature"
            value={(step.model?.overrides?.temperature as number) ?? null}
            onChange={(v) =>
              onChange({
                model: { ...step.model, ref: step.model?.ref ?? "", overrides: { ...step.model?.overrides, temperature: v } },
              })
            }
          />
          <NumberField
            label="max_tokens"
            value={(step.model?.overrides?.max_tokens as number) ?? null}
            onChange={(v) =>
              onChange({
                model: { ...step.model, ref: step.model?.ref ?? "", overrides: { ...step.model?.overrides, max_tokens: v } },
              })
            }
          />
        </div>
      </details>

      <Field label={`Tools (${selectedTools.size})`}>
        <div className="space-y-1 max-h-40 overflow-y-auto rounded border border-neutral-200 dark:border-neutral-800 p-2">
          {mcps.length === 0 && <p className="opacity-60">No MCPs loaded yet.</p>}
          {mcps.map((m) => {
            const checked = selectedTools.has(m.ref);
            return (
              <label key={m.ref} className="flex items-start gap-2">
                <input
                  type="checkbox"
                  checked={checked}
                  onChange={(e) => {
                    const next = new Set(selectedTools);
                    if (e.target.checked) next.add(m.ref);
                    else next.delete(m.ref);
                    onChange({ tools: Array.from(next) });
                  }}
                />
                <span className="font-mono text-[10px]">{m.ref}</span>
              </label>
            );
          })}
        </div>
        {toolsOutsidePin.length > 0 && (
          <p className="mt-1 text-rose-700">
            ⚠ {toolsOutsidePin.length} tool(s) outside the workflow's MCP set: {toolsOutsidePin.join(", ")}
          </p>
        )}
      </Field>

      <Field label="Output schema">
        <OutputSchemaEditor
          value={step.outputs_schema ?? {}}
          onChange={(next) => onChange({ outputs_schema: next })}
        />
      </Field>

      <Field label="Max tool calls">
        <input
          type="number"
          min={0}
          value={step.max_tool_calls ?? 10}
          onChange={(e) => onChange({ max_tool_calls: parseInt(e.target.value, 10) || 0 })}
          className="w-24 rounded border border-neutral-300 dark:border-neutral-700 px-2 py-1 bg-transparent"
        />
      </Field>

      <div className="rounded border border-neutral-200 dark:border-neutral-800 p-2 text-[11px]">
        <p className="font-medium mb-1">Estimated cost / run</p>
        <p>
          ~{tokenEstimate} tokens × {(model?.pricingPerToken ?? 0).toExponential(2)} USD/token ={" "}
          <strong>${costEstimate.toFixed(6)}</strong>
        </p>
      </div>
    </div>
  );
}

function isFloatingRef(ref: string | undefined): boolean {
  if (!ref) return false;
  return /@(latest|main|master|stable|current)$/i.test(ref);
}

function estimateTokens(step: CanonicalStep): number {
  // Conservative placeholder. Real value comes from prompt-template-service
  // when the LLM node is wired up at runtime (task 4.5 follow-up). For the
  // editor we surface a rough range so the cost preview is non-zero.
  const base = step.prompt_template ? 300 : 0;
  return base + (step.tools?.length ?? 0) * 50;
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <label className="block">
      <span className="block text-[10px] uppercase tracking-wider opacity-60 mb-1">{label}</span>
      {children}
    </label>
  );
}

function NumberField({
  label,
  value,
  onChange,
}: {
  label: string;
  value: number | null;
  onChange: (v: number | undefined) => void;
}) {
  return (
    <label className="block">
      <span className="block text-[10px] opacity-60">{label}</span>
      <input
        type="number"
        step="0.01"
        value={value ?? ""}
        onChange={(e) => onChange(e.target.value === "" ? undefined : parseFloat(e.target.value))}
        className="w-full rounded border border-neutral-300 dark:border-neutral-700 px-2 py-1 bg-transparent"
      />
    </label>
  );
}

function OutputSchemaEditor({
  value,
  onChange,
}: {
  value: Record<string, string>;
  onChange: (next: Record<string, string>) => void;
}) {
  const entries = Object.entries(value);
  return (
    <div className="space-y-1">
      {entries.map(([name, type], i) => (
        <div key={i} className="flex gap-1">
          <input
            type="text"
            value={name}
            onChange={(e) => {
              const next = { ...value };
              delete next[name];
              next[e.target.value] = type;
              onChange(next);
            }}
            placeholder="field"
            className="flex-1 rounded border border-neutral-300 dark:border-neutral-700 px-1 py-0.5"
          />
          <input
            type="text"
            value={type}
            onChange={(e) => onChange({ ...value, [name]: e.target.value })}
            placeholder="string"
            className="w-20 rounded border border-neutral-300 dark:border-neutral-700 px-1 py-0.5"
          />
          <button
            type="button"
            onClick={() => {
              const next = { ...value };
              delete next[name];
              onChange(next);
            }}
            className="text-rose-600"
            aria-label={`Remove ${name}`}
          >
            ×
          </button>
        </div>
      ))}
      <button
        type="button"
        onClick={() => onChange({ ...value, "": "string" })}
        className="text-[11px] opacity-70 hover:opacity-100"
      >
        + add field
      </button>
    </div>
  );
}
