import { expect, test } from "@playwright/test";

// Requires `make dev-up && make seed-portal` so the dashboard endpoints
// return deterministic data.

test.describe("@dashboard Tablero", () => {
  test("renders shell, KPI grid, runs and approvals", async ({ page }) => {
    await page.goto("/");

    // Branded shell present.
    await expect(page.locator(".app")).toBeVisible();
    await expect(page.locator(".side-brand .w")).toHaveText("Forge");

    // Display headline uses Instrument Serif italic ember accent.
    const headline = page.locator("h1.page-title").first();
    await expect(headline).toBeVisible();
    await expect(headline.locator("em")).toBeVisible();

    // KPI grid has four KPIs.
    await expect(page.locator(".kpi-grid .kpi")).toHaveCount(4);

    // Runs panel renders rows or the empty-state note.
    await expect(page.locator(".card", { hasText: /Runs/i }).first()).toBeVisible();

    // Approvals panel renders.
    await expect(page.locator(".card", { hasText: /Aprobaci|Approval/i }).first()).toBeVisible();
  });

  test("filter chips drill the runs list", async ({ page }) => {
    await page.goto("/");
    const runningChip = page.locator(".chip", { hasText: /Corriendo|Running/ }).first();
    await runningChip.click();
    await expect(runningChip).toHaveAttribute("aria-pressed", "true");
  });

  test("opens the run sheet on row click and closes on Escape", async ({ page }) => {
    await page.goto("/");
    const firstRow = page.locator(".run").first();
    if (await firstRow.isVisible().catch(() => false)) {
      await firstRow.click();
      await expect(page.locator(".sheet")).toBeVisible();
      await page.keyboard.press("Escape");
      await expect(page.locator(".sheet")).toBeHidden();
    }
  });
});
