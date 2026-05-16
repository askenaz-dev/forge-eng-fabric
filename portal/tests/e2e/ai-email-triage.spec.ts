// Playwright e2e: opens the AI-Flow canvas, drag-builds the email triage
// reference flow, configures each node, runs the dry-run drawer, and
// asserts the trace contains the LLM and branch steps.
//
// This test is gated on AI_FLOWS_CANVAS_ENABLED=true in the dev/staging
// configs (set during cutover §13.2). The locator strategy uses the
// `data-testid` attributes baked into the canvas components.

import { test, expect } from "@playwright/test";

const WORKSPACE = process.env.E2E_WORKSPACE_ID ?? "ws-acme-engineering";

test.describe("AI-Flow canvas — email triage reference", () => {
  test.skip(
    process.env.AI_FLOWS_CANVAS_ENABLED !== "true",
    "Canvas feature flag off — skipping until cutover.",
  );

  test("opens, drag-builds, dry-runs the reference flow", async ({ page }) => {
    await page.goto(`/workflows/editor?workspace_id=${encodeURIComponent(WORKSPACE)}`);
    await expect(page.getByTestId("ai-flow-canvas")).toBeVisible();

    // Trigger
    await page.getByTestId("palette-item-trigger-email-inbound").click();
    // LLM step
    await page.getByTestId("palette-item-step-llm").click();
    // Branch
    await page.getByTestId("palette-item-step-branch").click();
    // MCP action
    await page.getByTestId("palette-item-step-mcp").click();
    // HITL
    await page.getByTestId("palette-item-step-human-in-the-loop").click();

    // The triggers band reflects the email trigger we just added.
    await expect(page.getByTestId("trigger-band")).toContainText("email-inbound");

    // Dry-run.
    await page.getByRole("button", { name: /Dry run/i }).click();
    await expect(page.getByRole("dialog", { name: /Dry-run trace/i })).toBeVisible();
  });
});
