/**
 * Portal proxy for POST /v1/intent/:draft_id/answer.
 * (alfred-console-redesign §5.3)
 */
import { NextRequest, NextResponse } from "next/server";
import { authToken, correlationId, endpoint } from "@/lib/api";

export async function POST(req: NextRequest) {
  const body = (await req.json().catch(() => null)) as {
    draft_id?: string;
    answer?: string;
    view?: string;
    field_updates?: Record<string, unknown>;
  } | null;
  const draftId = body?.draft_id;
  if (!draftId) {
    return NextResponse.json({ error: "draft_id is required" }, { status: 400 });
  }
  const { token } = await authToken();
  const correlation = correlationId();
  try {
    const r = await fetch(`${endpoint("ALFRED_URL")}/v1/intent/${encodeURIComponent(draftId)}/answer`, {
      method: "POST",
      headers: {
        "content-type": "application/json",
        "x-correlation-id": correlation,
        ...(token ? { authorization: `Bearer ${token}` } : {}),
      },
      body: JSON.stringify({
        answer: body?.answer ?? "",
        view: body?.view ?? "friendly",
        field_updates: body?.field_updates ?? {},
      }),
    });
    const data = await r.json();
    return NextResponse.json(data, { status: r.status });
  } catch (err) {
    return NextResponse.json({ error: (err as Error).message }, { status: 502 });
  }
}
