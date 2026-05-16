import { NextRequest, NextResponse } from "next/server";
import { authToken, correlationId, endpoint, proxyJson } from "@/lib/api";
import { traceSpan } from "@/instrumentation";
import type { Run, RunStatus } from "@/components/runs/RunRow";

type ExecutionStatus =
  | "pending"
  | "running"
  | "waiting"
  | "completed"
  | "failed"
  | "cancelled"
  | "compensating";

type Execution = {
  id: string;
  tenant_id: string;
  workspace_id: string;
  namespace: string;
  workflow_id: string;
  started_at: string;
  completed_at?: string;
  status: ExecutionStatus;
  inputs?: Record<string, unknown>;
};

function mapStatus(s: ExecutionStatus): RunStatus {
  switch (s) {
    case "completed":    return "success";
    case "waiting":      return "pending";
    case "cancelled":    return "failed";
    case "compensating": return "running";
    default:             return s as RunStatus;
  }
}

function formatDuration(startedAt: string, completedAt?: string): string {
  const ms =
    (completedAt ? new Date(completedAt) : new Date()).getTime() -
    new Date(startedAt).getTime();
  const s = Math.floor(ms / 1000);
  if (s < 60) return `${s}s`;
  const m = Math.floor(s / 60);
  if (m < 60) return `${m}m ${s % 60}s`;
  return `${Math.floor(m / 60)}h ${m % 60}m`;
}

function toRun(e: Execution): Run {
  return {
    id: e.id,
    status: mapStatus(e.status),
    agent: e.workflow_id,
    agent_tag: e.workflow_id,
    purpose: (e.inputs?.intent as string) ?? e.workflow_id,
    repo: e.workspace_id,
    duration: formatDuration(e.started_at, e.completed_at),
    policy: e.namespace,
  };
}

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
        const data = await proxyJson<{ executions: Execution[] }>(
          `${endpoint("WORKFLOW_RUNTIME_URL")}/v1/executions`,
          { token, correlation },
        );
        const runs = (data.executions ?? []).map(toRun);
        return NextResponse.json({ runs });
      } catch (err) {
        return NextResponse.json({ runs: [], error: (err as Error).message }, { status: 502 });
      }
    },
  );
}
