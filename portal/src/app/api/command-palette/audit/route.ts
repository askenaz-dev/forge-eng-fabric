import { NextRequest, NextResponse } from "next/server";
import { authToken, correlationId, emitAudit } from "@/lib/api";

export async function POST(req: NextRequest) {
  const body = await req.json().catch(() => ({}));
  const source = String(body.source ?? "unknown");
  const targetId = String(body.target_id ?? "");
  const query = String(body.query ?? "").slice(0, 200);
  const { actor } = await authToken();
  await emitAudit({
    type: "portal.command.invoked",
    principal: actor,
    data: { source, target_id: targetId, query },
    correlation: correlationId(),
  });
  return NextResponse.json({ ok: true });
}
