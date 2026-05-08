import { expect, test } from "@playwright/test";

test("renders SDLC initiatives, phase progression, traceability, and costs", async ({ page }) => {
  await page.goto("/initiatives");
  await expect(page.getByRole("heading", { name: "SDLC phase progression" })).toBeVisible();
  await expect(page.getByText("Checkout reliability uplift")).toBeVisible();
  await expect(page.getByRole("heading", { name: "Phase progression", exact: true })).toBeVisible();
  await expect(page.getByText("SAST finding SEC-77 requires triage")).toBeVisible();
  await expect(page.getByText("Traceability graph")).toBeVisible();
  await expect(page.getByText("Costs by initiative")).toBeVisible();
});
