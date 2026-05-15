/**
 * Portal proxy for recording match dismissals (alfred-console-redesign §6.6).
 * Emits alfred.intent.match_dismissed.v1 via the Alfred service.
 */
import { NextRequest, NextResponse } from "next/server";
import { authToken, correlationId, endpoint } from "@/lib/api";

export async function POST(req: NextRequest) {
  const body = (await req.json().catch(() => null)) as Record<string, unknown> | null;
  const { token } = await authToken();
  const correlation = correlationId();
  try {
    const r = await fetch(`${endpoint("ALFRED_URL")}/v1/intent/match_dismissed`, {
      method: "POST",
      headers: {
        "content-type": "application/json",
        "x-correlation-id": correlation,
        ...(token ? { authorization: `Bearer ${token}` } : {}),
      },
      body: JSON.stringify({ ...(body ?? {}), correlation_id: correlation }),
    });
    const data = await r.json().catch(() => ({}));
    return NextResponse.json(data, { status: r.status });
  } catch (err) {
    return NextResponse.json({ error: (err as Error).message }, { status: 502 });
  }
}
