// Alfred Wizard — conversational intent capture for non-technical users.
// Backend: services/alfred /v1/intent/* (proxied to services/openspec).
// Feature flag: surface this route only when ?wizard=1 is set OR
// ALFRED_DIALOGUE_API=enabled on the server. The slash-command console at
// /alfred remains the default for power users.

import { authOptions } from "@/auth";
import { getServerSession } from "next-auth";
import { redirect } from "next/navigation";
import { randomUUID } from "crypto";

type SearchParams = {
  wizard?: string;
  draft_id?: string;
  workspace_id?: string;
  result?: string;
  error?: string;
};

const alfredUrl = () => process.env.ALFRED_URL ?? "http://localhost:8090";

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
      body: JSON.stringify({ workspace_id: workspaceId, business_intent: businessIntent }),
    });
    if (!r.ok) {
      const text = await r.text();
      redirect(`/alfred/wizard?wizard=1&error=${encodeURIComponent(`alfred ${r.status}: ${text}`)}`);
    }
    const body = await r.json();
    redirect(`/alfred/wizard?wizard=1&draft_id=${body.draft.draft_id}&workspace_id=${workspaceId}`);
  } catch (e: any) {
    redirect(`/alfred/wizard?wizard=1&error=${encodeURIComponent(e?.message ?? "fetch failed")}`);
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
  const error = searchParams.error;

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
          <label className="block text-sm">
            <span className="mb-1 block font-medium">Workspace ID</span>
            <input
              name="workspace_id"
              required
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

        {ready && (
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
