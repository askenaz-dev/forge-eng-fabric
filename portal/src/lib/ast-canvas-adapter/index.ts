/**
 * AST ↔ canvas adapter.
 *
 * Translates between the React Flow canvas's native graph format (nodes +
 * edges) and the platform's canonical workflow AST defined in
 * pkg/workflow/ast. Round-trip identity is required so the YAML and the
 * visual editor are interchangeable surfaces over the same artifact.
 *
 * Source of truth for the catalog is pkg/workflow/ast/catalog.json; the
 * Go-side parity test (pkg/workflow/ast/parity_test.go) catches drift in
 * either direction.
 *
 * Historically this module was called `flowise-adapter` (see ADR-0001).
 * It was renamed in the ai-flow-authoring change after the Flowise embed
 * decision was superseded by React Flow (ADR-0002). The old function
 * names remain as deprecated aliases so prior call sites keep building
 * during the rename.
 */

// Source of truth: pkg/workflow/ast/catalog.json. Drift is caught by the Go
// parity test pkg/workflow/ast/parity_test.go. Any change here MUST be paired
// with the same change in the JSON catalog and a green Go test run.
// (See ai-flow-authoring change, design.md D8.)
export const CANONICAL_STEP_TYPES = [
  "skill",
  "mcp",
  "llm",
  "agent",
  "prompt-template",
  "branch",
  "loop",
  "human-in-the-loop",
  "sub-workflow",
  "webhook",
  "github-action",
  "deploy-action",
  "approval-action",
  "notification-action",
  "eval",
  "custom",
] as const;

export type CanonicalStepType = (typeof CANONICAL_STEP_TYPES)[number];

// Deprecated step types remain parseable so legacy ASTs round-trip through
// the editor. The runtime auto-migrates them on save per the entries here.
export const DEPRECATED_STEP_TYPES = {
  prompt: "prompt-template",
  "event-trigger": "", // migrated into spec.triggers, no step replacement
} as const;

export type CanonicalDeprecatedStepType = keyof typeof DEPRECATED_STEP_TYPES;

// Trigger types are a sibling block to steps (spec.triggers). Source of truth
// is also catalog.json (trigger_types). See automation-triggers spec.
export const CANONICAL_TRIGGER_TYPES = [
  "manual",
  "cron",
  "webhook-in",
  "event-bus",
  "email-inbound",
] as const;

export type CanonicalTriggerType = (typeof CANONICAL_TRIGGER_TYPES)[number];

export type TriggerConcurrency = "queue" | "drop" | "overlap";

export interface CanonicalTrigger {
  id: string;
  type: CanonicalTriggerType;
  config?: Record<string, unknown>;
  outputs?: Record<string, string>;
  concurrency?: TriggerConcurrency;
}

export interface ModelBinding {
  ref: string;
  overrides?: Record<string, unknown>;
}

export interface CanonicalStep {
  id: string;
  type: CanonicalStepType;
  ref?: string;
  tool?: string;
  inputs?: Record<string, unknown>;
  depends_on?: string[];
  retries?: { max?: number; backoff?: "constant" | "exponential" };
  timeout?: string;
  approver_role?: string;
  on_timeout?: "fail" | "escalate";
  escalation_role?: string;

  // LLM step fields (type === "llm"). See llm-flow-node capability.
  prompt_template?: string;
  model?: ModelBinding;
  tools?: string[];
  max_tool_calls?: number;
  outputs_schema?: Record<string, string>;

  /**
   * Active surface pinned at design time. When set, the visual editor
   * persists the gateway endpoint chosen for this node so the runtime
   * does not need to re-resolve it from the registry on every dispatch.
   * The active-registry-gateways spec calls this "active_surface" and
   * keeps the wire shape parallel with the Asset Registry field of the
   * same name.
   */
  active_surface?: NodeActiveSurface;
}

/** Narrowing helper: is this an LLM step? */
export function isLlmStep(step: CanonicalStep): boolean {
  return step.type === "llm";
}

/**
 * NodeActiveSurface is the per-node projection of the asset's active
 * surface. The endpoint field is required for `family ∈ {mcp, a2a}`;
 * skills carry the artifact pointer instead. The shape matches the
 * canonical block on the Asset Registry row.
 */
export interface NodeActiveSurface {
  family: "mcp" | "a2a" | "skill";
  endpoint?: string;
  artifact_pointer?: string;
  digest?: string;
  signature_id?: string;
}

export interface CanonicalWorkflow {
  apiVersion: "forge.workflows/v1";
  kind: "Workflow";
  metadata: {
    id: string;
    name: string;
    version: string;
    visibility?: "private" | "workspace" | "tenant" | "public";
    criticality?: "low" | "medium" | "high" | "critical";
    description?: string;
    tags?: string[];
    owners?: string[];
  };
  spec: {
    inputs?: { name: string; type: string; required?: boolean; default?: unknown }[];
    triggers?: CanonicalTrigger[];
    steps: CanonicalStep[];
    on_failure?: CanonicalStep[];
    outputs?: { name: string; from: string }[];
  };
}

