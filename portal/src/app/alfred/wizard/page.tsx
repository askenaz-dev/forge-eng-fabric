// Alfred Wizard — conversational intent capture for non-technical users.
// Backend: services/alfred /v1/intent/* (proxied to services/openspec).
// Feature flag: surface this route only when ?wizard=1 is set OR
// ALFRED_DIALOGUE_API=enabled on the server. The slash-command console at
// /alfred remains the default for power users.

import { authOptions } from "@/auth";
import { getServerSession } from "next-auth";
import { redirect } from "next/navigation";
import { randomUUID } from "crypto";
import { DesignSystemPickerForm } from "@/components/alfred/DesignSystemPickerForm";

type SearchParams = {
  wizard?: string;
  draft_id?: string;
  workspace_id?: string;
  // Phase 5 (app-first-class-entity 6.1): the App scope chosen by the
  // wizard's first step is propagated through the URL between steps. The
  // sentinel `_unassigned` is used for the "decide later" branch.
  app_id?: string;
  // app_scope_step=1 surfaces the App scope picker as the first step. When
  // unset and the URL has no app_id, the picker is shown automatically.
  app_scope_step?: string;
  // design-system-catalog (6.1): set after the "create new App" branch
  // creates the App but before business-intent capture starts. The step is
  // skipped on the "extend existing" and "decide later" branches.
  design_system_step?: string;
  // alfred-design-system-picker (D4): when the "create new App" branch is
  // active and the user has not yet picked a Design System, these carry
  // the App parameters forward so the design-system step's Continue can
  // issue a single atomic POST that creates the App with the picked ref.
  new_name?: string;
  new_slug?: string;
  result?: string;
  error?: string;
};

const alfredUrl = () => process.env.ALFRED_URL ?? "http://localhost:8090";
const applicationUrl = () => process.env.APPLICATION_URL ?? "http://localhost:8095";
const registryUrl = () => process.env.REGISTRY_URL ?? "http://localhost:8082";

async function getToken() {
  const session = await getServerSession(authOptions);
  if (!session) redirect("/api/auth/signin");
  return (session as { accessToken?: string }).accessToken;
}

async function startDraft(formData: FormData) {
  "use server";
  const token = await getToken();
  const workspaceId = formData.get("workspace_id") as string;
  const businessIntent = formData.get("business_intent") as string;
  // App scope is captured in the prior step (selectAppScope below) and
  // forwarded as a hidden field.
  const appId = (formData.get("app_id") as string) || "";
  if (!workspaceId || !businessIntent) {
    redirect(`/alfred/wizard?wizard=1&error=${encodeURIComponent("workspace and intent are required")}`);
  }
  try {
    const r = await fetch(`${alfredUrl()}/v1/intent/start`, {
      method: "POST",
      headers: {
        "content-type": "application/json",
        "x-correlation-id": randomUUID(),
        ...(token ? { authorization: `Bearer ${token}` } : {}),
      },
      body: JSON.stringify({
        workspace_id: workspaceId,
        business_intent: businessIntent,
        ...(appId ? { app_id: appId } : {}),
      }),
    });
    if (!r.ok) {
      const text = await r.text();
      redirect(`/alfred/wizard?wizard=1&error=${encodeURIComponent(`alfred ${r.status}: ${text}`)}`);
    }
    const body = await r.json();
    const qp = new URLSearchParams({
      wizard: "1",
      draft_id: body.draft.draft_id,
      workspace_id: workspaceId,
    });
    if (appId) qp.set("app_id", appId);
    redirect(`/alfred/wizard?${qp.toString()}`);
  } catch (e: any) {
    redirect(`/alfred/wizard?wizard=1&error=${encodeURIComponent(e?.message ?? "fetch failed")}`);
  }
}

