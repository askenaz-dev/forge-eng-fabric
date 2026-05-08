import { expect, test } from "@playwright/test";

// E2E coverage for Phase 6 autonomous-ops portal modules.
//
// These specs run against synthetic incidents (the detection service emits
// incidents with `synthetic=true`) so a live healing path is never executed
// against production. The fixture services in docker-compose seed each test
// with a deterministic state.

test("incidents module shows the incident timeline", async ({ page }) => {
  await page.goto("/incidents");
  await expect(page.getByRole("heading", { name: "Incidents" })).toBeVisible();
  await expect(page.getByText("Detection → diagnosis → healing → postmortem")).toBeVisible();
  await expect(page.getByTestId("incident-list")).toBeVisible();
});

test("evolution inbox renders proposals with autonomous-loop badges and stats", async ({ page }) => {
  await page.goto("/evolution");
  await expect(page.getByRole("heading", { name: "Evolution Inbox" })).toBeVisible();
  await expect(page.getByTestId("evolution-stats")).toBeVisible();
  // Stats panel contains the four counters.
  await expect(page.getByTestId("stat-total")).toBeVisible();
  await expect(page.getByTestId("stat-inbox")).toBeVisible();
  await expect(page.getByTestId("stat-converted")).toBeVisible();
  await expect(page.getByTestId("stat-rejected")).toBeVisible();
});

test("finops recommendations module renders savings summary", async ({ page }) => {
  await page.goto("/finops-recommendations");
  await expect(page.getByRole("heading", { name: "FinOps Recommendations" })).toBeVisible();
  await expect(page.getByTestId("finops-summary")).toBeVisible();
});

test("kill-switch shows current state and toggle controls", async ({ page }) => {
  await page.goto("/kill-switch");
  await expect(page.getByRole("heading", { name: "Kill switch" })).toBeVisible();
  await expect(page.getByTestId("kill-switch-state")).toBeVisible();
  await expect(page.getByTestId("kill-switch-status")).toBeVisible();
  // The action button label depends on current state — assert one is present.
  const activate = page.getByTestId("kill-switch-activate");
  const deactivate = page.getByTestId("kill-switch-deactivate");
  await expect(activate.or(deactivate)).toBeVisible();
});
