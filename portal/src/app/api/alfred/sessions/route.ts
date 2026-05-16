import { randomUUID } from "crypto";
import { NextRequest, NextResponse } from "next/server";
import { getServerSession } from "next-auth";
import { authOptions } from "@/auth";
import { correlationId, endpoint, proxyJson } from "@/lib/api";

export async function POST(req: NextRequest) {
  const body = (await req.json().catch(() => null)) as
    | { workspace_id?: string; openspec_id?: string; intent?: string; start_step?: string }
    | null;
  if (!body?.intent) {
    return NextResponse.json({ error: "intent is required" }, { status: 400 });
  }
  const session = await getServerSession(authOptions);
  const token = session?.accessToken;
  const correlation = correlationId();
  const workspaceId = body.workspace_id || session?.workspaceSlug || "";
  if (!workspaceId) {
    return NextResponse.json({ error: "no active workspace" }, { status: 409 });
  }
  try {
    const data = await proxyJson<{ session_id: string; status: string }>(
      `${endpoint("ALFRED_URL")}/v1/agent-mode/sessions`,
      {
        method: "POST",
        token,
        correlation,
        body: JSON.stringify({
          workspace_id: workspaceId,
          openspec_id: body.openspec_id ?? randomUUID(),
          intent: body.intent,
          ...(body.start_step ? { start_step: body.start_step } : {}),
        }),
      },
    );
    return NextResponse.json(data);
  } catch (err) {
    return NextResponse.json({ error: (err as Error).message }, { status: 502 });
  }
}
