import { authOptions } from "@/auth";
import { getServerSession } from "next-auth";
import { redirect } from "next/navigation";
import type { OnboardingEvent, OnboardingRequest, PipelineGateResult, RepoTemplate } from "./onboarding-types";

type PortalIdentity = { token?: string; user: string };
type TestStore = { requests: OnboardingRequest[]; events: Record<string, OnboardingEvent[]> };

declare global {
  // eslint-disable-next-line no-var
  var __forgePortalOnboardingTestStore: TestStore | undefined;
}

export const appOnboardingUrl = () => process.env.APP_ONBOARDING_URL ?? "http://localhost:8085";
export const portalTestMode = () => process.env.FORGE_PORTAL_TEST_MODE === "1";

export async function resolvePortalIdentity(): Promise<PortalIdentity | null> {
  if (portalTestMode()) return { token: "e2e-token", user: "e2e@forge.local" };
  const session = await getServerSession(authOptions);
  if (!session) return null;
  return {
    token: (session as { accessToken?: string }).accessToken,
    user: session.user?.email ?? session.user?.name ?? "portal-user",
  };
}

export async function requirePortalIdentity() {
  const identity = await resolvePortalIdentity();
  if (!identity) redirect("/api/auth/signin");
  return identity;
}

export async function fetchTemplates(token?: string) {
  if (portalTestMode()) return fixtureTemplates;
  const response = await fetch(`${appOnboardingUrl()}/v1/templates`, {
    headers: authHeaders(token),
    cache: "no-store",
  });
  if (!response.ok) throw new Error(`app-onboarding templates ${response.status}: ${await response.text()}`);
  return ((await response.json()) as { templates: RepoTemplate[] }).templates;
}

export async function fetchOnboardingRequests(filters: { workspace_id?: string; status?: string }, token?: string) {
  if (portalTestMode()) {
    const store = testStore();
    return store.requests.filter((request) => {
      if (filters.workspace_id && request.workspace_id !== filters.workspace_id) return false;
      if (filters.status && request.status !== filters.status) return false;
      return true;
    });
  }
  const params = new URLSearchParams();
  if (filters.workspace_id) params.set("workspace_id", filters.workspace_id);
  if (filters.status) params.set("status", filters.status);
  const response = await fetch(`${appOnboardingUrl()}/v1/onboarding?${params}`, {
    headers: authHeaders(token),
    cache: "no-store",
  });
  if (!response.ok) throw new Error(`app-onboarding history ${response.status}: ${await response.text()}`);
  return ((await response.json()) as { requests: OnboardingRequest[] }).requests;
}

export async function fetchOnboardingRequest(id: string, token?: string) {
  if (portalTestMode()) return testStore().requests.find((request) => request.id === id) ?? null;
  const response = await fetch(`${appOnboardingUrl()}/v1/onboarding/${id}`, {
    headers: authHeaders(token),
    cache: "no-store",
  });
  if (response.status === 404) return null;
  if (!response.ok) throw new Error(`app-onboarding request ${response.status}: ${await response.text()}`);
  return (await response.json()) as OnboardingRequest;
}

export async function fetchOnboardingTimeline(id: string, token?: string) {
  if (portalTestMode()) return testStore().events[id] ?? [];
  const response = await fetch(`${appOnboardingUrl()}/v1/onboarding/${id}/timeline`, {
    headers: authHeaders(token),
    cache: "no-store",
  });
  if (!response.ok) throw new Error(`app-onboarding timeline ${response.status}: ${await response.text()}`);
  return (await response.json()) as OnboardingEvent[];
}

export async function fetchPipelineGates(filters: { workspace_id?: string; repo?: string; pr?: string }, token?: string) {
  if (portalTestMode()) return fixtureGates.filter((gate) => !filters.repo || gate.repo_full_name === filters.repo);
  const params = new URLSearchParams();
  if (filters.workspace_id) params.set("workspace_id", filters.workspace_id);
  if (filters.repo) params.set("repo", filters.repo);
  if (filters.pr) params.set("pr", filters.pr);
  const response = await fetch(`${appOnboardingUrl()}/v1/pipeline-gates?${params}`, {
    headers: authHeaders(token),
    cache: "no-store",
  });
  if (!response.ok) throw new Error(`pipeline gates ${response.status}: ${await response.text()}`);
  return ((await response.json()) as { results: PipelineGateResult[] }).results;
}

