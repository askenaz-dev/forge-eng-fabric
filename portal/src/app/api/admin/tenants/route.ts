import { NextRequest, NextResponse } from "next/server";
import { authToken, correlationId, endpoint } from "@/lib/api";

export async function GET() {
  const { token } = await authToken();
  const correlation = correlationId();
  const upstream = await fetch(`${endpoint("CONTROL_PLANE_URL")}/v1/tenants`, {
    headers: {
      ...(token ? { authorization: `Bearer ${token}` } : {}),
      "x-correlation-id": correlation,
    },
    cache: "no-store",
  });
  const text = await upstream.text();
  return new NextResponse(text, {
    status: upstream.status,
    headers: { "content-type": upstream.headers.get("content-type") ?? "application/json" },
  });
}

export async function POST(req: NextRequest) {
  const body = await req.json().catch(() => ({}));
  const { token } = await authToken();
  const correlation = correlationId();
  const upstream = await fetch(`${endpoint("CONTROL_PLANE_URL")}/v1/tenants`, {
    method: "POST",
    headers: {
      "content-type": "application/json",
      ...(token ? { authorization: `Bearer ${token}` } : {}),
      "x-correlation-id": correlation,
    },
    body: JSON.stringify(body),
  });
  const text = await upstream.text();
  return new NextResponse(text, {
    status: upstream.status,
    headers: { "content-type": upstream.headers.get("content-type") ?? "application/json" },
  });
}
