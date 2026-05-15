// App detail page (app-first-class-entity 10.2): tabs for Specs / Runs /
// Deployments / Runtimes / Settings. The path mirrors the
// `/workspaces/{ws_slug}/apps/{app_slug}` URL form chosen in design Decision 2.
//
// Data sources:
//   - Apps  : services/application  (forwarded via env var APPLICATION_URL)
//   - Specs : services/openspec
//   - Deploys + Runtimes: registry / runtime-registry
//
// The page is rendered server-side; tab selection drives a single client
// re-fetch via the `tab` query string.

import { authOptions } from "@/auth";
import { getServerSession } from "next-auth";
import { redirect } from "next/navigation";
import Link from "next/link";

type SearchParams = { tab?: string; ds_error?: string; ds_ok?: string };
type PageParams = { ws_slug: string; app_slug: string };

const applicationUrl = () => process.env.APPLICATION_URL ?? "http://localhost:8095";
const registryUrl = () => process.env.REGISTRY_URL ?? "http://localhost:8082";

type TargetValue = "required" | "optional" | "opt-in" | "skipped";

type AppTargets = {
  architect?: TargetValue;
  design?: TargetValue;
  development?: TargetValue;
  qa?: TargetValue;
  security?: TargetValue;
  devops?: TargetValue;
  iac?: TargetValue;
  sre?: TargetValue;
  finops?: TargetValue;
  observability?: TargetValue;
};

const TARGET_PHASES = [
  "architect", "design", "development", "qa", "security",
  "devops", "iac", "sre", "finops", "observability",
] as const;

const TARGET_VALUES: TargetValue[] = ["required", "optional", "opt-in", "skipped"];

type App = {
  id: string;
  slug: string;
  name: string;
  description?: string;
  workspace_id: string;
  tenant_id: string;
  lifecycle_state: "active" | "archived" | "deleted";
  owners: string[];
  system_managed: boolean;
  repo_links?: string[];
  runtime_links?: string[];
  default_environments?: string[];
  targets?: AppTargets;
  created_at: string;
  archived_at?: string | null;
  // design-system-catalog: every App carries a design_system_ref (defaults to
  // ds-forge-default). Overrides map component primitives to a secondary ref.
  design_system_ref?: string;
  design_system_overrides?: Record<string, string>;
};

type DesignSystemCatalogEntry = {
  asset_id: string;
  version: string;
  name: string;
  manifest?: { use_case?: string };
  built_in_template?: boolean;
};

type SwapPR = {
  pr_url: string;
  target_ref: string;
  reason?: string;
  status: string;
  opened_at: string;
};

async function fetchApp(workspaceSlug: string, appSlug: string, token: string | undefined): Promise<App | null> {
  // The application service exposes apps by id and by workspace listing; the
  // portal looks up via the workspace's list to translate slugs into ids and
  // returns the matching row.
  try {
    const r = await fetch(`${applicationUrl()}/v1/workspaces/${workspaceSlug}/apps`, {
      headers: { ...(token ? { authorization: `Bearer ${token}` } : {}) },
      cache: "no-store",
    });
    if (!r.ok) return null;
    const body = (await r.json()) as { apps: App[] };
    return body.apps.find((a) => a.slug === appSlug) ?? null;
  } catch {
    return null;
  }
}

const TABS = [
  { id: "specs", label: "Specs" },
  { id: "runs", label: "Runs" },
  { id: "deployments", label: "Deployments" },
  { id: "runtimes", label: "Runtimes" },
  { id: "settings", label: "Settings" },
];

// design-system-catalog: canonical primitive list mirrored from the
// application service's CanonicalComponentPrimitives. The Settings panel
// renders this list for the per-component overrides advanced section.
const CANONICAL_COMPONENT_PRIMITIVES = [
  "button", "badge", "card", "kpi", "chip", "seg", "sheet", "terminal", "code", "run_row", "approval_card",
] as const;

