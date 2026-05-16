/**
 * Portal proxy for /v1/apps/{id} (Application service).
 *
 * alfred-design-system-picker (D4): the Friendly view uses PATCH here to
 * apply the picker's selection until the alfred service propagates the ref
 * through `/v1/intent/start` (follow-up). The atomic POST path is exercised
 * by /alfred/wizard via `POST /v1/workspaces/{ws}/apps` directly.
 */
import { NextRequest, NextResponse } from "next/server";
import { authToken, correlationId, endpoint } from "@/lib/api";

type RouteCtx = { params: { id: string } };

export async function GET(_req: NextRequest, ctx: RouteCtx) {
  const { token } = await authToken();
  const correlation = correlationId();
  try {
    const r = await fetch(`${endpoint("APPLICATION_URL")}/v1/apps/${ctx.params.id}`, {
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

export async function PATCH(req: NextRequest, ctx: RouteCtx) {
  const body = await req.json().catch(() => ({}));
  const { token } = await authToken();
  const correlation = correlationId();
  try {
    const r = await fetch(`${endpoint("APPLICATION_URL")}/v1/apps/${ctx.params.id}`, {
      method: "PATCH",
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
