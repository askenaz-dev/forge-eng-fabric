import { NextRequest, NextResponse } from "next/server";
import { authToken, correlationId, endpoint, proxyJson } from "@/lib/api";
import { traceSpan } from "@/instrumentation";

export type KpiPayload = {
  runs_in_flight: { value: number; samples: number[]; delta_pct?: number };
  success_rate_24h: { value: number; samples: number[]; delta_pts?: number };
  p95_ms: { value: number | null; samples: number[]; delta_ms?: number };
  hours_saved: { value: number; samples: number[]; delta_h?: number };
};

export async function GET(req: NextRequest) {
  const window = req.nextUrl.searchParams.get("window") ?? "24h";
  const { token } = await authToken();
  const correlation = correlationId();
  return traceSpan(
    "portal.api.observability.kpis",
    { "http.route": "/api/observability/kpis", "kpi.window": window, "x-correlation-id": correlation },
    async () => {
      try {
        const data = await proxyJson<KpiPayload>(
          `${endpoint("OBS_URL")}/v1/kpis?window=${encodeURIComponent(window)}`,
          { token, correlation },
        );
        return NextResponse.json(data, { headers: { "x-correlation-id": correlation } });
      } catch (err) {
        return NextResponse.json({ error: (err as Error).message }, { status: 502 });
      }
    },
  );
}
