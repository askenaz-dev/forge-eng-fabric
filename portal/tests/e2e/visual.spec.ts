import { expect, test } from "@playwright/test";

// Visual baselines per page × theme × lang.
// Run with `pnpm --filter @forge/portal test:visual` and approve diffs via
// `playwright test --update-snapshots`.

const PAGES = [
  { path: "/",            name: "dashboard" },
  { path: "/approvals",   name: "approvals" },
  { path: "/onboarding",  name: "onboarding" },
  { path: "/openspecs",   name: "openspecs" },
  { path: "/deployments", name: "deployments" },
];

const THEMES: Array<"light" | "dark"> = ["light", "dark"];
const LANGS: Array<"es" | "en"> = ["es", "en"];

test.describe("@visual baselines", () => {
  for (const { path, name } of PAGES) {
    for (const theme of THEMES) {
      for (const lang of LANGS) {
        test(`${name} ${theme} ${lang}`, async ({ page }) => {
          await page.addInitScript(
            ({ t, l }) => {
              localStorage.setItem("forge_theme", t);
              localStorage.setItem("forge_lang", l);
              document.cookie = `forge_prefs=${encodeURIComponent(JSON.stringify({ theme: t, lang: l }))};path=/`;
            },
            { t: theme, l: lang },
          );
          await page.goto(path);
          await page.waitForLoadState("networkidle");
          await expect(page).toHaveScreenshot(`${name}-${theme}-${lang}.png`, {
            fullPage: true,
            maxDiffPixels: 200,
            animations: "disabled",
          });
        });
      }
    }
  }
});
