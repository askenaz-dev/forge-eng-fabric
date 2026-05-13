import { NextRequest, NextResponse } from "next/server";
import { authToken, correlationId, endpoint } from "@/lib/api";

export type Workspace = {
  id: string;
  tenant_id: string;
  business_unit_id: string;
  name: string;
  description?: string;
};

export async function GET(req: NextRequest) {
  const { token } = await authToken();
  const correlation = correlationId();
  const tenantId = req.nextUrl.searchParams.get("tenant_id") ?? "";
  try {
    const resp = await fetch(`${endpoint("CONTROL_PLANE_URL")}/v1/workspaces`, {
      headers: {
        ...(token ? { authorization: `Bearer ${token}` } : {}),
        "x-correlation-id": correlation,
      },
      cache: "no-store",
    });
    if (!resp.ok) {
      return NextResponse.json({ workspaces: [], error: `control-plane ${resp.status}` }, { status: 502 });
    }
    const body = await resp.json();
    const list: Workspace[] = Array.isArray(body) ? body : body?.workspaces ?? [];
    const workspaces = tenantId ? list.filter((w) => w.tenant_id === tenantId) : list;
    return NextResponse.json({ workspaces });
  } catch (err) {
    return NextResponse.json({ workspaces: [], error: (err as Error).message }, { status: 502 });
  }
}
