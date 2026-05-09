import { describe, it, expect } from "vitest";
import { astToFlowise, flowiseToAST, roundTrip, type CanonicalWorkflow } from "./index";

const sample: CanonicalWorkflow = {
  apiVersion: "forge.workflows/v1",
  kind: "Workflow",
  metadata: {
    id: "demo-workflow",
    name: "Demo",
    version: "1.0.0",
    visibility: "workspace",
    criticality: "medium",
  },
  spec: {
    inputs: [
      { name: "ticket", type: "string", required: true },
    ],
    steps: [
      {
        id: "refine",
        type: "skill",
        ref: "registry:skill/sdlc-product/refine-user-story@1.2.0",
        inputs: { story: "$inputs.ticket" },
        retries: { max: 3, backoff: "exponential" },
        timeout: "60s",
      },
      {
        id: "approval",
        type: "human-in-the-loop",
        approver_role: "product-owner",
        on_timeout: "escalate",
        escalation_role: "engineering-manager",
        depends_on: ["refine"],
      },
      {
        id: "open-pr",
        type: "mcp",
        ref: "registry:mcp/github@write",
        tool: "create_pr",
        depends_on: ["approval"],
        inputs: { title: "$steps.refine.outputs.refined" },
      },
    ],
    outputs: [{ name: "pr_url", from: "$steps.open-pr.outputs.url" }],
  },
};

describe("flowise-adapter", () => {
  it("converts AST → Flowise → AST without losing semantics", () => {
    const back = roundTrip(sample);
    expect(back.metadata.id).toBe(sample.metadata.id);
    expect(back.spec.steps.length).toBe(sample.spec.steps.length);

    for (let i = 0; i < sample.spec.steps.length; i += 1) {
      const before = sample.spec.steps[i];
      const after = back.spec.steps[i];
      expect(after.id).toBe(before.id);
      expect(after.type).toBe(before.type);
      expect(after.ref).toBe(before.ref);
      expect(after.tool).toBe(before.tool);
      expect(after.depends_on ?? []).toEqual(before.depends_on ?? []);
    }
  });

  it("emits one Flowise node per canonical step", () => {
    const graph = astToFlowise(sample);
    expect(graph.nodes.length).toBe(sample.spec.steps.length);
    const expectedEdges = sample.spec.steps.reduce(
      (n, s) => n + (s.depends_on?.length ?? 0),
      0,
    );
    expect(graph.edges.length).toBe(expectedEdges);
  });

  it("re-derives depends_on from edges, ignoring raw.depends_on", () => {
    const graph = astToFlowise(sample);
    // Mutate the graph as the editor would: remove the approval→open-pr edge
    graph.edges = graph.edges.filter((e) => !(e.source === "approval" && e.target === "open-pr"));
    const back = flowiseToAST(graph);
    const openPr = back.spec.steps.find((s) => s.id === "open-pr")!;
    expect(openPr.depends_on).toBeUndefined();
  });
});
