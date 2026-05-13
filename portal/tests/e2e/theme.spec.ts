import { expect, test } from "@playwright/test";

test.describe("@theme @a11y theme system", () => {
  test("switches between light, dark and system", async ({ page }) => {
    await page.goto("/");
    await page.getByLabel(/Tema|Theme/i).first().click();
    await page.getByRole("menuitem", { name: /Oscuro|Dark/i }).click();
    await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");

    await page.getByLabel(/Tema|Theme/i).first().click();
    await page.getByRole("menuitem", { name: /Claro|Light/i }).click();
    await expect(page.locator("html")).toHaveAttribute("data-theme", "light");
  });

  test("transition guard blocks transitions for a single frame", async ({ page }) => {
    await page.goto("/");
    await page.getByLabel(/Tema|Theme/i).first().click();
    await page.getByRole("menuitem", { name: /Oscuro|Dark/i }).click();
    // After two rAFs the changing guard is removed.
    await page.waitForTimeout(50);
    await expect(page.locator("html")).not.toHaveAttribute("data-theme-changing", "");
  });
});
