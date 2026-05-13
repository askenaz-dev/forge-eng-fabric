import { NextResponse } from "next/server";
import { authToken, correlationId, endpoint } from "@/lib/api";

export async function POST() {
  const { token, actor } = await authToken();
  const correlation = correlationId();
  if (token) {
    fetch(`${endpoint("NOTIFICATIONS_URL")}/v1/ack`, {
      method: "POST",
      headers: {
        "content-type": "application/json",
        authorization: `Bearer ${token}`,
        "x-correlation-id": correlation,
      },
      body: JSON.stringify({ principal: actor }),
    }).catch(() => undefined);
  }
  return NextResponse.json({ ok: true });
}
