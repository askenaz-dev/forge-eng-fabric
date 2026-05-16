"use client";

// FlowNode is the generic React Flow node renderer used for every
// canonical step type. Presentation is driven by nodeMetadata (label,
// tint, family); type-specific config lives in the property panel
// rather than per-node renderers. This keeps the 16-type catalog
// maintainable without 16 React components that share 95% of their
// markup.
//
// Type-specific custom renderers (e.g. an inline prompt-template
// preview inside the LLM node body) can override this by registering
// a different component in the React Flow `nodeTypes` map.

import { Handle, Position, type NodeProps } from "@xyflow/react";
import { STEP_NODE_META, TRIGGER_NODE_META } from "../nodeMetadata";
import type { CanonicalStepType, CanonicalTriggerType } from "@/lib/ast-canvas-adapter";

export interface FlowNodeData extends Record<string, unknown> {
  kind: "step" | "trigger";
  type: CanonicalStepType | CanonicalTriggerType;
  label?: string;
  ref?: string;
  tool?: string;
  // Marker for legacy event-trigger-as-step shapes encountered before save
  // migration kicks in. The canvas shows a banner so authors know the node
  // will be moved to the triggers block on save.
  migratedFrom?: string;
}

export function FlowNode({ data, selected }: NodeProps) {
  const d = data as FlowNodeData;
  const meta =
    d.kind === "trigger"
      ? TRIGGER_NODE_META[d.type as CanonicalTriggerType]
      : STEP_NODE_META[d.type as CanonicalStepType];
  const tint = meta?.tint ?? "bg-neutral-100 border-neutral-400";
  const label = d.label ?? meta?.label ?? String(d.type);
  const isTrigger = d.kind === "trigger";

  return (
    <div
      className={`rounded-md border ${tint} ${selected ? "ring-2 ring-black" : ""} min-w-[180px] shadow-sm`}
      role="group"
      aria-label={`${meta?.family ?? "node"}: ${label}`}
    >
      {!isTrigger && (
        <Handle type="target" position={Position.Left} className="!bg-neutral-500" />
      )}
      <div className="px-3 py-2">
        <div className="text-[10px] uppercase tracking-wider opacity-60">{meta?.family ?? d.kind}</div>
        <div className="font-medium text-sm">{label}</div>
        {d.ref && <div className="font-mono text-[10px] opacity-70 truncate max-w-[180px]">{d.ref}</div>}
        {d.tool && <div className="font-mono text-[10px] opacity-70">→ {d.tool}</div>}
        {d.migratedFrom && (
          <div className="mt-1 text-[10px] text-orange-700">
            Legacy {d.migratedFrom} — will migrate on save
          </div>
        )}
      </div>
      <Handle type="source" position={Position.Right} className="!bg-neutral-500" />
    </div>
  );
}
