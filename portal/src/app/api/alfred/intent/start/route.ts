/**
 * Portal proxy for POST /v1/intent/start (Friendly view intent capture).
 * (alfred-console-redesign §5.1)
 */
import { NextRequest, NextResponse } from "next/server";
import { authToken, correlationId, endpoint } from "@/lib/api";

export async function POST(req: NextRequest) {
  const body = (await req.json().catch(() => null)) as Record<string, unknown> | null;
  if (!body?.workspace_id) {
    return NextResponse.json({ error: "workspace_id is required" }, { status: 400 });
  }
  const { token } = await authToken();
  const correlation = correlationId();
  try {
    const r = await fetch(`${endpoint("ALFRED_URL")}/v1/intent/start`, {
      method: "POST",
      headers: {
        "content-type": "application/json",
        "x-correlation-id": correlation,
        ...(token ? { authorization: `Bearer ${token}` } : {}),
      },
      body: JSON.stringify(body),
    });
    const data = await r.json();
    return NextResponse.json(data, { status: r.status });
  } catch (err) {
    return NextResponse.json({ error: (err as Error).message }, { status: 502 });
  }
}
