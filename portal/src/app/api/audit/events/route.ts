import { NextRequest, NextResponse } from "next/server";
import { authToken, correlationId, endpoint, proxyJson } from "@/lib/api";

export type AuditEvent = {
  id: string;
  type: string;
  principal: string;
  timestamp: string;
  data: Record<string, unknown>;
};

export async function GET(req: NextRequest) {
  const limit = req.nextUrl.searchParams.get("limit") ?? "20";
  const scope = req.nextUrl.searchParams.get("scope") ?? "workspace";
  const { token } = await authToken();
  const correlation = correlationId();
  try {
    const data = await proxyJson<{ events: AuditEvent[] }>(
      `${endpoint("AUDIT_URL")}/v1/events?limit=${encodeURIComponent(limit)}&scope=${encodeURIComponent(scope)}`,
      { token, correlation },
    );
    return NextResponse.json(data);
  } catch (err) {
    return NextResponse.json({ events: [], error: (err as Error).message }, { status: 502 });
  }
}