// selectAppScope is the form action for the wizard's first step: pick an App
// scope (extend existing | create new | decide later). The branches are
// resolved here so the rest of the wizard receives a concrete `app_id`.
async function selectAppScope(formData: FormData) {
  "use server";
  const token = await getToken();
  const workspaceId = formData.get("workspace_id") as string;
  const branch = formData.get("branch") as string; // "existing" | "new" | "later"
  if (!workspaceId) {
    redirect(`/alfred/wizard?wizard=1&app_scope_step=1&error=${encodeURIComponent("workspace is required")}`);
  }
  if (branch === "existing") {
    const appId = formData.get("existing_app_id") as string;
    if (!appId) {
      redirect(`/alfred/wizard?wizard=1&workspace_id=${workspaceId}&app_scope_step=1&error=${encodeURIComponent("pick an existing app")}`);
    }
    redirect(`/alfred/wizard?wizard=1&workspace_id=${workspaceId}&app_id=${appId}`);
  }
  if (branch === "new") {
    const slug = formData.get("new_slug") as string;
    const name = formData.get("new_name") as string;
    if (!slug || !name) {
      redirect(`/alfred/wizard?wizard=1&workspace_id=${workspaceId}&app_scope_step=1&error=${encodeURIComponent("slug and name are required")}`);
    }
    // alfred-design-system-picker (D4): do NOT create the App yet. Defer
    // creation to selectDesignSystem so it can issue a single atomic POST
    // carrying both the App params and the chosen design_system_ref. The
    // app params travel through the URL between steps.
    const qp = new URLSearchParams({
      wizard: "1",
      workspace_id: workspaceId,
      design_system_step: "1",
      new_slug: slug,
      new_name: name,
    });
    redirect(`/alfred/wizard?${qp.toString()}`);
  }
  // branch === "later" — park the draft under _unassigned for the workspace.
  // The picker lookup returns the system-managed App id so the rest of the
  // wizard knows which one it parked against.
  const unassignedId = (formData.get("unassigned_app_id") as string) || "_unassigned";
  redirect(`/alfred/wizard?wizard=1&workspace_id=${workspaceId}&app_id=${unassignedId}`);
}

// selectDesignSystem is the server action for the design-system step.
//
// alfred-design-system-picker (D4): issues a single atomic
// `POST /v1/workspaces/{ws}/apps` carrying the App params (stashed in the
// URL by selectAppScope) AND the picker's design_system_ref +
// design_system_chosen_explicitly. Replaces the previous POST-then-PATCH
// flow so audit log shows one `app.created.v1` per App (plus, when skipped,
// the sibling `app.design_system.user_skipped.v1`).
async function selectDesignSystem(formData: FormData) {
  "use server";
  const token = await getToken();
  const workspaceId = formData.get("workspace_id") as string;
  const slug = formData.get("new_slug") as string;
  const name = formData.get("new_name") as string;
  // Two outcomes: explicit "continue" with a selected ref, or "skip" which
  // omits the ref entirely (lets the service alias-resolve ds-forge-default).
  const action = (formData.get("action") as string) || "continue";
  const ref = (formData.get("design_system_ref") as string) || "";
  if (!workspaceId || !slug || !name) {
    redirect(`/alfred/wizard?wizard=1&app_scope_step=1&error=${encodeURIComponent("missing workspace or app params")}`);
  }
  const body: Record<string, unknown> = {
    slug,
    name,
    owners: ["self"],
  };
  if (action === "skip") {
    body.design_system_chosen_explicitly = false;
    // omit design_system_ref — service resolves ds-forge-default.
  } else {
    body.design_system_chosen_explicitly = true;
    if (ref) body.design_system_ref = ref;
  }
  try {
    const r = await fetch(`${applicationUrl()}/v1/workspaces/${workspaceId}/apps`, {
      method: "POST",
      headers: {
        "content-type": "application/json",
        ...(token ? { authorization: `Bearer ${token}` } : {}),
      },
      body: JSON.stringify(body),
    });
    if (!r.ok) {
      const text = await r.text();
      const qp = new URLSearchParams({
        wizard: "1",
        workspace_id: workspaceId,
        design_system_step: "1",
        new_slug: slug,
        new_name: name,
        error: `create ${r.status}: ${text}`,
      });
      redirect(`/alfred/wizard?${qp.toString()}`);
    }
    const app = await r.json();
    redirect(`/alfred/wizard?wizard=1&workspace_id=${workspaceId}&app_id=${app.id}`);
  } catch (e: any) {
    const qp = new URLSearchParams({
      wizard: "1",
      workspace_id: workspaceId,
      design_system_step: "1",
      new_slug: slug,
      new_name: name,
      error: e?.message ?? "fetch failed",
    });
    redirect(`/alfred/wizard?${qp.toString()}`);
  }
}

