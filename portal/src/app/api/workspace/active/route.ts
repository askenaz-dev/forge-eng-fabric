import { NextResponse, type NextRequest } from "next/server";
import { getServerSession } from "next-auth";
import { authOptions } from "@/auth";
import { authToken, correlationId, emitAudit } from "@/lib/api";

const applicationUrl = () => process.env.APPLICATION_URL ?? "http://localhost:8095";

/**
 * GET /api/workspace/active
 *
 * Returns the workspace currently scoped to the user's next-auth session
 * along with the list of Apps in that workspace. The active workspace lives
 * in the signed JWT (see `auth.ts`), not in a separate cookie — switches go
 * through `session.update()` on the client + the POST below (audit only).
 */
export async function GET() {
  const session = await getServerSession(authOptions);
  const workspaceId = session?.workspaceSlug ?? "";
  if (!workspaceId) {
    return NextResponse.json({ workspace_id: null, apps: [] });
  }
  const token = session?.accessToken;
  let apps: { id: string; name: string; slug: string }[] = [];
  try {
    const r = await fetch(`${applicationUrl()}/v1/workspaces/${workspaceId}/apps`, {
      headers: {
        ...(token ? { authorization: `Bearer ${token}` } : {}),
        "cache-control": "no-store",
      },
      cache: "no-store",
    });
    if (r.ok) {
      const body = await r.json();
      apps = Array.isArray(body) ? body : (body.apps ?? []);
    }
  } catch {
    // Non-fatal — the workspace id is still returned for the caller.
  }
  return NextResponse.json({ workspace_id: workspaceId, apps });
}

/**
 * POST /api/workspace/active
 *
 * Side-effect for a workspace switch: emit an audit event. The JWT is updated
 * client-side via `session.update()` (next-auth's signed-cookie mechanism) —
 * this endpoint does not write cookies and does not call the control-plane.
 */
export async function POST(req: NextRequest) {
  const body = await req.json().catch(() => ({}));
  const tenant = String(body.tenant ?? "").trim();
  const workspace = String(body.workspace ?? "").trim();
  if (!tenant || !workspace) {
    return NextResponse.json({ error: "tenant and workspace required" }, { status: 400 });
  }
  const { actor } = await authToken();
  const correlation = correlationId();

  // The signed JWT (next-auth) is the source of truth for the active workspace;
  // session.update() on the client re-signs it. We only emit an audit event
  // here for traceability — there's no server-side persistence to sync.
  await emitAudit({
    type: "portal.workspace.switched",
    principal: actor,
    data: { tenant, workspace },
    correlation,
  });

  return NextResponse.json({ ok: true });
}
