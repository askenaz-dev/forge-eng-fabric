import { NextRequest, NextResponse } from "next/server";
import { authToken, correlationId, endpoint } from "@/lib/api";

export async function POST(req: NextRequest, { params }: { params: { id: string } }) {
  const body = await req.json().catch(() => ({}));
  const { token, actor } = await authToken();
  const correlation = correlationId();
  const upstream = await fetch(
    `${endpoint("APPROVALS_URL")}/v1/approvals/${encodeURIComponent(params.id)}/decisions`,
    {
      method: "POST",
      headers: {
        "content-type": "application/json",
        ...(token ? { authorization: `Bearer ${token}` } : {}),
        "x-correlation-id": correlation,
      },
      body: JSON.stringify({
        actor,
        decision: body.decision,
        comment: body.comment ?? "",
      }),
    },
  );
  const text = await upstream.text();
  let payload: unknown;
  try {
    payload = text ? JSON.parse(text) : {};
  } catch {
    payload = { raw: text };
  }
  return NextResponse.json(payload as object, { status: upstream.status });
}