// fetchDesignSystemCatalog returns the four built-in templates and any
// tenant-published Design Systems visible to the caller. Failure is
// non-fatal — the step renders an empty state with the i18n
// `wiz_ds_no_catalog` copy.
async function fetchDesignSystemCatalog(token: string | undefined) {
  try {
    const r = await fetch(`${registryUrl()}/v1/design-systems`, {
      headers: { ...(token ? { authorization: `Bearer ${token}` } : {}) },
      cache: "no-store",
    });
    if (!r.ok) return { catalog: [] as DesignSystemCatalogEntry[] };
    const body = await r.json();
    return { catalog: Array.isArray(body) ? (body as DesignSystemCatalogEntry[]) : [] };
  } catch {
    return { catalog: [] };
  }
}

type DesignSystemCatalogEntry = {
  asset_id: string;
  version: string;
  name: string;
  description?: string;
  manifest?: {
    use_case?: string;
    screenshots?: { light?: string; dark?: string };
  };
  built_in_template?: boolean;
  eval_scores?: Record<string, number>;
};

async function fetchWorkspaceApps(workspaceId: string, token: string | undefined) {
  try {
    const r = await fetch(`${applicationUrl()}/v1/workspaces/${workspaceId}/apps`, {
      headers: { ...(token ? { authorization: `Bearer ${token}` } : {}) },
      cache: "no-store",
    });
    if (!r.ok) return { apps: [] as Array<{ id: string; slug: string; name: string; system_managed: boolean }> };
    return r.json();
  } catch {
    return { apps: [] };
  }
}

async function answerDraft(formData: FormData) {
  "use server";
  const token = await getToken();
  const draftId = formData.get("draft_id") as string;
  const workspaceId = formData.get("workspace_id") as string;
  const answer = formData.get("answer") as string;
  const fieldUpdatesRaw = formData.get("field_updates") as string;
  let fieldUpdates: Record<string, unknown> = {};
  try {
    fieldUpdates = fieldUpdatesRaw ? JSON.parse(fieldUpdatesRaw) : {};
  } catch {
    fieldUpdates = {};
  }
  try {
    const r = await fetch(`${alfredUrl()}/v1/intent/${draftId}/answer`, {
      method: "POST",
      headers: {
        "content-type": "application/json",
        ...(token ? { authorization: `Bearer ${token}` } : {}),
      },
      body: JSON.stringify({ answer, field_updates: fieldUpdates }),
    });
    if (!r.ok) {
      const text = await r.text();
      redirect(`/alfred/wizard?wizard=1&draft_id=${draftId}&workspace_id=${workspaceId}&error=${encodeURIComponent(`alfred ${r.status}: ${text}`)}`);
    }
    redirect(`/alfred/wizard?wizard=1&draft_id=${draftId}&workspace_id=${workspaceId}`);
  } catch (e: any) {
    redirect(`/alfred/wizard?wizard=1&draft_id=${draftId}&workspace_id=${workspaceId}&error=${encodeURIComponent(e?.message ?? "fetch failed")}`);
  }
}

