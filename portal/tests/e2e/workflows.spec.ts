import { expect, test } from "@playwright/test";

test("renders workflows module with editor scaffolding", async ({ page }) => {
  await page.goto("/workflows");
  await expect(page.getByRole("heading", { name: "Workflows" })).toBeVisible();
  await expect(
    page.getByText(
      "Compose Skills, MCPs, Prompts, gates, branches and human-in-the-loop steps. Versioned with",
    ),
  ).toBeVisible();
  await expect(page.getByPlaceholder("Tenant ID")).toBeVisible();
  await expect(page.getByPlaceholder("Workspace ID")).toBeVisible();
});

test("renders marketplace with browse and install affordances", async ({ page }) => {
  await page.goto("/marketplace");
  await expect(page.getByRole("heading", { name: "Marketplace" })).toBeVisible();
  await expect(page.getByText("Browse workflows shared in your Tenant. Install pins to an exact version.")).toBeVisible();
});
