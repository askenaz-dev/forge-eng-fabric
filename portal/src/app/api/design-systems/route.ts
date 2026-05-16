/**
 * Portal proxy for GET /v1/design-systems (Registry catalog endpoint).
 *
 * alfred-design-system-picker (D2): the Friendly view's picker step calls
 * this from the browser to render the catalog. Server-side fetch keeps the
 * Registry URL and auth token out of the client bundle.
 */
import { NextRequest, NextResponse } from "next/server";
import { authToken, correlationId, endpoint } from "@/lib/api";

export async function GET(_req: NextRequest) {
  const { token } = await authToken();
  const correlation = correlationId();
  try {
    const r = await fetch(`${endpoint("REGISTRY_URL")}/v1/design-systems`, {
      headers: {
        accept: "application/json",
        "x-correlation-id": correlation,
        ...(token ? { authorization: `Bearer ${token}` } : {}),
      },
      cache: "no-store",
    });
    const data = await r.json().catch(() => ({}));
    return NextResponse.json(data, { status: r.status });
  } catch (err) {
    return NextResponse.json({ error: (err as Error).message }, { status: 502 });
  }
}
