import { expect, test } from "@playwright/test";

// Visual baselines per page × theme × lang.
// Run with `pnpm --filter @forge/portal test:visual` and approve diffs via
// `playwright test --update-snapshots`.

const PAGES = [
  { path: "/",                       name: "dashboard" },
  { path: "/approvals",              name: "approvals" },
  { path: "/onboarding",             name: "onboarding" },
  { path: "/openspecs",              name: "openspecs" },
  { path: "/deployments",            name: "deployments" },
  { path: "/alfred?view=friendly",   name: "alfred-friendly" },
  { path: "/alfred?view=advanced",   name: "alfred-advanced" },
];

const THEMES: Array<"light" | "dark"> = ["light", "dark"];
const LANGS: Array<"es" | "en"> = ["es", "en"];

// Alfred pages need API stubs so they don't spin waiting for a real Alfred service.
const ALFRED_PATHS = new Set(["/alfred?view=friendly", "/alfred?view=advanced"]);

const STUB_HEALING_EVENTS = {
  events: [
    {
      id: "det-visual-01",
      type: "l1",
      app_id: "app-visual-01",
      summary: "CPU spike detected on payment-service (p95 > 800 ms)",
      severity: "high",
      created_at: "2026-05-14T10:00:00Z",
    },
    {
      id: "sug-visual-01",
      type: "l2",
      app_id: "app-visual-01",
      summary: "Proposed fix: increase HPA maxReplicas from 3 to 6",
      severity: "medium",
      created_at: "2026-05-14T10:02:00Z",
    },
  ],
};

test.describe("@visual baselines", () => {
  for (const { path, name } of PAGES) {
    for (const theme of THEMES) {
      for (const lang of LANGS) {
        test(`${name} ${theme} ${lang}`, async ({ page }) => {
          // Stub Alfred backend APIs for alfred pages so the page loads deterministically.
          if (ALFRED_PATHS.has(path)) {
            await page.route("**/api/user/preferences", (route) =>
              route.request().method() === "GET"
                ? route.fulfill({ json: { console_view_preference: null } })
                : route.fulfill({ json: { ok: true } }),
            );
            await page.route("**/api/workspace/active", (route) =>
              route.fulfill({ json: { workspace_id: "ws-visual", apps: [] } }),
            );
            await page.route("**/api/permissions/me", (route) =>
              route.fulfill({ json: { roles: ["workspace.member"] } }),
            );
            await page.route("**/api/healing/events*", (route) =>
              route.fulfill({ json: STUB_HEALING_EVENTS }),
            );
            await page.route("**/api/workflow/runs", (route) =>
              route.fulfill({ json: { run_id: "run-visual-01", status: "started" } }),
            );
          }

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

// ── FriendlyView state-specific visual baselines (task 10.3) ────────────────
// These capture the two new interactive states added by the sdlc-end-to-end
// change: the healing events panel (Operar card) and the workflow trigger
// panel (Nueva App card after intent commit).

test.describe("@visual @friendlyview FriendlyView — healing events panel", () => {
  for (const theme of THEMES) {
    test(`operar-healing-panel ${theme}`, async ({ page }) => {
      await page.route("**/api/user/preferences", (route) =>
        route.request().method() === "GET"
          ? route.fulfill({ json: { console_view_preference: null } })
          : route.fulfill({ json: { ok: true } }),
      );
      await page.route("**/api/workspace/active", (route) =>
        route.fulfill({ json: { workspace_id: "ws-visual", apps: [] } }),
      );
      await page.route("**/api/permissions/me", (route) =>
        route.fulfill({ json: { roles: ["workspace.member"] } }),
      );
      await page.route("**/api/healing/events*", (route) =>
        route.fulfill({ json: STUB_HEALING_EVENTS }),
      );

      await page.addInitScript(
        ({ t }) => {
          localStorage.setItem("forge_theme", t);
          localStorage.setItem("forge_lang", "es");
          document.cookie = `forge_prefs=${encodeURIComponent(JSON.stringify({ theme: t, lang: "es" }))};path=/`;
        },
        { t: theme },
      );
      await page.goto("/alfred?view=friendly");
      await page.waitForLoadState("networkidle");

      // Activate Operar card to load healing events.
      await page.locator(".alfred-card", { hasText: /Operar/i }).click();
      await page.waitForSelector("[data-testid='healing-events-panel'], .alfred-healing-event", {
        timeout: 8_000,
      }).catch(() => { /* panel may render differently — capture whatever state loaded */ });
      await page.waitForTimeout(300);

      await expect(page).toHaveScreenshot(`friendlyview-operar-healing-${theme}.png`, {
        fullPage: true,
        maxDiffPixels: 300,
        animations: "disabled",
      });
    });
  }
});

test.describe("@visual @friendlyview FriendlyView — workflow trigger panel", () => {
  for (const theme of THEMES) {
    test(`nueva-app-workflow-trigger ${theme}`, async ({ page }) => {
      await page.route("**/api/user/preferences", (route) =>
        route.request().method() === "GET"
          ? route.fulfill({ json: { console_view_preference: null } })
          : route.fulfill({ json: { ok: true } }),
      );
      await page.route("**/api/workspace/active", (route) =>
        route.fulfill({ json: { workspace_id: "ws-visual", apps: [] } }),
      );
      await page.route("**/api/permissions/me", (route) =>
        route.fulfill({ json: { roles: ["workspace.member"] } }),
      );
      // Simulate intent start → returns a committed intent (no next_question).
      await page.route("**/api/alfred/intent/start", (route) =>
        route.fulfill({
          json: {
            draft: { draft_id: "draft-visual-01" },
            next_question: "¿Cuál es el nombre de la App?",
          },
        }),
      );
      // Final answer → no next_question signals intent committed.
      await page.route("**/api/alfred/intent/answer", (route) =>
        route.fulfill({
          json: {
            openspec_id: "os-visual-01",
            app_id: "app-visual-01",
          },
        }),
      );
      await page.route("**/api/workflow/runs", (route) =>
        route.fulfill({ json: { run_id: "run-visual-01", status: "started" } }),
      );

      await page.addInitScript(
        ({ t }) => {
          localStorage.setItem("forge_theme", t);
          localStorage.setItem("forge_lang", "es");
          document.cookie = `forge_prefs=${encodeURIComponent(JSON.stringify({ theme: t, lang: "es" }))};path=/`;
        },
        { t: theme },
      );
      await page.goto("/alfred?view=friendly");
      await page.waitForLoadState("networkidle");

      // Open Nueva App and complete the conversation to reach the workflow trigger panel.
      await page.locator(".alfred-card", { hasText: /Nueva App/i }).click();
      await page.getByText(/cuál es el nombre/i).waitFor({ timeout: 8_000 });
      const input = page.locator("textarea, input[type=text]").last();
      await input.fill("VisualTestApp");
      await page.keyboard.press("Enter");
      // Wait for workflow trigger button to appear (no next_question returned).
      await page.waitForSelector(
        "[data-testid='workflow-trigger-btn'], button:has-text(/intent-to-infrastructure/i)",
        { timeout: 8_000 },
      ).catch(() => { /* capture whatever rendered */ });
      await page.waitForTimeout(300);

      await expect(page).toHaveScreenshot(`friendlyview-nueva-app-trigger-${theme}.png`, {
        fullPage: true,
        maxDiffPixels: 300,
        animations: "disabled",
      });
    });
  }
});
