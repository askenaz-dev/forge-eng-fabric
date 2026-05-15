import { NextRequest, NextResponse } from "next/server";
import { authToken, correlationId, endpoint } from "@/lib/api";

export type BusinessUnit = {
  id: string;
  tenant_id: string;
  name: string;
};

// Control-plane only exposes BU listing scoped to a tenant. We require the
// caller to pass the tenant UUID; if missing, we list all tenants and union
// their BUs so the dropdown stays useful when the form has no tenant context.
export async function GET(req: NextRequest) {
  const { token } = await authToken();
  const correlation = correlationId();
  const tenantId = req.nextUrl.searchParams.get("tenant_id") ?? "";
  const headers = {
    ...(token ? { authorization: `Bearer ${token}` } : {}),
    "x-correlation-id": correlation,
  };
  try {
    if (tenantId) {
      const resp = await fetch(`${endpoint("CONTROL_PLANE_URL")}/v1/tenants/${encodeURIComponent(tenantId)}/business-units`, {
        headers,
        cache: "no-store",
      });
      if (!resp.ok) {
        return NextResponse.json({ business_units: [], error: `control-plane ${resp.status}` }, { status: 502 });
      }
      const body = await resp.json();
      const list: BusinessUnit[] = Array.isArray(body) ? body : body?.business_units ?? [];
      return NextResponse.json({ business_units: list });
    }
    // No tenant filter — use the flat /v1/business-units endpoint which is
    // FGA-scoped server-side.
    const resp = await fetch(`${endpoint("CONTROL_PLANE_URL")}/v1/business-units`, {
      headers,
      cache: "no-store",
    });
    if (!resp.ok) {
      return NextResponse.json(
        { business_units: [], error: `control-plane ${resp.status}` },
        { status: 502 },
      );
    }
    const body = await resp.json();
    const list: BusinessUnit[] = Array.isArray(body) ? body : body?.business_units ?? [];
    return NextResponse.json({ business_units: list });
  } catch (err) {
    return NextResponse.json({ business_units: [], error: (err as Error).message }, { status: 502 });
  }
}