// swapDesignSystem opens a PR against the App's portal-bundle repo.
async function swapDesignSystem(formData: FormData) {
  "use server";
  const { redirect } = await import("next/navigation");
  const wsSlug = formData.get("ws_slug") as string;
  const appSlug = formData.get("app_slug") as string;
  const appId = formData.get("app_id") as string;
  const targetRef = formData.get("target_ref") as string;
  const reason = (formData.get("reason") as string) || "";
  if (!appId || !targetRef) {
    redirect(`/workspaces/${wsSlug}/apps/${appSlug}?tab=settings&ds_error=${encodeURIComponent("missing fields")}`);
  }
  try {
    const r = await fetch(`${applicationUrl()}/v1/apps/${appId}/design-system:swap`, {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ target_ref: targetRef, reason }),
    });
    if (!r.ok) {
      const text = await r.text();
      redirect(`/workspaces/${wsSlug}/apps/${appSlug}?tab=settings&ds_error=${encodeURIComponent(`swap ${r.status}: ${text}`)}`);
    }
    redirect(`/workspaces/${wsSlug}/apps/${appSlug}?tab=settings&ds_ok=swap_pr_opened`);
  } catch (e: any) {
    redirect(`/workspaces/${wsSlug}/apps/${appSlug}?tab=settings&ds_error=${encodeURIComponent(e?.message ?? "fetch failed")}`);
  }
}

// patchTargets updates the per-phase SDLC targets for this App.
// The backend rejects callers without app#owner with 403.
async function patchTargets(formData: FormData) {
  "use server";
  const { redirect } = await import("next/navigation");
  const wsSlug = formData.get("ws_slug") as string;
  const appSlug = formData.get("app_slug") as string;
  const appId = formData.get("app_id") as string;
  const targets: Record<string, string> = {};
  for (const phase of TARGET_PHASES) {
    const v = formData.get(`target_${phase}`);
    if (typeof v === "string" && v) targets[phase] = v;
  }
  try {
    const r = await fetch(`${applicationUrl()}/v1/apps/${appId}`, {
      method: "PATCH",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ targets }),
    });
    if (!r.ok) {
      const text = await r.text();
      redirect(`/workspaces/${wsSlug}/apps/${appSlug}?tab=settings&ds_error=${encodeURIComponent(`targets ${r.status}: ${text}`)}`);
    }
    redirect(`/workspaces/${wsSlug}/apps/${appSlug}?tab=settings&ds_ok=targets_saved`);
  } catch (e: any) {
    redirect(`/workspaces/${wsSlug}/apps/${appSlug}?tab=settings&ds_error=${encodeURIComponent(e?.message ?? "fetch failed")}`);
  }
}

// patchOverrides updates the per-component overrides.
async function patchOverrides(formData: FormData) {
  "use server";
  const { redirect } = await import("next/navigation");
  const wsSlug = formData.get("ws_slug") as string;
  const appSlug = formData.get("app_slug") as string;
  const appId = formData.get("app_id") as string;
  const overrides: Record<string, string> = {};
  for (const component of CANONICAL_COMPONENT_PRIMITIVES) {
    const v = formData.get(`override_${component}`);
    if (typeof v === "string" && v.trim()) overrides[component] = v.trim();
  }
  try {
    const r = await fetch(`${applicationUrl()}/v1/apps/${appId}/design-system/overrides`, {
      method: "PATCH",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ overrides }),
    });
    if (!r.ok) {
      const text = await r.text();
      redirect(`/workspaces/${wsSlug}/apps/${appSlug}?tab=settings&ds_error=${encodeURIComponent(`overrides ${r.status}: ${text}`)}`);
    }
    redirect(`/workspaces/${wsSlug}/apps/${appSlug}?tab=settings&ds_ok=overrides_saved`);
  } catch (e: any) {
    redirect(`/workspaces/${wsSlug}/apps/${appSlug}?tab=settings&ds_error=${encodeURIComponent(e?.message ?? "fetch failed")}`);
  }
}

async function fetchDesignSystemCatalog(token: string | undefined): Promise<DesignSystemCatalogEntry[]> {
  try {
    const r = await fetch(`${registryUrl()}/v1/design-systems`, {
      headers: { ...(token ? { authorization: `Bearer ${token}` } : {}) },
      cache: "no-store",
    });
    if (!r.ok) return [];
    const body = await r.json();
    return Array.isArray(body) ? body : [];
  } catch {
    return [];
  }
}

