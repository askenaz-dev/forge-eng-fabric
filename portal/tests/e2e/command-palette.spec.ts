import { expect, test } from "@playwright/test";

test.describe("@palette command palette", () => {
  test("⌘K opens, Escape closes", async ({ page }) => {
    await page.goto("/");
    await page.keyboard.press(process.platform === "darwin" ? "Meta+K" : "Control+K");
    await expect(page.locator(".cmdk-root")).toBeVisible();
    await page.keyboard.press("Escape");
    await expect(page.locator(".cmdk-root")).toBeHidden();
  });

  test("clicking the top-bar search opens the palette", async ({ page }) => {
    await page.goto("/");
    await page.locator(".top-search").click();
    await expect(page.locator(".cmdk-root")).toBeVisible();
  });

  test("action subcommand switches the theme", async ({ page }) => {
    await page.goto("/");
    await page.keyboard.press(process.platform === "darwin" ? "Meta+K" : "Control+K");
    const input = page.locator(".cmdk-input");
    await input.fill("theme");
    await page.locator(".cmdk-item", { hasText: /Oscuro|dark theme/i }).first().click();
    await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
  });
});
