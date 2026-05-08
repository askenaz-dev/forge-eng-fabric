import { expect, test } from "@playwright/test";

test("creates an app from the New App wizard", async ({ page }) => {
  await page.goto("/apps/new");
  await expect(page.getByRole("heading", { name: "New App" })).toBeVisible();
  await expect(page.getByText("go-microservice@1.0.0")).toBeVisible();

  await page.getByRole("button", { name: "Next" }).click();
  await page.getByLabel("Workspace ID").fill("ws-e2e");
  await page.getByLabel("Tenant ID").fill("tn-e2e");
  await page.getByLabel("GitHub org").fill("forge-pilot");
  await page.getByLabel("Repository name").fill("svc-e2e");
  await page.getByLabel("Owners").fill("@platform-engineering,@security");
  await page.getByLabel("Criticality").selectOption("high");
  await page.getByLabel("Data classification").selectOption("internal");

  await page.getByRole("button", { name: "Next" }).click();
  await expect(page.getByText("Preview repository contract")).toBeVisible();
  await expect(page.getByText("forge/lint")).toBeVisible();

  await page.getByRole("button", { name: "Next" }).click();
  await page.getByTestId("confirm-onboarding").click();
  await expect(page).toHaveURL(/\/onboarding\/req-e2e-1$/);
  await expect(page.getByRole("heading", { name: "forge-pilot/svc-e2e" })).toBeVisible();
  await expect(page.getByText("asset.register")).toBeVisible();
});

test("renders template catalog and PR gates", async ({ page }) => {
  await page.goto("/templates");
  await expect(page.getByRole("heading", { name: "Templates" })).toBeVisible();
  await expect(page.getByText("go-microservice")).toBeVisible();

  await page.goto("/pr-gates?repo=forge-pilot/svc-e2e");
  await expect(page.getByRole("heading", { name: "PR gates" })).toBeVisible();
  await expect(page.getByText("golangci-lint")).toBeVisible();
  await expect(page.getByText("open logs").first()).toBeVisible();
});
