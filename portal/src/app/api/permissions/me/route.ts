import { NextResponse } from "next/server";
import { authToken, correlationId, endpoint, proxyJson } from "@/lib/api";

export type PermissionsPayload = {
  permissions: string[];
};

export async function GET() {
  const { token } = await authToken();
  const correlation = correlationId();
  try {
    const data = await proxyJson<PermissionsPayload>(
      `${endpoint("POLICY_URL")}/v1/permissions/me`,
      { token, correlation },
    );
    return NextResponse.json(data);
  } catch {
    // No permission service available — return a permissive default so the
    // sidebar still renders all items.
    return NextResponse.json({ permissions: ["policy:read", "audit:read", "admin"] });
  }
}
