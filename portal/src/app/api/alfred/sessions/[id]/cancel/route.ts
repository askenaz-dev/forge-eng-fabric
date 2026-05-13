import { NextRequest, NextResponse } from "next/server";
import { authToken, correlationId, endpoint, proxyJson } from "@/lib/api";

export async function POST(
  req: NextRequest,
  { params }: { params: { id: string } },
) {
  const body = (await req.json().catch(() => ({}))) as { reason?: string } | null;
  const { token } = await authToken();
  const correlation = correlationId();
  try {
    const data = await proxyJson<{ session_id: string; status: string }>(
      `${endpoint("ALFRED_URL")}/v1/agent-mode/sessions/${params.id}/cancel`,
      {
        method: "POST",
        token,
        correlation,
        body: JSON.stringify({ reason: body?.reason ?? "cancelled from portal" }),
      },
    );
    return NextResponse.json(data);
  } catch (err) {
    return NextResponse.json({ error: (err as Error).message }, { status: 502 });
  }
}