export default async function AppDetailPage({
  params,
  searchParams,
}: {
  params: PageParams;
  searchParams: SearchParams;
}) {
  const session = await getServerSession(authOptions);
  if (!session) redirect("/api/auth/signin");
  const token = (session as { accessToken?: string }).accessToken;
  const app = await fetchApp(params.ws_slug, params.app_slug, token);

  if (!app) {
    return (
      <div>
        <h1 className="page-title">App not found</h1>
        <p className="page-sub">
          No App named <code>{params.app_slug}</code> in workspace{" "}
          <code>{params.ws_slug}</code>.
        </p>
        <Link href={`/workspaces/${params.ws_slug}/apps/new`} style={{ color: "var(--primary)" }}>
          Create a new App
        </Link>
      </div>
    );
  }

  const activeTab = TABS.find((t) => t.id === searchParams.tab) ?? TABS[0];
  const isUnassigned = app.system_managed || app.slug === "_unassigned";
  // design-system-catalog: load the catalog when the Settings tab is active.
  // Renders are otherwise unaffected — the panel only shows under Settings.
  const designSystemCatalog = activeTab.id === "settings" ? await fetchDesignSystemCatalog(token) : [];
  const dsRef = app.design_system_ref || "ds-forge-default";
  const dsOverrides = app.design_system_overrides || {};

  return (
    <div>
      <header style={{ marginBottom: 16 }}>
        <div className="h-eyebrow">App</div>
        <h1 className="page-title">
          {app.name}
          {isUnassigned && (
            <span
              style={{
                marginLeft: 12,
                padding: "2px 8px",
                fontSize: 11,
                color: "var(--fg-3)",
                background: "var(--bg-hover)",
                borderRadius: 4,
                verticalAlign: "middle",
              }}
            >
              system-managed
            </span>
          )}
          {app.lifecycle_state === "archived" && (
            <span
              style={{
                marginLeft: 12,
                padding: "2px 8px",
                fontSize: 11,
                color: "var(--amber)",
                background: "color-mix(in oklch, var(--amber), transparent 80%)",
                borderRadius: 4,
                verticalAlign: "middle",
              }}
            >
              archived
            </span>
          )}
        </h1>
        <p className="page-sub" style={{ fontFamily: "var(--f-mono)", fontSize: 11 }}>
          slug: <code>{app.slug}</code> · id: <code>{app.id}</code>
        </p>
        {app.description && <p style={{ marginTop: 8 }}>{app.description}</p>}
      </header>

      <nav style={{ display: "flex", gap: 4, borderBottom: "1px solid var(--bd-2)", marginBottom: 16 }}>
        {TABS.map((tab) => {
          const href = `/workspaces/${params.ws_slug}/apps/${params.app_slug}?tab=${tab.id}`;
          const active = activeTab.id === tab.id;
          return (
            <Link
              key={tab.id}
              href={href}
              style={{
                padding: "8px 14px",
                fontSize: 13,
                fontWeight: active ? 600 : 400,
                color: active ? "var(--fg-1)" : "var(--fg-3)",
                borderBottom: `2px solid ${active ? "var(--primary)" : "transparent"}`,
                marginBottom: -1,
              }}
            >
              {tab.label}
            </Link>
          );
        })}
      </nav>

      <section>
        {activeTab.id === "specs" && (
          <p style={{ color: "var(--fg-3)" }}>
            OpenSpecs anchored to this App appear here. Wired to the OpenSpec backbone
            via <code>GET /v1/openspecs?app_id={app.id}</code>.
          </p>
        )}
        {activeTab.id === "runs" && (
          <p style={{ color: "var(--fg-3)" }}>
            SDLC runs against this App appear here. Wired to the SDLC orchestrator.
          </p>
        )}
        {activeTab.id === "deployments" && (
          <p style={{ color: "var(--fg-3)" }}>
            Asset deployments anchored to this App appear here. Wired to the registry.
          </p>
        )}
        {activeTab.id === "runtimes" && (
          <p style={{ color: "var(--fg-3)" }}>
            Runtime registrations anchored to this App appear here. Wired to the runtime registry.
          </p>
        )}
        {activeTab.id === "settings" && (
          <>
            <dl style={{ display: "grid", gridTemplateColumns: "160px 1fr", gap: "8px 16px" }}>
              <dt>Owners</dt>
              <dd>{app.owners.join(", ") || "(none)"}</dd>
              <dt>Repo links</dt>
              <dd>{app.repo_links?.join(", ") || "(none)"}</dd>
              <dt>Runtime links</dt>
              <dd>{app.runtime_links?.join(", ") || "(none)"}</dd>
              <dt>Default environments</dt>
              <dd>{app.default_environments?.join(", ") || "(none)"}</dd>
              <dt>Created at</dt>
              <dd>{app.created_at}</dd>
              {app.archived_at && (
                <>
                  <dt>Archived at</dt>
                  <dd>{app.archived_at}</dd>
                </>
              )}
            </dl>

            {/* sdlc-end-to-end: SDLC targets panel (app#owner can edit; app#viewer read-only) */}
            <section data-targets-panel style={{ marginTop: 24, padding: 16, border: "1px solid var(--border)", borderRadius: "var(--r-3)", background: "var(--bg-card)" }}>
              <h2 style={{ fontSize: 16, margin: "0 0 12px" }}>SDLC Targets</h2>
              <p style={{ fontSize: 12, color: "var(--fg-2)", marginBottom: 12 }}>
                Declare which SDLC phases are <strong>required</strong> (workflow blocks on gate failure),
                <strong> optional</strong> (warning only), <strong>opt-in</strong> (skipped unless explicitly requested),
                or <strong>skipped</strong> (removed from the plan entirely).
              </p>
              {searchParams.ds_ok === "targets_saved" && (
                <div className="mb-3 rounded border border-emerald-300 bg-emerald-50 p-2 text-xs text-emerald-800 dark:border-emerald-800 dark:bg-emerald-950 dark:text-emerald-200">
                  Targets saved.
                </div>
              )}
              <form action={patchTargets}>
                <input type="hidden" name="ws_slug" value={params.ws_slug} />
                <input type="hidden" name="app_slug" value={params.app_slug} />
                <input type="hidden" name="app_id" value={app.id} />
                <table style={{ width: "100%", borderCollapse: "collapse", fontSize: 13 }}>
                  <thead>
                    <tr style={{ borderBottom: "1px solid var(--bd-2)" }}>
                      <th style={{ textAlign: "left", padding: "4px 8px", fontWeight: 600 }}>Phase</th>
                      <th style={{ textAlign: "left", padding: "4px 8px", fontWeight: 600 }}>Policy</th>
                    </tr>
                  </thead>
                  <tbody>
                    {TARGET_PHASES.map((phase) => {
                      const current = app.targets?.[phase] ?? "required";
                      return (
                        <tr key={phase} style={{ borderBottom: "1px solid var(--bd-3)" }}>
                          <td style={{ padding: "6px 8px", fontFamily: "var(--f-mono)", fontSize: 12 }}>{phase}</td>
                          <td style={{ padding: "4px 8px" }}>
                            <select
                              name={`target_${phase}`}
                              defaultValue={current}
                              disabled={isUnassigned}
                              className="rounded border border-neutral-300 px-2 py-1 text-xs dark:border-neutral-700 dark:bg-neutral-800"
                            >
                              {TARGET_VALUES.map((v) => (
                                <option key={v} value={v}>{v}</option>
                              ))}
                            </select>
                          </td>
                        </tr>
                      );
                    })}
                  </tbody>
                </table>
                {!isUnassigned && (
                  <button
                    type="submit"
                    className="mt-3 rounded bg-neutral-900 px-3 py-1.5 text-sm font-medium text-white dark:bg-neutral-100 dark:text-neutral-900"
                  >
                    Save targets
                  </button>
                )}
              </form>
            </section>

            {/* design-system-catalog: Design System swap & override panel.
                Visibility is enforced server-side (the API rejects swap
                requests from callers without app#owner). The panel is
                rendered for every viewer for now; an app#owner check at the
                Portal layer is a follow-up. */}
            <section data-design-system-panel style={{ marginTop: 24, padding: 16, border: "1px solid var(--border)", borderRadius: "var(--r-3)", background: "var(--bg-card)" }}>
              <header style={{ display: "flex", alignItems: "center", justifyContent: "space-between" }}>
                <h2 style={{ fontSize: 16, margin: 0 }}>Design System</h2>
                <code style={{ fontSize: 11, color: "var(--fg-3)", background: "var(--bg-hover)", padding: "2px 6px", borderRadius: 4 }}>
                  {dsRef}
                </code>
              </header>

              {searchParams.ds_error && (
                <div className="mt-3 rounded border border-red-300 bg-red-50 p-2 text-xs text-red-800 dark:border-red-800 dark:bg-red-950 dark:text-red-200">
                  {decodeURIComponent(searchParams.ds_error)}
                </div>
              )}
              {searchParams.ds_ok && (
                <div className="mt-3 rounded border border-emerald-300 bg-emerald-50 p-2 text-xs text-emerald-800 dark:border-emerald-800 dark:bg-emerald-950 dark:text-emerald-200">
                  {searchParams.ds_ok === "swap_pr_opened" ? "Swap PR opened — merging will deploy the new design system." : "Overrides saved."}
                </div>
              )}

              <form action={swapDesignSystem} style={{ marginTop: 16 }}>
                <input type="hidden" name="ws_slug" value={params.ws_slug} />
                <input type="hidden" name="app_slug" value={params.app_slug} />
                <input type="hidden" name="app_id" value={app.id} />
                <p style={{ fontSize: 12, color: "var(--fg-2)" }}>
                  Choosing a different Design System opens a pull request against the App's portal-bundle
                  repository. The PR carries the diff of the token sheet, the regenerated Tailwind config
                  and the new font preload manifest; merging it triggers a fresh deployment.
                </p>
                <label className="block text-sm" style={{ marginTop: 8 }}>
                  <span className="mb-1 block font-medium">Target Design System</span>
                  <select
                    name="target_ref"
                    className="w-full rounded border border-neutral-300 px-3 py-2 text-sm dark:border-neutral-700 dark:bg-neutral-800"
                  >
                    {designSystemCatalog.map((entry) => {
                      const value = entry.built_in_template && entry.asset_id.endsWith("desing-system-1")
                        ? "ds-forge-default"
                        : `${entry.asset_id}@${entry.version}`;
                      return (
                        <option key={`${entry.asset_id}@${entry.version}`} value={value}>
                          {entry.name} ({value})
                        </option>
                      );
                    })}
                  </select>
                </label>
                <label className="block text-sm" style={{ marginTop: 8 }}>
                  <span className="mb-1 block font-medium">Reason (optional)</span>
                  <input
                    name="reason"
                    placeholder="e.g. Corporate refresh"
                    className="w-full rounded border border-neutral-300 px-3 py-2 text-sm dark:border-neutral-700 dark:bg-neutral-800"
                  />
                </label>
                <button className="mt-3 rounded bg-neutral-900 px-3 py-1.5 text-sm font-medium text-white dark:bg-neutral-100 dark:text-neutral-900">
                  Open swap PR
                </button>
              </form>

              <details style={{ marginTop: 16 }}>
                <summary style={{ cursor: "pointer", fontSize: 13, fontWeight: 500 }}>Per-component overrides</summary>
                <form action={patchOverrides} style={{ marginTop: 8 }}>
                  <input type="hidden" name="ws_slug" value={params.ws_slug} />
                  <input type="hidden" name="app_slug" value={params.app_slug} />
                  <input type="hidden" name="app_id" value={app.id} />
                  <p style={{ fontSize: 12, color: "var(--fg-2)" }}>
                    Override surface tokens (color, typography, border radius, shadow) per component. Layout
                    tokens cannot be overridden — the build-time merger rejects them with a clear error.
                  </p>
                  <div style={{ display: "grid", gridTemplateColumns: "140px 1fr", gap: "6px 12px", marginTop: 8 }}>
                    {CANONICAL_COMPONENT_PRIMITIVES.map((component) => (
                      <>
                        <label key={`label-${component}`} htmlFor={`override_${component}`} style={{ fontSize: 12, alignSelf: "center" }}>
                          {component}
                        </label>
                        <input
                          key={`input-${component}`}
                          id={`override_${component}`}
                          name={`override_${component}`}
                          defaultValue={dsOverrides[component] ?? ""}
                          placeholder="e.g. design_system:platform:desing-system-3@2.0.0"
                          className="w-full rounded border border-neutral-300 px-2 py-1 text-xs dark:border-neutral-700 dark:bg-neutral-800"
                        />
                      </>
                    ))}
                  </div>
                  <button className="mt-2 rounded bg-neutral-900 px-3 py-1 text-xs font-medium text-white dark:bg-neutral-100 dark:text-neutral-900">
                    Save overrides
                  </button>
                </form>
              </details>
            </section>
          </>
        )}
      </section>
    </div>
  );
}
