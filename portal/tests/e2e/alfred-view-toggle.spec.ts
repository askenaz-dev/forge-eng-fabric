import { expect, test } from "@playwright/test";

// ── View resolution — role-based defaults ────────────────────────────────────

test.describe("@alfred-toggle Alfred view toggle — role-based defaults", () => {
  test("workspace.member defaults to Friendly view on first sign-in", async ({ page }) => {
    // No persisted preference.
    await page.route("**/api/user/preferences", (route) => {
      if (route.request().method() === "GET") {
        return route.fulfill({ json: { console_view_preference: null } });
      }
      return route.fulfill({ json: { ok: true } });
    });
    // Member role only.
    await page.route("**/api/permissions/me", (route) =>
      route.fulfill({ json: { roles: ["workspace.member"] } }),
    );
    await page.route("**/api/workspace/active", (route) =>
      route.fulfill({ json: { workspace_id: "ws-1", apps: [] } }),
    );

    await page.goto("/alfred");
    await page.waitForLoadState("networkidle");

    // Friendly landing cards should appear.
    await expect(page.locator(".alfred-card-title", { hasText: /Nueva App/i })).toBeVisible({ timeout: 8_000 });
  });

  test("workspace.developer defaults to Advanced view on first sign-in", async ({ page }) => {
    await page.route("**/api/user/preferences", (route) => {
      if (route.request().method() === "GET") {
        return route.fulfill({ json: { console_view_preference: null } });
      }
      return route.fulfill({ json: { ok: true } });
    });
    // Developer role.
    await page.route("**/api/permissions/me", (route) =>
      route.fulfill({ json: { roles: ["workspace.developer"] } }),
    );
    await page.route("**/api/workspace/active", (route) =>
      route.fulfill({ json: { workspace_id: "ws-1", apps: [] } }),
    );

    await page.goto("/alfred");
    await page.waitForLoadState("networkidle");

    // Advanced view should render (no friendly cards).
    await expect(page.locator(".alfred-card-title", { hasText: /Nueva App/i })).toBeHidden({ timeout: 8_000 });
    await expect(page.locator(".alfred-advanced-root, .alfred-advanced-input")).toBeVisible({ timeout: 8_000 });
  });
});

// ── Toggle persistence ───────────────────────────────────────────────────────

test.describe("@alfred-toggle Alfred view toggle — persistence", () => {
  test("persisted preference takes precedence over role-based default", async ({ page }) => {
    // Member role but has persisted 'advanced'.
    await page.route("**/api/user/preferences", (route) => {
      if (route.request().method() === "GET") {
        return route.fulfill({ json: { console_view_preference: "advanced" } });
      }
      return route.fulfill({ json: { ok: true } });
    });
    await page.route("**/api/permissions/me", (route) =>
      route.fulfill({ json: { roles: ["workspace.member"] } }),
    );
    await page.route("**/api/workspace/active", (route) =>
      route.fulfill({ json: { workspace_id: "ws-1", apps: [] } }),
    );

    await page.goto("/alfred");
    await page.waitForLoadState("networkidle");

    // Advanced view despite member role (persisted preference wins).
    await expect(page.locator(".alfred-card-title", { hasText: /Nueva App/i })).toBeHidden({ timeout: 8_000 });
  });

  test("?view= query param overrides persisted preference", async ({ page }) => {
    // Has persisted 'advanced', but ?view=friendly forces Friendly.
    await page.route("**/api/user/preferences", (route) => {
      if (route.request().method() === "GET") {
        return route.fulfill({ json: { console_view_preference: "advanced" } });
      }
      return route.fulfill({ json: { ok: true } });
    });
    await page.route("**/api/workspace/active", (route) =>
      route.fulfill({ json: { workspace_id: "ws-1", apps: [] } }),
    );

    await page.goto("/alfred?view=friendly");
    await page.waitForLoadState("networkidle");

    await expect(page.locator(".alfred-card-title", { hasText: /Nueva App/i })).toBeVisible({ timeout: 8_000 });
  });
});

// ── Tenant override (via ?view= in e2e) ──────────────────────────────────────

test.describe("@alfred-toggle Alfred view toggle — tenant override", () => {
  test("force friendly via query param works for developer role", async ({ page }) => {
    await page.route("**/api/user/preferences", (route) => {
      if (route.request().method() === "GET") {
        return route.fulfill({ json: { console_view_preference: null } });
      }
      return route.fulfill({ json: { ok: true } });
    });
    await page.route("**/api/permissions/me", (route) =>
      route.fulfill({ json: { roles: ["workspace.developer"] } }),
    );
    await page.route("**/api/workspace/active", (route) =>
      route.fulfill({ json: { workspace_id: "ws-1", apps: [] } }),
    );

    // Simulate tenant override: a developer is sent to friendly.
    await page.goto("/alfred?view=friendly");
    await page.waitForLoadState("networkidle");

    await expect(page.locator(".alfred-card-title", { hasText: /Nueva App/i })).toBeVisible({ timeout: 8_000 });
  });
});

// ── Session-level switch ─────────────────────────────────────────────────────

test.describe("@alfred-toggle Alfred view toggle — session-level switch", () => {
  test("switch-to-developer-mode link navigates to advanced view", async ({ page }) => {
    await page.route("**/api/user/preferences", (route) => {
      if (route.request().method() === "GET") {
        return route.fulfill({ json: { console_view_preference: null } });
      }
      return route.fulfill({ json: { ok: true } });
    });
    await page.route("**/api/permissions/me", (route) =>
      route.fulfill({ json: { roles: ["workspace.member"] } }),
    );
    await page.route("**/api/workspace/active", (route) =>
      route.fulfill({ json: { workspace_id: "ws-1", apps: [] } }),
    );

    await page.goto("/alfred?view=friendly");
    await page.waitForLoadState("networkidle");

    // Click "Cambiar a modo desarrollador →" link.
    await page.getByText(/Cambiar a modo desarrollador|Switch to developer mode/i).click();

    // Should navigate to advanced view.
    await expect(page).toHaveURL(/view=advanced/);
    await expect(page.locator(".alfred-card-title", { hasText: /Nueva App/i })).toBeHidden({ timeout: 8_000 });
  });
});
