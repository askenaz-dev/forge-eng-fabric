import { NextRequest, NextResponse } from "next/server";
import { authToken, correlationId, endpoint, proxyJson } from "@/lib/api";
import { traceSpan } from "@/instrumentation";
import type { Run } from "@/components/runs/RunRow";

export async function GET(req: NextRequest) {
  const limit = req.nextUrl.searchParams.get("limit") ?? "50";
  const order = req.nextUrl.searchParams.get("order") ?? "desc";
  const { token } = await authToken();
  const correlation = correlationId();
  return traceSpan(
    "portal.api.sdlc.runs",
    { "http.route": "/api/sdlc/runs", "sdlc.limit": limit, "sdlc.order": order, "x-correlation-id": correlation },
    async () => {
      try {
        const data = await proxyJson<{ runs: Run[] }>(
          `${endpoint("SDLC_URL")}/v1/runs?limit=${encodeURIComponent(limit)}&order=${encodeURIComponent(order)}`,
          { token, correlation },
        );
        return NextResponse.json(data);
      } catch (err) {
        return NextResponse.json({ runs: [], error: (err as Error).message }, { status: 502 });
      }
    },
  );
}
