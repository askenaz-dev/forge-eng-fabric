import { getServerSession } from "next-auth";
import { authOptions } from "@/auth";
import { headers } from "next/headers";
import { randomUUID } from "crypto";

export type Endpoint =
  | "CONTROL_PLANE_URL"
  | "APPROVALS_URL"
  | "SDLC_URL"
  | "REGISTRY_URL"
  | "AUDIT_URL"
  | "OBS_URL"
  | "NOTIFICATIONS_URL"
  | "OPENSPEC_URL"
  | "DEPLOY_URL"
  | "INCIDENT_URL"
  | "EVOLUTION_URL"
  | "FINOPS_URL"
  | "POLICY_URL"
  | "RUNTIME_REGISTRY_URL"
  | "ALFRED_URL"
  | "HEALING_URL"
  | "WORKFLOW_RUNTIME_URL"
  | "APPLICATION_URL";

const DEFAULTS: Record<Endpoint, string> = {
  CONTROL_PLANE_URL:    "http://localhost:8081",
  APPROVALS_URL:        "http://localhost:8105",
  SDLC_URL:             "http://localhost:8086",
  REGISTRY_URL:         "http://localhost:8089",
  AUDIT_URL:            "http://localhost:8088",
  OBS_URL:              "http://localhost:8091",
  NOTIFICATIONS_URL:    "http://localhost:8092",
  OPENSPEC_URL:         "http://localhost:8094",
  DEPLOY_URL:           "http://localhost:8096",
  INCIDENT_URL:         "http://localhost:8097",
  EVOLUTION_URL:        "http://localhost:8098",
  FINOPS_URL:           "http://localhost:8099",
  POLICY_URL:           "http://localhost:8102",
  RUNTIME_REGISTRY_URL: "http://localhost:8110",
  ALFRED_URL:           "http://localhost:8090",
  HEALING_URL:          "http://localhost:8107",
  WORKFLOW_RUNTIME_URL: "http://localhost:8095",
  APPLICATION_URL:      "http://localhost:8095",
};

export function endpoint(name: Endpoint): string {
  return process.env[name] ?? DEFAULTS[name];
}

export async function authToken(): Promise<{ token?: string; actor: string }> {
  const session = await getServerSession(authOptions);
  const token = (session as { accessToken?: string } | null)?.accessToken;
  const actor = session?.user?.email ?? session?.user?.name ?? "anonymous";
  return { token, actor };
}

export function correlationId(): string {
  try {
    return headers().get("x-correlation-id") ?? randomUUID();
  } catch {
    return randomUUID();
  }
}

export async function proxyJson<T>(
  url: string,
  init: RequestInit & { token?: string; correlation?: string } = {},
): Promise<T> {
  const { token, correlation, headers: extra, ...rest } = init;
  const headerBag: Record<string, string> = {
    accept: "application/json",
    "x-correlation-id": correlation ?? randomUUID(),
    ...(extra as Record<string, string> | undefined),
  };
  if (token) headerBag.authorization = `Bearer ${token}`;
  if (rest.method && rest.method !== "GET" && !headerBag["content-type"]) {
    headerBag["content-type"] = "application/json";
  }
  const response = await fetch(url, { ...rest, headers: headerBag, cache: "no-store" });
  if (!response.ok) {
    const body = await response.text().catch(() => "");
    throw new Error(`upstream ${response.status} ${url}: ${body || "(no body)"}`);
  }
  return (await response.json()) as T;
}

export async function emitAudit(event: {
  type: string;
  principal: string;
  data: Record<string, unknown>;
  correlation: string;
}): Promise<void> {
  try {
    await fetch(`${endpoint("AUDIT_URL")}/v1/events`, {
      method: "POST",
      headers: {
        "content-type": "application/json",
        "x-correlation-id": event.correlation,
      },
      body: JSON.stringify({
        type: event.type,
        principal: event.principal,
        timestamp: new Date().toISOString(),
        data: event.data,
        correlation_id: event.correlation,
      }),
      keepalive: true,
    });
  } catch {
    // Audit failure must not affect the user-visible action.
  }
}