async function commitDraft(formData: FormData) {
  "use server";
  const token = await getToken();
  const draftId = formData.get("draft_id") as string;
  try {
    const r = await fetch(`${alfredUrl()}/v1/intent/${draftId}/commit`, {
      method: "POST",
      headers: {
        "content-type": "application/json",
        ...(token ? { authorization: `Bearer ${token}` } : {}),
      },
      body: JSON.stringify({}),
    });
    if (!r.ok) {
      const text = await r.text();
      redirect(`/alfred/wizard?wizard=1&draft_id=${draftId}&error=${encodeURIComponent(`commit failed: ${text}`)}`);
    }
    const body = await r.json();
    redirect(`/openspecs?committed=${body.openspec.openspec_id}`);
  } catch (e: any) {
    redirect(`/alfred/wizard?wizard=1&draft_id=${draftId}&error=${encodeURIComponent(e?.message ?? "fetch failed")}`);
  }
}

async function fetchDraft(draftId: string, token: string | undefined) {
  const r = await fetch(`${alfredUrl()}/v1/intent/${draftId}`, {
    headers: { ...(token ? { authorization: `Bearer ${token}` } : {}) },
    cache: "no-store",
  });
  if (!r.ok) return null;
  return r.json();
}

const STATUS_BADGE: Record<"complete" | "partial" | "empty", string> = {
  complete: "bg-emerald-100 text-emerald-800 dark:bg-emerald-950 dark:text-emerald-300",
  partial: "bg-amber-100 text-amber-800 dark:bg-amber-950 dark:text-amber-300",
  empty: "bg-neutral-200 text-neutral-700 dark:bg-neutral-800 dark:text-neutral-300",
};

