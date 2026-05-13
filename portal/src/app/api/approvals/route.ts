import { NextRequest, NextResponse } from "next/server";
import { authToken, correlationId, endpoint, proxyJson } from "@/lib/api";
import { traceSpan } from "@/instrumentation";
import type { Approval } from "@/components/approvals/ApprovalCard";

export async function GET(req: NextRequest) {
  const status = req.nextUrl.searchParams.get("status") ?? "pending";
  const limit = req.nextUrl.searchParams.get("limit") ?? "10";
  const { token, actor } = await authToken();
  const approver = req.nextUrl.searchParams.get("approver") ?? actor;
  const correlation = correlationId();
  return traceSpan(
    "portal.api.approvals",
    {
      "http.route": "/api/approvals",
      "approvals.status": status,
      "approvals.limit": limit,
      "x-correlation-id": correlation,
    },
    async () => {
      try {
        const data = await proxyJson<{ approvals: Approval[] }>(
          `${endpoint("APPROVALS_URL")}/v1/approvals?status=${encodeURIComponent(status)}&approver=${encodeURIComponent(approver)}&limit=${encodeURIComponent(limit)}`,
          { token, correlation },
        );
        return NextResponse.json(data);
      } catch (err) {
        return NextResponse.json({ approvals: [], error: (err as Error).message }, { status: 502 });
      }
    },
  );
}
