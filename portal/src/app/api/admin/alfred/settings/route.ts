import { NextRequest, NextResponse } from "next/server";
import { getServerSession } from "next-auth";
import { authOptions } from "@/auth";
import { correlationId, endpoint, proxyJson } from "@/lib/api";

type AlfredAdminState = { workspace_id: string; dock_enabled: boolean };

async function resolveWorkspace(): Promise<{ workspace: string | null; token?: string }> {
  const session = await getServerSession(authOptions);
  return { workspace: session?.workspaceSlug ?? null, token: session?.accessToken };
}

export async function GET(_req: NextRequest) {
  const { workspace, token } = await resolveWorkspace();
  if (!workspace) {
    return NextResponse.json({ error: "no active workspace" }, { status: 409 });
  }
  try {
    const data = await proxyJson<AlfredAdminState>(
      `${endpoint("ALFRED_URL")}/v1/agent-mode/admin/settings?workspace_id=${workspace}`,
      { token, correlation: correlationId() },
    );
    return NextResponse.json(data);
  } catch {
    return NextResponse.json({ workspace_id: workspace, dock_enabled: false });
  }
}

export async function POST(req: NextRequest) {
  const { workspace, token } = await resolveWorkspace();
  if (!workspace) {
    return NextResponse.json({ error: "no active workspace" }, { status: 409 });
  }
  const body = (await req.json().catch(() => null)) as { dock_enabled?: boolean } | null;
  if (typeof body?.dock_enabled !== "boolean") {
    return NextResponse.json({ error: "dock_enabled is required" }, { status: 400 });
  }
  try {
    const data = await proxyJson<AlfredAdminState>(
      `${endpoint("ALFRED_URL")}/v1/agent-mode/admin/settings`,
      {
        method: "POST",
        token,
        correlation: correlationId(),
        body: JSON.stringify({ workspace_id: workspace, dock_enabled: body.dock_enabled }),
      },
    );
    return NextResponse.json(data);
  } catch (err) {
    return NextResponse.json({ error: (err as Error).message }, { status: 502 });
  }
}
