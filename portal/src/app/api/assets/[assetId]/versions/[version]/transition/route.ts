import { NextRequest, NextResponse } from "next/server";
import { authToken, correlationId, endpoint } from "@/lib/api";

export async function POST(
  req: NextRequest,
  { params }: { params: { assetId: string; version: string } },
) {
  const body = await req.json().catch(() => ({}));
  const { token } = await authToken();
  const correlation = correlationId();
  const upstream = await fetch(
    `${endpoint("REGISTRY_URL")}/v1/assets/${encodeURIComponent(params.assetId)}/versions/${encodeURIComponent(params.version)}/transition`,
    {
      method: "POST",
      headers: {
        "content-type": "application/json",
        ...(token ? { authorization: `Bearer ${token}` } : {}),
        "x-correlation-id": correlation,
      },
      body: JSON.stringify(body),
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