export interface CanvasGraph {
  nodes: CanvasNode[];
  edges: CanvasEdge[];
  meta?: Record<string, unknown>;
}

export interface CanvasNode {
  id: string;
  data: {
    label?: string;
    nodeType: CanonicalStepType;
    ref?: string;
    tool?: string;
    inputs?: Record<string, unknown>;
    raw?: CanonicalStep;
  };
  position: { x: number; y: number };
  type?: string;
}

export interface CanvasEdge {
  id: string;
  source: string;
  target: string;
}

/** Convert a canonical workflow AST into a canvas graph for the editor. */
export function astToCanvas(workflow: CanonicalWorkflow): CanvasGraph {
  const nodes: CanvasNode[] = workflow.spec.steps.map((step, idx) => ({
    id: step.id,
    type: "default",
    position: { x: 100 + (idx % 4) * 220, y: 100 + Math.floor(idx / 4) * 160 },
    data: {
      label: step.id,
      nodeType: step.type,
      ref: step.ref,
      tool: step.tool,
      inputs: step.inputs,
      raw: step,
    },
  }));

  const edges: CanvasEdge[] = [];
  for (const step of workflow.spec.steps) {
    for (const dep of step.depends_on ?? []) {
      edges.push({ id: `${dep}->${step.id}`, source: dep, target: step.id });
    }
  }

  return {
    nodes,
    edges,
    meta: {
      apiVersion: workflow.apiVersion,
      kind: workflow.kind,
      metadata: workflow.metadata,
      inputs: workflow.spec.inputs ?? [],
      triggers: workflow.spec.triggers ?? [],
      on_failure: workflow.spec.on_failure ?? [],
      outputs: workflow.spec.outputs ?? [],
    },
  };
}

/** Convert a canvas graph back into the canonical AST. Inverse of astToCanvas. */
export function canvasToAST(graph: CanvasGraph): CanonicalWorkflow {
  const meta = (graph.meta ?? {}) as {
    apiVersion?: CanonicalWorkflow["apiVersion"];
    kind?: CanonicalWorkflow["kind"];
    metadata?: CanonicalWorkflow["metadata"];
    inputs?: CanonicalWorkflow["spec"]["inputs"];
    triggers?: CanonicalWorkflow["spec"]["triggers"];
    on_failure?: CanonicalWorkflow["spec"]["on_failure"];
    outputs?: CanonicalWorkflow["spec"]["outputs"];
  };

  const dependsOnByTarget = new Map<string, string[]>();
  for (const edge of graph.edges) {
    const list = dependsOnByTarget.get(edge.target) ?? [];
    if (!list.includes(edge.source)) list.push(edge.source);
    dependsOnByTarget.set(edge.target, list);
  }

  const steps: CanonicalStep[] = graph.nodes.map((node) => {
    const raw = node.data.raw;
    const merged: CanonicalStep = raw
      ? { ...raw }
      : {
          id: node.id,
          type: node.data.nodeType,
          ref: node.data.ref,
          tool: node.data.tool,
          inputs: node.data.inputs,
        };
    const depends = dependsOnByTarget.get(node.id);
    merged.depends_on = depends && depends.length ? depends : undefined;
    return merged;
  });

  const metadata = meta.metadata ?? { id: "untitled", name: "Untitled", version: "0.1.0" };

  return {
    apiVersion: meta.apiVersion ?? "forge.workflows/v1",
    kind: meta.kind ?? "Workflow",
    metadata,
    spec: {
      inputs: meta.inputs,
      triggers: meta.triggers,
      steps,
      on_failure: meta.on_failure,
      outputs: meta.outputs,
    },
  };
}

/** Convenience: round-trip through the canvas format and back. */
export function roundTrip(workflow: CanonicalWorkflow): CanonicalWorkflow {
  return canvasToAST(astToCanvas(workflow));
}

// ---------------------------------------------------------------------------
// Deprecated aliases. Remove after every call site moves to the canvas names.
// ---------------------------------------------------------------------------

/** @deprecated Renamed to {@link astToCanvas}. */
export const astToFlowise = astToCanvas;

/** @deprecated Renamed to {@link canvasToAST}. */
export const flowiseToAST = canvasToAST;

/** @deprecated Renamed to {@link CanvasGraph}. */
export type FlowiseGraph = CanvasGraph;

/** @deprecated Renamed to {@link CanvasNode}. */
export type FlowiseNode = CanvasNode;

/** @deprecated Renamed to {@link CanvasEdge}. */
export type FlowiseEdge = CanvasEdge;
