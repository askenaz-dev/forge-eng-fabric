import { NextResponse } from "next/server";
import { authToken, correlationId, endpoint } from "@/lib/api";

type Counts = {
  agents: number;
  skills: number;
  mcp: number;
  approvals: number;
};

async function safeCount(url: string, token?: string, correlation?: string): Promise<number> {
  try {
    const headers: Record<string, string> = { accept: "application/json" };
    if (token) headers.authorization = `Bearer ${token}`;
    if (correlation) headers["x-correlation-id"] = correlation;
    const r = await fetch(url, { headers, cache: "no-store" });
    if (!r.ok) return 0;
    const payload = (await r.json()) as { total?: number; count?: number; items?: unknown[]; approvals?: unknown[] };
    if (typeof payload.total === "number") return payload.total;
    if (typeof payload.count === "number") return payload.count;
    if (Array.isArray(payload.items)) return payload.items.length;
    if (Array.isArray(payload.approvals)) return payload.approvals.length;
    return 0;
  } catch {
    return 0;
  }
}

export async function GET() {
  const { token, actor } = await authToken();
  const correlation = correlationId();
  const [agents, skills, mcp, approvals] = await Promise.all([
    safeCount(`${endpoint("REGISTRY_URL")}/v1/assets?kind=agent&status=approved&summary=true`, token, correlation),
    safeCount(`${endpoint("REGISTRY_URL")}/v1/assets?kind=skill&status=approved&summary=true`, token, correlation),
    safeCount(`${endpoint("REGISTRY_URL")}/v1/assets?kind=mcp&status=approved&summary=true`, token, correlation),
    safeCount(
      `${endpoint("APPROVALS_URL")}/v1/approvals?status=pending&approver=${encodeURIComponent(actor)}`,
      token,
      correlation,
    ),
  ]);
  const counts: Counts = { agents, skills, mcp, approvals };
  return NextResponse.json(counts);
}
