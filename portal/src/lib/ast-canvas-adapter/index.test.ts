import { describe, it, expect } from "vitest";
import {
  astToCanvas,
  canvasToAST,
  roundTrip,
  type CanonicalWorkflow,
  CANONICAL_STEP_TYPES,
  CANONICAL_TRIGGER_TYPES,
  DEPRECATED_STEP_TYPES,
  isLlmStep,
} from "./index";

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
    inputs: [{ name: "ticket", type: "string", required: true }],
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

describe("ast-canvas-adapter", () => {
  it("converts AST → canvas → AST without losing semantics", () => {
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

  it("emits one canvas node per canonical step", () => {
    const graph = astToCanvas(sample);
    expect(graph.nodes.length).toBe(sample.spec.steps.length);
    const expectedEdges = sample.spec.steps.reduce(
      (n, s) => n + (s.depends_on?.length ?? 0),
      0,
    );
    expect(graph.edges.length).toBe(expectedEdges);
  });

  it("re-derives depends_on from edges, ignoring raw.depends_on", () => {
    const graph = astToCanvas(sample);
    graph.edges = graph.edges.filter((e) => !(e.source === "approval" && e.target === "open-pr"));
    const back = canvasToAST(graph);
    const openPr = back.spec.steps.find((s) => s.id === "open-pr")!;
    expect(openPr.depends_on).toBeUndefined();
  });

  it("preserves active_surface across the round trip (active-registry-gateways §7.5)", () => {
    const pinned: CanonicalWorkflow = {
      ...sample,
      spec: {
        ...sample.spec,
        steps: sample.spec.steps.map((s) =>
          s.id === "open-pr"
            ? {
                ...s,
                active_surface: {
                  family: "mcp",
                  endpoint: "/v1/gw/mcp/registry:mcp/github@write",
                },
              }
            : s,
        ),
      },
    };
    const back = roundTrip(pinned);
    const openPr = back.spec.steps.find((s) => s.id === "open-pr")!;
    expect(openPr.active_surface).toBeDefined();
    expect(openPr.active_surface?.family).toBe("mcp");
    expect(openPr.active_surface?.endpoint).toBe("/v1/gw/mcp/registry:mcp/github@write");
  });

  it("preserves triggers block across the round trip", () => {
    const withTrigger: CanonicalWorkflow = {
      ...sample,
      spec: {
        ...sample.spec,
        triggers: [
          {
            id: "support-mail",
            type: "email-inbound",
            config: { mailbox_ref: "ws:mailbox:support", filter: { subject_contains: "[urgent]" } },
            outputs: { subject: "string", from: "string", body: "string" },
            concurrency: "queue",
          },
        ],
      },
    };
    const back = roundTrip(withTrigger);
    expect(back.spec.triggers).toBeDefined();
    expect(back.spec.triggers).toHaveLength(1);
    expect(back.spec.triggers?.[0].id).toBe("support-mail");
    expect(back.spec.triggers?.[0].type).toBe("email-inbound");
    expect(back.spec.triggers?.[0].outputs?.subject).toBe("string");
  });

  it("preserves LLM step fields across the round trip", () => {
    const withLlm: CanonicalWorkflow = {
      ...sample,
      spec: {
        ...sample.spec,
        steps: [
          {
            id: "classify",
            type: "llm",
            prompt_template: "registry:prompt/sdlc-product/email-classify@1.3.0",
            model: {
              ref: "gateway:model/claude-opus-4-7@latest-stable",
              overrides: { temperature: 0.2, max_tokens: 1024 },
            },
            tools: ["registry:mcp/email-tools@2.0.0"],
            max_tool_calls: 5,
            outputs_schema: { category: "string", draft: "string", confidence: "number" },
          },
        ],
      },
    };
    const back = roundTrip(withLlm);
    const llm = back.spec.steps[0];
    expect(isLlmStep(llm)).toBe(true);
    expect(llm.prompt_template).toBe("registry:prompt/sdlc-product/email-classify@1.3.0");
    expect(llm.model?.ref).toBe("gateway:model/claude-opus-4-7@latest-stable");
    expect(llm.model?.overrides?.temperature).toBe(0.2);
    expect(llm.tools).toEqual(["registry:mcp/email-tools@2.0.0"]);
    expect(llm.max_tool_calls).toBe(5);
    expect(llm.outputs_schema?.confidence).toBe("number");
  });

  it("declares a catalog that does not list retry as a step type", () => {
    // Retry is a per-step policy (step.retries), not a node type. The
    // legacy palette had it; the canonical catalog does not.
    expect((CANONICAL_STEP_TYPES as readonly string[]).includes("retry")).toBe(false);
  });

  it("includes the new node types added in catalog reconciliation (D8)", () => {
    expect(CANONICAL_STEP_TYPES).toContain("llm");
    expect(CANONICAL_STEP_TYPES).toContain("agent");
    expect(CANONICAL_STEP_TYPES).toContain("prompt-template");
    expect(CANONICAL_STEP_TYPES).toContain("sub-workflow");
    expect(CANONICAL_STEP_TYPES).toContain("custom");
    expect(CANONICAL_STEP_TYPES).toContain("notification-action");
  });

  it("exposes deprecated step types with their replacements", () => {
    expect(DEPRECATED_STEP_TYPES.prompt).toBe("prompt-template");
    expect(DEPRECATED_STEP_TYPES["event-trigger"]).toBe("");
  });

  it("includes the canonical trigger types", () => {
    expect(CANONICAL_TRIGGER_TYPES).toEqual([
      "manual",
      "cron",
      "webhook-in",
      "event-bus",
      "email-inbound",
    ]);
  });
});
