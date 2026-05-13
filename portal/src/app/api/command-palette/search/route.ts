import { NextRequest, NextResponse } from "next/server";
import { authToken, correlationId, endpoint } from "@/lib/api";
import { traceSpan } from "@/instrumentation";
import type { PaletteResult, PaletteSourceId, PaletteSourceResponse } from "@/components/palette/types";
import { NAV_GROUPS } from "@/components/shell/nav";
import { DICTIONARY } from "@/i18n/dictionary";

const PER_SOURCE_TIMEOUT_MS = 1500;

function withTimeout<T>(promise: Promise<T>): Promise<{ status: "ok"; value: T } | { status: "unreachable" }> {
  return new Promise((resolve) => {
    const handle = setTimeout(() => resolve({ status: "unreachable" }), PER_SOURCE_TIMEOUT_MS);
    promise.then(
      (value) => {
        clearTimeout(handle);
        resolve({ status: "ok", value });
      },
      () => {
        clearTimeout(handle);
        resolve({ status: "unreachable" });
      },
    );
  });
}

async function fetchJson<T>(url: string, token?: string, correlation?: string): Promise<T> {
  const headers: Record<string, string> = { accept: "application/json" };
  if (token) headers.authorization = `Bearer ${token}`;
  if (correlation) headers["x-correlation-id"] = correlation;
  const r = await fetch(url, { headers, cache: "no-store" });
  if (!r.ok) throw new Error(`${url} ${r.status}`);
  return (await r.json()) as T;
}

export async function GET(req: NextRequest) {
  const q = req.nextUrl.searchParams.get("q")?.trim() ?? "";
  const { token, actor } = await authToken();
  const correlation = correlationId();
  return traceSpan(
    "portal.command-palette.search",
    { "http.route": "/api/command-palette/search", "palette.q": q.slice(0, 60), "x-correlation-id": correlation },
    () => doSearch(q, token, actor, correlation),
  );
}

