import { NextRequest, NextResponse } from "next/server";

// Lightweight client telemetry sink for portal-side Alfred events:
// portal.alfred.dock_opened.v1, dock_closed, dock_session_started,
// dock_follow_up_sent, dock_navigated_to_artifact.
//
// In production these go to the audit ingestion pipeline; here we just log
// them so the contract surface is stable while wiring catches up.
export async function POST(req: NextRequest) {
  const body = (await req.json().catch(() => null)) as
    | { type?: string; data?: Record<string, unknown> }
    | null;
  if (!body?.type) {
    return NextResponse.json({ error: "type is required" }, { status: 400 });
  }
  console.info(JSON.stringify({ portal_alfred_event: body.type, data: body.data ?? {} }));
  return NextResponse.json({ ok: true });
}
