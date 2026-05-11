import { expect, test } from "@playwright/test";

test("renders SDLC initiatives from live services only after a Workspace is loaded", async ({ page }) => {
  await page.goto("/initiatives");
  await expect(page.getByRole("heading", { name: "SDLC phase progression" })).toBeVisible();
  await expect(page.getByText("Load a Workspace")).toBeVisible();
});
