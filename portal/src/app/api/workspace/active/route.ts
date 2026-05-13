import { NextResponse, type NextRequest } from "next/server";
import { authToken, correlationId, emitAudit, endpoint } from "@/lib/api";
import { cookies } from "next/headers";

export async function POST(req: NextRequest) {
  const body = await req.json().catch(() => ({}));
  const tenant = String(body.tenant ?? "").trim();
  const workspace = String(body.workspace ?? "").trim();
  if (!tenant || !workspace) {
    return NextResponse.json({ error: "tenant and workspace required" }, { status: 400 });
  }
  const { token, actor } = await authToken();
  const correlation = correlationId();

  if (token) {
    fetch(`${endpoint("CONTROL_PLANE_URL")}/v1/users/me/active-workspace`, {
      method: "PUT",
      headers: {
        "content-type": "application/json",
        authorization: `Bearer ${token}`,
        "x-correlation-id": correlation,
      },
      body: JSON.stringify({ tenant_id: tenant, workspace_id: workspace }),
    }).catch(() => undefined);
  }

  const jar = cookies();
  const prev = jar.get("forge_prefs")?.value;
  let merged: Record<string, unknown> = { tenant, workspace };
  if (prev) {
    try {
      merged = { ...JSON.parse(prev), ...merged };
    } catch {
      // ignored
    }
  }
  const res = NextResponse.json({ ok: true });
  res.cookies.set("forge_prefs", JSON.stringify(merged), {
    path: "/",
    httpOnly: false,
    sameSite: "lax",
    maxAge: 60 * 60 * 24 * 365,
  });

  await emitAudit({
    type: "portal.workspace.switched",
    principal: actor,
    data: { tenant, workspace },
    correlation,
  });

  return res;
}