export default async function AlfredWizardPage({ searchParams }: { searchParams: SearchParams }) {
  const wizardEnabled = searchParams.wizard === "1";
  const session = await getServerSession(authOptions);
  if (!session) redirect("/api/auth/signin");
  const token = (session as { accessToken?: string }).accessToken;

  if (!wizardEnabled) {
    return (
      <>
        <h1 className="page-title">
          Alfred <em>Wizard</em>
        </h1>
        <p className="page-sub">
          The conversational wizard is feature-flagged. Enable it for this session by appending{" "}
          <code style={{ background: "var(--bg-hover)", padding: "1px 6px", borderRadius: 4 }}>?wizard=1</code> to the URL, or use the{" "}
          <a style={{ color: "var(--primary)" }} href="/alfred">slash-command Alfred Console</a> instead.
        </p>
      </>
    );
  }

  const draftId = searchParams.draft_id ?? "";
  const workspaceId = searchParams.workspace_id ?? "";
  const appId = searchParams.app_id ?? "";
  const error = searchParams.error;
  // App scope step renders when no draft has been started yet AND no app_id
  // is carried in the URL. The "decide later" branch uses `_unassigned`,
  // which is treated as a *missing* scope so the commit button refuses.
  const needsAppScope = !draftId && !appId;
  const isUnassigned = appId === "_unassigned" || appId.startsWith("_unassigned");

  if (needsAppScope) {
    const appsBody = workspaceId ? await fetchWorkspaceApps(workspaceId, token) : { apps: [] };
    const apps = (appsBody?.apps ?? []) as Array<{ id: string; slug: string; name: string; system_managed: boolean }>;
    const unassigned = apps.find((a) => a.system_managed) ?? null;
    return (
      <>
        <h1 className="page-title">Alfred <em>Wizard</em></h1>
        <p className="page-sub" style={{ marginBottom: 16 }}>
          First, pick the App this spec belongs to. Every OpenSpec needs an App scope so it lands in
          the right home — extending an existing product, starting a new one, or parked for later.
        </p>
        {error && (
          <div className="rounded border border-red-300 bg-red-50 p-3 text-sm text-red-800 dark:border-red-800 dark:bg-red-950 dark:text-red-200">
            {error}
          </div>
        )}
        <form action={selectAppScope} className="space-y-4 rounded border border-neutral-200 bg-white p-4 dark:border-neutral-800 dark:bg-neutral-900">
          <label className="block text-sm">
            <span className="mb-1 block font-medium">Workspace ID</span>
            <input
              name="workspace_id"
              required
              defaultValue={workspaceId}
              className="w-full rounded border border-neutral-300 px-3 py-2 text-sm dark:border-neutral-700 dark:bg-neutral-800"
            />
          </label>

          <fieldset className="space-y-3">
            <legend className="text-sm font-medium">App scope</legend>

            <div className="rounded border border-neutral-200 p-3 dark:border-neutral-800">
              <label className="flex items-center gap-2">
                <input type="radio" name="branch" value="existing" defaultChecked />
                <span className="font-medium">Extend an existing App</span>
              </label>
              <select
                name="existing_app_id"
                className="mt-2 w-full rounded border border-neutral-300 px-3 py-2 text-sm dark:border-neutral-700 dark:bg-neutral-800"
              >
                <option value="">Pick an App…</option>
                {apps
                  .filter((a) => !a.system_managed)
                  .map((a) => (
                    <option key={a.id} value={a.id}>
                      {a.name} ({a.slug})
                    </option>
                  ))}
              </select>
            </div>

            <div className="rounded border border-neutral-200 p-3 dark:border-neutral-800">
              <label className="flex items-center gap-2">
                <input type="radio" name="branch" value="new" />
                <span className="font-medium">Create a new App</span>
              </label>
              <div className="mt-2 grid grid-cols-2 gap-2">
                <input
                  name="new_slug"
                  placeholder="slug (e.g. hr-portal)"
                  className="w-full rounded border border-neutral-300 px-3 py-2 text-sm dark:border-neutral-700 dark:bg-neutral-800"
                />
                <input
                  name="new_name"
                  placeholder="Display name"
                  className="w-full rounded border border-neutral-300 px-3 py-2 text-sm dark:border-neutral-700 dark:bg-neutral-800"
                />
              </div>
            </div>

            <div className="rounded border border-amber-200 bg-amber-50 p-3 dark:border-amber-900 dark:bg-amber-950">
              <label className="flex items-center gap-2">
                <input type="radio" name="branch" value="later" />
                <span className="font-medium">I don't know yet</span>
              </label>
              <p className="mt-1 text-xs text-amber-800 dark:text-amber-200">
                Park this draft under the workspace's <code>_unassigned</code> bucket. You will need to
                pick a real App before you can commit.
              </p>
              <input type="hidden" name="unassigned_app_id" value={unassigned?.id ?? "_unassigned"} />
            </div>
          </fieldset>

          <button className="rounded bg-neutral-900 px-4 py-2 text-sm font-medium text-white dark:bg-neutral-100 dark:text-neutral-900">
            Continue
          </button>
        </form>
      </>
    );
  }

  if (searchParams.design_system_step === "1" && searchParams.new_slug && searchParams.new_name) {
    const { catalog } = await fetchDesignSystemCatalog(token);
    const newSlug = searchParams.new_slug;
    const newName = searchParams.new_name;
    return (
      <>
        {error && (
          <div className="rounded border border-red-300 bg-red-50 p-3 text-sm text-red-800 dark:border-red-800 dark:bg-red-950 dark:text-red-200">
            {error}
          </div>
        )}
        {/* alfred-design-system-picker (6.4): the wizard renders the shared
            DesignSystemPicker via DesignSystemPickerForm, which adapts the
            client-side picker (callbacks) to the wizard's server action
            (`<form action>`). Same component as the Friendly view path —
            single source of truth for layout, screenshots, copy and Skip
            semantics. The server action issues the atomic POST per D4. */}
        <DesignSystemPickerForm
          catalog={catalog}
          hiddenFields={{
            workspace_id: workspaceId,
            new_slug: newSlug,
            new_name: newName,
          }}
          formAction={selectDesignSystem}
          loadError={catalog.length === 0}
        />
        <p className="mt-3 text-sm">
          <a style={{ color: "var(--primary)" }} href={`/alfred/wizard?wizard=1&workspace_id=${workspaceId}&app_scope_step=1`}>← Back</a>
        </p>
      </>
    );
  }

  if (!draftId) {
    return (
      <>
        <h1 className="page-title">
          Alfred <em>Wizard</em>
        </h1>
        <p className="page-sub" style={{ marginBottom: 16 }}>
          Describe what you want to build. Alfred will ask follow-up questions and assemble a structured specification for review.
        </p>
        {error && (
          <div className="rounded border border-red-300 bg-red-50 p-3 text-sm text-red-800 dark:border-red-800 dark:bg-red-950 dark:text-red-200">
            {error}
          </div>
        )}
        <form action={startDraft} className="space-y-3 rounded border border-neutral-200 bg-white p-4 dark:border-neutral-800 dark:bg-neutral-900">
          <input type="hidden" name="app_id" value={appId} />
          {appId && (
            <p className="text-xs" style={{ color: "var(--fg-3)" }}>
              App scope: <code style={{ background: "var(--bg-hover)", padding: "1px 6px", borderRadius: 4 }}>{appId}</code>
              {" · "}
              <a href={`/alfred/wizard?wizard=1&workspace_id=${workspaceId}`} style={{ color: "var(--primary)" }}>
                change
              </a>
            </p>
          )}
          <label className="block text-sm">
            <span className="mb-1 block font-medium">Workspace ID</span>
            <input
              name="workspace_id"
              required
              defaultValue={workspaceId}
              className="w-full rounded border border-neutral-300 px-3 py-2 text-sm dark:border-neutral-700 dark:bg-neutral-800"
            />
          </label>
          <label className="block text-sm">
            <span className="mb-1 block font-medium">What are you trying to build?</span>
            <textarea
              name="business_intent"
              required
              rows={5}
              placeholder="e.g., A retail loyalty rewards engine that tracks purchase history and issues tier-based discounts."
              className="w-full rounded border border-neutral-300 px-3 py-2 text-sm dark:border-neutral-700 dark:bg-neutral-800"
            />
          </label>
          <button className="rounded bg-neutral-900 px-4 py-2 text-sm font-medium text-white dark:bg-neutral-100 dark:text-neutral-900">
            Start the wizard
          </button>
        </form>
      </>
    );
  }

  const draftBody = await fetchDraft(draftId, token);
  if (!draftBody) {
    return (
      <>
        <h1 className="page-title">
          Alfred <em>Wizard</em>
        </h1>
        <div style={{ padding: 14, background: "color-mix(in oklch, var(--spark), transparent 80%)", borderRadius: "var(--r-3)", color: "var(--spark)" }}>
          Draft not found. <a style={{ color: "var(--primary)" }} href="/alfred/wizard?wizard=1">Start a new draft</a>.
        </div>
      </>
    );
  }

  const draft = draftBody.draft;
  const completeness = draftBody.completeness;
  const nextQuestion = draftBody.next_question as string | null;
  const ready = !nextQuestion;

  return (
    <div className="grid gap-6 lg:grid-cols-[1fr_320px]">
      <div className="space-y-4">
        <header>
          <div className="h-eyebrow">Wizard</div>
          <h1 className="page-title">Capturing intent — <em>{draft.title || "(untitled)"}</em></h1>
          <p style={{ fontFamily: "var(--f-mono)", fontSize: 11, color: "var(--fg-3)" }}>draft_id: {draft.draft_id} · turns: {draft.turn_count}</p>
        </header>

        {error && (
          <div className="rounded border border-red-300 bg-red-50 p-3 text-sm text-red-800 dark:border-red-800 dark:bg-red-950 dark:text-red-200">
            {error}
          </div>
        )}

        {!ready && (
          <form action={answerDraft} className="space-y-3 rounded border border-neutral-200 bg-white p-4 dark:border-neutral-800 dark:bg-neutral-900">
            <input type="hidden" name="draft_id" value={draftId} />
            <input type="hidden" name="workspace_id" value={workspaceId} />
            <p className="font-medium">{nextQuestion}</p>
            <textarea
              name="answer"
              required
              rows={4}
              className="w-full rounded border border-neutral-300 px-3 py-2 text-sm dark:border-neutral-700 dark:bg-neutral-800"
              placeholder="Type your answer..."
            />
            <details className="text-xs">
              <summary className="cursor-pointer opacity-70 hover:opacity-100">Advanced — set field updates</summary>
              <textarea
                name="field_updates"
                rows={3}
                placeholder='JSON, e.g., {"requirements_functional": ["track purchases", "issue tier-based discounts"]}'
                className="mt-2 w-full rounded border border-neutral-300 px-3 py-2 font-mono text-xs dark:border-neutral-700 dark:bg-neutral-800"
              />
            </details>
            <button className="rounded bg-neutral-900 px-4 py-2 text-sm font-medium text-white dark:bg-neutral-100 dark:text-neutral-900">
              Submit answer
            </button>
          </form>
        )}

        {ready && isUnassigned && (
          <div className="space-y-2 rounded border border-amber-300 bg-amber-50 p-4 dark:border-amber-800 dark:bg-amber-950">
            <p className="font-medium text-amber-900 dark:text-amber-200">App scope still missing</p>
            <p className="text-sm text-amber-900/80 dark:text-amber-200/80">
              This draft is parked under the workspace&apos;s <code>_unassigned</code> bucket. Pick a real App
              before committing — the platform refuses commits without a real App scope.
            </p>
            <a
              href={`/alfred/wizard?wizard=1&workspace_id=${workspaceId}&app_scope_step=1`}
              className="inline-block rounded bg-amber-700 px-4 py-2 text-sm font-medium text-white"
            >
              Pick an App
            </a>
          </div>
        )}

        {ready && !isUnassigned && (
          <form action={commitDraft} className="space-y-3 rounded border border-emerald-300 bg-emerald-50 p-4 dark:border-emerald-800 dark:bg-emerald-950">
            <input type="hidden" name="draft_id" value={draftId} />
            <p className="font-medium text-emerald-900 dark:text-emerald-200">Ready to commit</p>
            <p className="text-sm text-emerald-900/80 dark:text-emerald-200/80">
              All sections complete. Click below to commit the specification and hand off to the SDLC orchestrator.
            </p>
            <button className="rounded bg-emerald-700 px-4 py-2 text-sm font-medium text-white">
              Ejecutar SDLC (Commit specification)
            </button>
          </form>
        )}

        <details className="text-sm">
          <summary className="cursor-pointer text-neutral-600 dark:text-neutral-300">Draft preview</summary>
          <pre className="mt-2 overflow-x-auto rounded border border-neutral-200 bg-white p-3 text-xs dark:border-neutral-800 dark:bg-neutral-950">
            {JSON.stringify(draft, null, 2)}
          </pre>
        </details>
      </div>

      <aside className="space-y-3">
        <p className="text-sm font-medium uppercase tracking-wide text-neutral-500">Completeness</p>
        <div className="space-y-2">
          {completeness?.sections?.map((section: any) => (
            <div key={section.name} className="rounded border border-neutral-200 bg-white p-3 dark:border-neutral-800 dark:bg-neutral-900">
              <div className="flex items-center justify-between">
                <span className="font-medium capitalize">{section.name}</span>
                <span className={`rounded-full px-2 py-1 text-xs ${STATUS_BADGE[section.status as keyof typeof STATUS_BADGE]}`}>
                  {section.status}
                </span>
              </div>
              <ul className="mt-2 space-y-0.5 text-xs">
                {Object.entries(section.fields ?? {}).map(([fieldName, fieldStatus]: any) => (
                  <li key={fieldName} className="flex items-center justify-between">
                    <span className="font-mono opacity-70">{fieldName}</span>
                    <span className={`rounded-full px-1.5 py-0.5 ${STATUS_BADGE[fieldStatus as keyof typeof STATUS_BADGE]}`}>{fieldStatus}</span>
                  </li>
                ))}
              </ul>
            </div>
          ))}
        </div>
      </aside>
    </div>
  );
}
