import { expect, test } from "@playwright/test";

test("covers runtime onboarding and preflight actions", async ({ page }) => {
  await page.goto("/runtimes");
  await expect(page.getByRole("heading", { name: "Deploy targets" })).toBeVisible();
  await expect(page.getByText("local-minikube")).toBeVisible();
  await expect(page.getByText("pilot-prod")).toBeVisible();
  await expect(page.getByText("byo").first()).toBeVisible();
  await expect(page.getByRole("button", { name: "Register BYO runtime" })).toBeVisible();
  await expect(page.getByRole("button", { name: "Run preflight" }).first()).toBeVisible();
});

test("covers deployment history, live stages, and rollback confirmation", async ({ page }) => {
  await page.goto("/deployments");
  await expect(page.getByRole("heading", { name: "Release history and live status" })).toBeVisible();
  await expect(page.getByText("worker-bar")).toBeVisible();
  await expect(page.getByText("rt-dev-1")).toBeVisible();
  await expect(page.getByText("verify").first()).toBeVisible();

  await page.getByText("Rollback confirmation").first().click();
  await page.getByPlaceholder("Explain customer impact and recovery intent").first().fill("E2E rollback validation for failed deploy restore");
  await expect(page.getByRole("button", { name: "Confirm rollback" }).first()).toBeVisible();
});

test("covers drift remediation proposal workflow", async ({ page }) => {
  await page.goto("/drift");
  await expect(page.getByRole("heading", { name: "Terraform drift findings" })).toBeVisible();
  await expect(page.getByText("google_project_iam_binding.deploy")).toBeVisible();
  await expect(page.getByText("high", { exact: true })).toBeVisible();
  await expect(page.getByRole("button", { name: "Propose PR" }).first()).toBeVisible();
});
