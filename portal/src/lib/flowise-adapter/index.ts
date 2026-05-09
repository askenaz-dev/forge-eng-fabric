/**
 * Flowise ↔ canonical AST adapter.
 *
 * Translates between Flowise's native node format (a graph of node/edge JSON
 * objects emitted by `react-flow`) and the platform's canonical workflow AST
 * defined in `pkg/workflow/ast`. Round-trip identity is required for task 5.7
 * (export-to-DSL parity).
 *
 * The adapter is intentionally small: any change to the canonical AST that
 * affects user-visible workflows is a spec change, and changes here MUST be
 * paired with regenerated test fixtures.
 */

export type CanonicalStepType =
  | "llm"
  | "mcp"
  | "skill"
  | "agent"
  | "prompt-template"
  | "human-in-the-loop"
  | "branch"
  | "loop"
  | "retry"
  | "eval"
  | "webhook"
  | "github-action"
  | "deploy-action"
  | "approval-action"
  | "notification-action";

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
    steps: CanonicalStep[];
    on_failure?: CanonicalStep[];
    outputs?: { name: string; from: string }[];
  };
}

export interface FlowiseGraph {
  nodes: FlowiseNode[];
  edges: FlowiseEdge[];
  meta?: Record<string, unknown>;
}

export interface FlowiseNode {
  id: string;
  data: { label?: string; nodeType: CanonicalStepType; ref?: string; tool?: string; inputs?: Record<string, unknown>; raw?: CanonicalStep };
  position: { x: number; y: number };
  type?: string;
}

export interface FlowiseEdge {
  id: string;
  source: string;
  target: string;
}

/** Convert a canonical workflow AST into a Flowise graph for the editor. */
export function astToFlowise(workflow: CanonicalWorkflow): FlowiseGraph {
  const nodes: FlowiseNode[] = workflow.spec.steps.map((step, idx) => ({
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

  const edges: FlowiseEdge[] = [];
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
      on_failure: workflow.spec.on_failure ?? [],
      outputs: workflow.spec.outputs ?? [],
    },
  };
}

/** Convert a Flowise graph back into the canonical AST. Inverse of astToFlowise. */
export function flowiseToAST(graph: FlowiseGraph): CanonicalWorkflow {
  const meta = (graph.meta ?? {}) as {
    apiVersion?: CanonicalWorkflow["apiVersion"];
    kind?: CanonicalWorkflow["kind"];
    metadata?: CanonicalWorkflow["metadata"];
    inputs?: CanonicalWorkflow["spec"]["inputs"];
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
      steps,
      on_failure: meta.on_failure,
      outputs: meta.outputs,
    },
  };
}

/** Convenience: round-trip through Flowise format and back. Used by task 5.7's
 * round-trip test. */
export function roundTrip(workflow: CanonicalWorkflow): CanonicalWorkflow {
  return flowiseToAST(astToFlowise(workflow));
}
