import AxeBuilder from "@axe-core/playwright";
import { expect, test } from "@playwright/test";

const PAGES = [
  { path: "/",            label: "dashboard" },
  { path: "/approvals",   label: "approvals" },
  { path: "/onboarding",  label: "onboarding" },
  { path: "/openspecs",   label: "openspecs" },
  { path: "/deployments", label: "deployments" },
];

test.describe("@a11y axe scan", () => {
  for (const { path, label } of PAGES) {
    for (const theme of ["light", "dark"]) {
      test(`${label} (${theme})`, async ({ page }) => {
        await page.addInitScript((t) => {
          localStorage.setItem("forge_theme", t);
        }, theme);
        await page.goto(path);
        const results = await new AxeBuilder({ page })
          .withTags(["wcag2a", "wcag2aa"])
          .analyze();
        const serious = results.violations.filter((v) => v.impact === "serious" || v.impact === "critical");
        expect(serious, JSON.stringify(serious, null, 2)).toEqual([]);
      });
    }
  }
});
