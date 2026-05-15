import { NextRequest, NextResponse } from "next/server";
import { authToken, correlationId, endpoint } from "@/lib/api";

export async function POST(req: NextRequest) {
  const { token } = await authToken();
  const correlation = correlationId();
  let body: unknown;
  try {
    body = await req.json();
  } catch {
    return NextResponse.json({ error: "invalid_json" }, { status: 400 });
  }

  try {
    const r = await fetch(`${endpoint("WORKFLOW_RUNTIME_URL")}/v1/runs`, {
      method: "POST",
      headers: {
        "content-type": "application/json",
        "x-correlation-id": correlation,
        ...(token ? { authorization: `Bearer ${token}` } : {}),
      },
      body: JSON.stringify(body),
    });
    const data = await r.json().catch(() => ({}));
    return NextResponse.json(data, { status: r.status });
  } catch (err) {
    return NextResponse.json({ error: (err as Error).message }, { status: 502 });
  }
}
