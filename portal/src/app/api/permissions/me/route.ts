import { NextResponse } from "next/server";
import { authToken, correlationId, endpoint, proxyJson } from "@/lib/api";

export type PermissionsPayload = {
  permissions: string[];
  // Alfred dock launcher gates — gathered from the permissions service for the
  // active workspace. See openspec/changes/alfred-agent-mode-orchestrator/.
  alfred_invoke?: boolean;
  alfred_agent_mode_run?: boolean;
};

export async function GET() {
  const { token } = await authToken();
  const correlation = correlationId();
  try {
    const data = await proxyJson<PermissionsPayload>(
      `${endpoint("POLICY_URL")}/v1/permissions/me`,
      { token, correlation },
    );
    const permissions = data.permissions ?? [];
    return NextResponse.json({
      permissions,
      alfred_invoke:
        typeof data.alfred_invoke === "boolean"
          ? data.alfred_invoke
          : permissions.includes("alfred:invoke"),
      alfred_agent_mode_run:
        typeof data.alfred_agent_mode_run === "boolean"
          ? data.alfred_agent_mode_run
          : permissions.includes("alfred:agent-mode.run"),
    });
  } catch {
    // No permission service available — return a permissive default so the
    // sidebar still renders all items.
    return NextResponse.json({
      permissions: ["policy:read", "audit:read", "admin"],
      alfred_invoke: true,
      alfred_agent_mode_run: true,
    });
  }
}
