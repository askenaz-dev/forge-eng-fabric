import { NextResponse } from "next/server";
import { authToken, correlationId, endpoint } from "@/lib/api";

export type Tenant = {
  id: string;
  name: string;
};

export async function GET() {
  const { token } = await authToken();
  const correlation = correlationId();
  try {
    const resp = await fetch(`${endpoint("CONTROL_PLANE_URL")}/v1/tenants`, {
      headers: {
        ...(token ? { authorization: `Bearer ${token}` } : {}),
        "x-correlation-id": correlation,
      },
      cache: "no-store",
    });
    if (!resp.ok) {
      return NextResponse.json({ tenants: [], error: `control-plane ${resp.status}` }, { status: 502 });
    }
    const body = await resp.json();
    const tenants: Tenant[] = Array.isArray(body) ? body : body?.tenants ?? [];
    return NextResponse.json({ tenants });
  } catch (err) {
    return NextResponse.json({ tenants: [], error: (err as Error).message }, { status: 502 });
  }
}
