import { NextRequest, NextResponse } from "next/server";
import { authToken, correlationId, endpoint } from "@/lib/api";

export async function POST(req: NextRequest) {
  const body = await req.json().catch(() => ({} as Record<string, unknown>));
  const workspaceId = typeof body.workspace_id === "string" ? body.workspace_id.trim() : "";
  if (!workspaceId) {
    return NextResponse.json({ error: "workspace_id is required" }, { status: 400 });
  }
  const { workspace_id: _omit, ...rest } = body as { workspace_id?: string } & Record<string, unknown>;
  const { token } = await authToken();
  const correlation = correlationId();
  const upstream = await fetch(
    `${endpoint("REGISTRY_URL")}/v1/workspaces/${encodeURIComponent(workspaceId)}/assets`,
    {
      method: "POST",
      headers: {
        "content-type": "application/json",
        ...(token ? { authorization: `Bearer ${token}` } : {}),
        "x-correlation-id": correlation,
      },
      body: JSON.stringify(rest),
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