async function doSearch(q: string, token: string | undefined, actor: string, correlation: string) {

  // Nav source — synchronous, no upstream.
  const navResults: PaletteResult[] = NAV_GROUPS.flatMap((g) =>
    g.items
      .filter((i) => {
        if (!q) return true;
        const label = DICTIONARY.es[i.labelKey].toLowerCase();
        const en = DICTIONARY.en[i.labelKey].toLowerCase();
        return label.includes(q.toLowerCase()) || en.includes(q.toLowerCase());
      })
      .map((i) => ({
        id: `nav.${i.id}`,
        source: "nav" as const,
        title: DICTIONARY.es[i.labelKey],
        subtitle: i.href,
        hrefOrAction: { kind: "navigate" as const, href: i.href },
      })),
  );

  type AssetResp = { items?: Array<{ id: string; name: string; kind: string; slug?: string }> };
  type RunResp = { runs?: Array<{ id: string; agent: string; purpose: string; repo: string }> };
  type ApprovalResp = { approvals?: Array<{ id: string; title?: string; rationale?: string; action?: string }> };
  type SpecResp = { items?: Array<{ id: string; title: string }>; specs?: Array<{ id: string; title: string }> };
  type WorkspaceResp = { workspaces?: Array<{ id: string; name: string; tenant_id: string }> };
  type TenantResp = { tenants?: Array<{ id: string; name: string }> };

  const queryParam = q ? `&q=${encodeURIComponent(q)}` : "";

  // All upstream calls fire in parallel with per-source timeouts.
  const [agents, skills, mcp, runs, approvals, specs, workspaces, tenants] = await Promise.all([
    withTimeout(fetchJson<AssetResp>(`${endpoint("REGISTRY_URL")}/v1/assets?kind=agent&status=approved&limit=15${queryParam}`, token, correlation)),
    withTimeout(fetchJson<AssetResp>(`${endpoint("REGISTRY_URL")}/v1/assets?kind=skill&status=approved&limit=15${queryParam}`, token, correlation)),
    withTimeout(fetchJson<AssetResp>(`${endpoint("REGISTRY_URL")}/v1/assets?kind=mcp&status=approved&limit=15${queryParam}`, token, correlation)),
    withTimeout(fetchJson<RunResp>(`${endpoint("SDLC_URL")}/v1/runs?limit=20&order=desc${queryParam}`, token, correlation)),
    withTimeout(fetchJson<ApprovalResp>(`${endpoint("APPROVALS_URL")}/v1/approvals?status=pending&approver=${encodeURIComponent(actor)}&limit=20`, token, correlation)),
    withTimeout(fetchJson<SpecResp>(`${endpoint("OPENSPEC_URL")}/v1/openspecs?limit=15${queryParam}`, token, correlation)),
    withTimeout(fetchJson<WorkspaceResp>(`${endpoint("CONTROL_PLANE_URL")}/v1/workspaces?limit=20`, token, correlation)),
    withTimeout(fetchJson<TenantResp>(`${endpoint("CONTROL_PLANE_URL")}/v1/tenants/me`, token, correlation)),
  ]);

  function emptyOrResults<T>(
    source: PaletteSourceId,
    result: { status: "ok"; value: T } | { status: "unreachable" },
    mapper: (value: T) => PaletteResult[],
  ): PaletteSourceResponse {
    if (result.status === "unreachable") {
      return { source, status: "unreachable", results: [] };
    }
    return { source, status: "ok", results: mapper(result.value) };
  }

  const sources: PaletteSourceResponse[] = [
    { source: "nav", status: "ok", results: navResults },
    emptyOrResults<AssetResp>("agents", agents, (v) =>
      (v.items ?? []).map((a) => ({
        id: `agent.${a.id}`,
        source: "agents",
        title: a.name,
        subtitle: a.slug ?? a.id,
        hrefOrAction: { kind: "navigate", href: `/assets?kind=agent&id=${a.id}` },
      })),
    ),
    emptyOrResults<AssetResp>("skills", skills, (v) =>
      (v.items ?? []).map((s) => ({
        id: `skill.${s.id}`,
        source: "skills",
        title: s.name,
        subtitle: s.slug ?? s.id,
        hrefOrAction: { kind: "navigate", href: `/assets?kind=skill&id=${s.id}` },
      })),
    ),
    emptyOrResults<AssetResp>("mcp", mcp, (v) =>
      (v.items ?? []).map((m) => ({
        id: `mcp.${m.id}`,
        source: "mcp",
        title: m.name,
        subtitle: m.slug ?? m.id,
        hrefOrAction: { kind: "navigate", href: `/assets?kind=mcp&id=${m.id}` },
      })),
    ),
    emptyOrResults<RunResp>("runs", runs, (v) =>
      (v.runs ?? []).map((r) => ({
        id: `run.${r.id}`,
        source: "runs",
        title: r.purpose,
        subtitle: `${r.agent} · ${r.repo} · ${r.id}`,
        hrefOrAction: { kind: "navigate", href: `/?run=${r.id}` },
      })),
    ),
    emptyOrResults<ApprovalResp>("approvals", approvals, (v) =>
      (v.approvals ?? []).map((a) => ({
        id: `apr.${a.id}`,
        source: "approvals",
        title: a.title ?? a.action ?? a.id,
        subtitle: a.rationale ?? a.id,
        hrefOrAction: { kind: "navigate", href: `/approvals?focus=${a.id}` },
      })),
    ),
    emptyOrResults<SpecResp>("specs", specs, (v) => {
      const items = v.items ?? v.specs ?? [];
      return items.map((s) => ({
        id: `spec.${s.id}`,
        source: "specs",
        title: s.title,
        subtitle: s.id,
        hrefOrAction: { kind: "navigate", href: `/openspecs?id=${s.id}` },
      }));
    }),
    emptyOrResults<WorkspaceResp>("workspaces", workspaces, (v) =>
      (v.workspaces ?? []).map((w) => ({
        id: `workspace.${w.id}`,
        source: "workspaces",
        title: w.name,
        subtitle: `${w.tenant_id} · ${w.id}`,
        hrefOrAction: { kind: "action", action: { type: "workspace", tenant: w.tenant_id, workspace: w.id } },
      })),
    ),
    emptyOrResults<TenantResp>("tenants", tenants, (v) =>
      (v.tenants ?? []).map((tn) => ({
        id: `tenant.${tn.id}`,
        source: "tenants",
        title: tn.name,
        subtitle: tn.id,
        hrefOrAction: { kind: "navigate", href: `/?tenant=${tn.id}` },
      })),
    ),
  ];

  return NextResponse.json({ sources });
}