export async function submitOnboarding(payload: Record<string, unknown>, token?: string, user = "portal-user") {
  if (portalTestMode()) return submitFixtureOnboarding(payload, user);
  const response = await fetch(`${appOnboardingUrl()}/v1/onboarding`, {
    method: "POST",
    headers: { "content-type": "application/json", "X-Forge-Principal": user, ...authHeaders(token) },
    body: JSON.stringify(payload),
    cache: "no-store",
  });
  if (!response.ok) throw new Error(`app-onboarding submit ${response.status}: ${await response.text()}`);
  return (await response.json()) as OnboardingRequest;
}

function authHeaders(token?: string): Record<string, string> {
  return token ? { authorization: `Bearer ${token}` } : {};
}

function testStore(): TestStore {
  globalThis.__forgePortalOnboardingTestStore ??= { requests: [], events: {} };
  return globalThis.__forgePortalOnboardingTestStore;
}

function submitFixtureOnboarding(payload: Record<string, unknown>, user: string) {
  const store = testStore();
  const now = new Date();
  const id = `req-e2e-${store.requests.length + 1}`;
  const request: OnboardingRequest = {
    id,
    workspace_id: String(payload.workspace_id ?? "ws-e2e"),
    tenant_id: String(payload.tenant_id ?? "tn-e2e"),
    repo_org: String(payload.repo_org ?? "forge-pilot"),
    repo_name: String(payload.repo_name ?? "svc-e2e"),
    template_id: String(payload.template_id ?? "go-microservice"),
    template_version: String(payload.template_version ?? "1.0.0"),
    parameters: (payload.parameters as Record<string, unknown>) ?? {},
    criticality: String(payload.criticality ?? "medium"),
    data_classification: String(payload.data_classification ?? "internal"),
    owners: Array.isArray(payload.owners) ? payload.owners.map(String) : ["@platform"],
    status: "completed",
    asset_id: `application:${String(payload.workspace_id ?? "ws-e2e")}:${String(payload.repo_name ?? "svc-e2e")}`,
    correlation_id: `corr-${id}`,
    requested_by: user,
    created_at: now.toISOString(),
    completed_at: new Date(now.getTime() + 42_000).toISOString(),
  };
  store.requests.unshift(request);
  store.events[id] = [
    fixtureEvent(id, "request.received", "started"),
    fixtureEvent(id, "policy.evaluate", "completed"),
    fixtureEvent(id, "scaffold.render", "completed"),
    fixtureEvent(id, "github.create_repo", "completed"),
    fixtureEvent(id, "asset.register", "completed"),
  ];
  return request;
}

function fixtureEvent(requestId: string, stage: string, outcome: string): OnboardingEvent {
  return {
    id: `${requestId}-${stage}`,
    request_id: requestId,
    stage,
    outcome,
    duration_ms: outcome === "completed" ? 250 : 0,
    created_at: new Date().toISOString(),
  };
}

const fixtureTemplates: RepoTemplate[] = [
  {
    id: "go-microservice",
    version: "1.0.0",
    description: "Minimal Go microservice template for Forge onboarding",
    category: "microservice",
    lifecycle_state: "approved",
    trust_level: "T3",
    parameters: { name: { type: "string", required: true, pattern: "^[a-z][a-z0-9-]{1,40}$" }, runtime: { type: "string", default: "go1.22" } },
    required_capabilities: ["ci-pipeline-baseline", "mcp-and-skills"],
  },
  {
    id: "nextjs-frontend",
    version: "1.0.0",
    description: "Minimal Next.js frontend template for Forge onboarding",
    category: "frontend",
    lifecycle_state: "approved",
    trust_level: "T3",
    parameters: { name: { type: "string", required: true }, runtime: { type: "string", default: "node20" } },
    required_capabilities: ["ci-pipeline-baseline", "mcp-and-skills"],
  },
];

const fixtureGates: PipelineGateResult[] = [
  { workspace_id: "ws-e2e", repo_full_name: "forge-pilot/svc-e2e", pr_number: 1, commit_sha: "abc123", stage: "lint", tool: "golangci-lint", outcome: "pass", report_url: "https://logs.example/lint", policy_version: "phase-2", created_at: new Date().toISOString() },
  { workspace_id: "ws-e2e", repo_full_name: "forge-pilot/svc-e2e", pr_number: 1, commit_sha: "abc123", stage: "sast", tool: "semgrep", outcome: "pass", report_url: "https://logs.example/sast", policy_version: "phase-2", created_at: new Date().toISOString() },
  { workspace_id: "ws-e2e", repo_full_name: "forge-pilot/svc-e2e", pr_number: 1, commit_sha: "abc123", stage: "sign", tool: "cosign", outcome: "pass", report_url: "https://logs.example/sign", policy_version: "phase-2", created_at: new Date().toISOString() },
];
