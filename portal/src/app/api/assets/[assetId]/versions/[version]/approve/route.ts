import { NextRequest, NextResponse } from "next/server";
import { authToken, correlationId, endpoint } from "@/lib/api";

export async function POST(
  req: NextRequest,
  { params }: { params: { assetId: string; version: string } },
) {
  const body = await req.json().catch(() => ({}));
  const { token, actor } = await authToken();
  const correlation = correlationId();
  const payload = {
    approval_id: typeof body.approval_id === "string" && body.approval_id ? body.approval_id : correlation,
    approved_by: typeof body.approved_by === "string" && body.approved_by ? body.approved_by : actor,
    comment: typeof body.comment === "string" ? body.comment : "",
    trust_level: typeof body.trust_level === "string" ? body.trust_level : "T3",
    eval_scores: body.eval_scores ?? {},
  };
  const upstream = await fetch(
    `${endpoint("REGISTRY_URL")}/v1/assets/${encodeURIComponent(params.assetId)}/versions/${encodeURIComponent(params.version)}/lifecycle-hooks/workspace-owner-approval`,
    {
      method: "POST",
      headers: {
        "content-type": "application/json",
        ...(token ? { authorization: `Bearer ${token}` } : {}),
        "x-correlation-id": correlation,
      },
      body: JSON.stringify(payload),
    },
  );
  const text = await upstream.text();
  let response: unknown;
  try {
    response = text ? JSON.parse(text) : {};
  } catch {
    response = { raw: text };
  }
  return NextResponse.json(response as object, { status: upstream.status });
}
