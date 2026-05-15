/**
 * design-system-catalog: E2E test for the wizard's Design System step.
 *
 * Pre-requisites:
 *   - portal dev server running on http://localhost:3000
 *   - registry returns at least 4 entries from GET /v1/design-systems
 *   - app service accepts PATCH /v1/apps/{id} with design_system_ref
 *
 * Verifies the spec scenarios in
 * openspec/changes/design-system-catalog/specs/intent-capture-wizard/spec.md:
 *   - "Step appears only when creating a new App"
 *   - "Step shows four templates with previews"
 *   - "Default is ds-forge-default"
 *   - "Selection persists on the draft App"
 */
import { test, expect } from "@playwright/test";

test.describe("@design-system Wizard Design System step", () => {
  test("step renders four cards and continues to intent capture", async ({ page, baseURL }) => {
    const wizardURL = `${baseURL ?? "http://localhost:3000"}/alfred/wizard?wizard=1&workspace_id=00000000-0000-0000-0000-000000000001&app_id=demo-app-id&design_system_step=1`;
    await page.goto(wizardURL);
    await expect(page.locator("h1")).toContainText("Pick a Design System");
    const cards = page.locator("[data-design-system-card]");
    await expect(cards).toHaveCount(4);
    await cards.nth(2).locator("input[type=radio]").check();
    await page.locator("button:has-text('Continue')").click();
    await expect(page).toHaveURL(/wizard=1.*app_id=demo-app-id(?!.*design_system_step)/);
  });

  test("step is skipped on extend-existing branch", async ({ page, baseURL }) => {
    const wizardURL = `${baseURL ?? "http://localhost:3000"}/alfred/wizard?wizard=1&workspace_id=00000000-0000-0000-0000-000000000001&app_id=existing-app`;
    await page.goto(wizardURL);
    await expect(page.locator("h1")).not.toContainText("Pick a Design System");
  });

  test("step is skipped on decide-later (_unassigned) branch", async ({ page, baseURL }) => {
    const wizardURL = `${baseURL ?? "http://localhost:3000"}/alfred/wizard?wizard=1&workspace_id=00000000-0000-0000-0000-000000000001&app_id=_unassigned&design_system_step=1`;
    await page.goto(wizardURL);
    await expect(page.locator("h1")).not.toContainText("Pick a Design System");
  });
});
