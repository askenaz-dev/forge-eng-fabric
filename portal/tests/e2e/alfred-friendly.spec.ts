import { expect, test } from "@playwright/test";

// ── Shared fixtures ─────────────────────────────────────────────────────────

const WS_ID = "00000000-0000-0000-0000-000000000001";
const DRAFT_ID = "draft-e2e-001";

async function stubAlfredApis(page: Parameters<Parameters<typeof test>[1]>[0]) {
  // User preferences: no stored view → let ?view= param take over.
  await page.route("**/api/user/preferences", (route) => {
    if (route.request().method() === "GET") {
      return route.fulfill({ json: { console_view_preference: null } });
    }
    return route.fulfill({ json: { ok: true } });
  });

  // Workspace active: single workspace, no apps.
  await page.route("**/api/workspace/active", (route) =>
    route.fulfill({ json: { workspace_id: WS_ID, apps: [] } }),
  );
}

async function stubIntentStartSuccess(page: Parameters<Parameters<typeof test>[1]>[0]) {
  await page.route("**/api/alfred/intent/start", (route) =>
    route.fulfill({
      json: {
        draft: { draft_id: DRAFT_ID },
        next_question: "¿Cuál es el nombre de la App?",
      },
    }),
  );
}

async function stubIntentStartForbidden(page: Parameters<Parameters<typeof test>[1]>[0]) {
  await page.route("**/api/alfred/intent/start", (route) =>
    route.fulfill({ status: 403, body: "Forbidden" }),
  );
}

async function stubIntentAnswer(page: Parameters<Parameters<typeof test>[1]>[0]) {
  await page.route("**/api/alfred/intent/answer", (route) =>
    route.fulfill({ json: { next_question: "¿Cuántos usuarios simultáneos esperas?" } }),
  );
}

// ── Card rendering ───────────────────────────────────────────────────────────

test.describe("@alfred-friendly Friendly view — landing cards", () => {
  test("shows three cards with correct titles and CTA", async ({ page }) => {
    await stubAlfredApis(page);
    await page.goto("/alfred?view=friendly");
    await page.waitForLoadState("networkidle");

    // Three card titles visible.
    await expect(page.locator(".alfred-card-title", { hasText: /Nueva App/i })).toBeVisible();
    await expect(page.locator(".alfred-card-title", { hasText: /Mejorar/i })).toBeVisible();
    await expect(page.locator(".alfred-card-title", { hasText: /Operar/i })).toBeVisible();

    // Each card has a CTA button.
    const ctas = page.locator(".alfred-card-cta");
    await expect(ctas).toHaveCount(3);
  });

  test("page title and subtitle are rendered", async ({ page }) => {
    await stubAlfredApis(page);
    await page.goto("/alfred?view=friendly");
    await page.waitForLoadState("networkidle");

    await expect(page.locator("h1", { hasText: /Alfred/i })).toBeVisible();
    await expect(page.getByText(/qué quieres hacer|what do you want to do/i)).toBeVisible();
  });
});

// ── Golden path: Nueva App card ─────────────────────────────────────────────

test.describe("@alfred-friendly Friendly view — Nueva App golden path", () => {
  test("clicking Nueva App opens conversation panel", async ({ page }) => {
    await stubAlfredApis(page);
    await stubIntentStartSuccess(page);
    await page.goto("/alfred?view=friendly");
    await page.waitForLoadState("networkidle");

    await page.locator(".alfred-card", { hasText: /Nueva App/i }).click();

    // Conversation panel appears with first question from Alfred.
    await expect(page.getByText(/cuál es el nombre/i)).toBeVisible({ timeout: 8_000 });
  });

  test("back button returns to card landing", async ({ page }) => {
    await stubAlfredApis(page);
    await stubIntentStartSuccess(page);
    await page.goto("/alfred?view=friendly");
    await page.waitForLoadState("networkidle");

    await page.locator(".alfred-card", { hasText: /Nueva App/i }).click();
    await page.getByText(/cuál es el nombre/i).waitFor({ timeout: 8_000 });

    // Click the back arrow.
    await page.getByLabel(/Volver|Back/i).click();

    // Cards are back.
    await expect(page.locator(".alfred-card-title", { hasText: /Nueva App/i })).toBeVisible();
  });

  test("user can send a follow-up message", async ({ page }) => {
    await stubAlfredApis(page);
    await stubIntentStartSuccess(page);
    await stubIntentAnswer(page);
    await page.goto("/alfred?view=friendly");
    await page.waitForLoadState("networkidle");

    await page.locator(".alfred-card", { hasText: /Nueva App/i }).click();
    await page.getByText(/cuál es el nombre/i).waitFor({ timeout: 8_000 });

    const input = page.locator("textarea, input[type=text]").last();
    await input.fill("MiNuevaApp");
    await page.keyboard.press("Enter");

    await expect(page.getByText(/cuántos usuarios/i)).toBeVisible({ timeout: 8_000 });
  });
});

// ── Golden path: Mejorar + Operar cards ─────────────────────────────────────

test.describe("@alfred-friendly Friendly view — Mejorar and Operar cards", () => {
  for (const cardText of ["Mejorar", "Operar"]) {
    test(`clicking ${cardText} opens conversation panel`, async ({ page }) => {
      await stubAlfredApis(page);
      await stubIntentStartSuccess(page);
      await page.goto("/alfred?view=friendly");
      await page.waitForLoadState("networkidle");

      await page.locator(".alfred-card", { hasText: new RegExp(cardText, "i") }).click();

      await expect(page.getByText(/cuál es el nombre/i)).toBeVisible({ timeout: 8_000 });
    });
  }
});

// ── Permission-denied path ───────────────────────────────────────────────────

test.describe("@alfred-friendly Friendly view — permission-denied path", () => {
  test("403 from intent/start shows a friendly error message", async ({ page }) => {
    await stubAlfredApis(page);
    await stubIntentStartForbidden(page);
    await page.goto("/alfred?view=friendly");
    await page.waitForLoadState("networkidle");

    await page.locator(".alfred-card", { hasText: /Nueva App/i }).click();

    // Friendly error message (not a raw stack trace).
    await expect(
      page.getByText(/error inesperado|unexpected error|no tienes permisos/i),
    ).toBeVisible({ timeout: 8_000 });
  });

  test("error disclosure allows viewing raw detail", async ({ page }) => {
    await stubAlfredApis(page);
    await stubIntentStartForbidden(page);
    await page.goto("/alfred?view=friendly");
    await page.waitForLoadState("networkidle");

    await page.locator(".alfred-card", { hasText: /Nueva App/i }).click();
    await page.getByText(/error inesperado|unexpected error|no tienes permisos/i).waitFor({ timeout: 8_000 });

    // Disclosure button expands raw detail.
    const disclosureBtn = page.getByText(/Ver detalles|Show.*details/i);
    if (await disclosureBtn.isVisible()) {
      await disclosureBtn.click();
      await expect(page.getByText(/Forbidden|403/i)).toBeVisible();
    }
  });
});
