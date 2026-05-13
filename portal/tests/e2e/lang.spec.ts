import { expect, test } from "@playwright/test";

test.describe("@lang ES / EN i18n", () => {
  test("toggle persists and updates the html lang attribute", async ({ page }) => {
    await page.goto("/");
    await page.locator(".lang-pill button", { hasText: "EN" }).click();
    await expect(page.locator("html")).toHaveAttribute("lang", "en");
    // KPI label switches to English.
    await expect(page.locator(".kpi .lbl").first()).toContainText(/Runs|Success|p95|Hours/);

    await page.locator(".lang-pill button", { hasText: "ES" }).click();
    await expect(page.locator("html")).toHaveAttribute("lang", "es");
  });
});
