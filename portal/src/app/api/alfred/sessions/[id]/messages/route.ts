import { NextRequest, NextResponse } from "next/server";
import { authToken, correlationId, endpoint, proxyJson } from "@/lib/api";

export async function POST(
  req: NextRequest,
  { params }: { params: { id: string } },
) {
  const body = (await req.json().catch(() => null)) as { intent?: string } | null;
  if (!body?.intent) {
    return NextResponse.json({ error: "intent is required" }, { status: 400 });
  }
  const { token } = await authToken();
  const correlation = correlationId();
  try {
    const data = await proxyJson<{ session_id: string; status: string; appended_idx: number }>(
      `${endpoint("ALFRED_URL")}/v1/agent-mode/sessions/${params.id}/messages`,
      {
        method: "POST",
        token,
        correlation,
        body: JSON.stringify({ intent: body.intent }),
      },
    );
    return NextResponse.json(data);
  } catch (err) {
    return NextResponse.json({ error: (err as Error).message }, { status: 502 });
  }
}
