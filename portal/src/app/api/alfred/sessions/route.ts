import { NextRequest, NextResponse } from "next/server";
import { authToken, correlationId, endpoint, proxyJson } from "@/lib/api";

export async function POST(req: NextRequest) {
  const body = (await req.json().catch(() => null)) as
    | { workspace_id?: string; openspec_id?: string; intent?: string; start_step?: string }
    | null;
  if (!body?.openspec_id || !body?.intent) {
    return NextResponse.json({ error: "openspec_id and intent are required" }, { status: 400 });
  }
  const { token } = await authToken();
  const correlation = correlationId();
  // Workspace id resolved server-side from the active workspace cookie if not
  // provided by the client. Falls back to body.workspace_id when present.
  const workspaceCookie = req.cookies.get("forge_workspace")?.value;
  const workspaceId = body.workspace_id || workspaceCookie || "";
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
          openspec_id: body.openspec_id,
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
