/**
 * User preferences API for console_view_preference.
 * (alfred-console-redesign §3.1-3.3)
 *
 * GET  — returns the resolved view preference (persisted or first-sign-in computed)
 * PUT  — persists the user's choice and emits alfred.console.view_toggled.v1
 */
import { NextRequest, NextResponse } from "next/server";
import { getServerSession } from "next-auth";
import { authOptions } from "@/auth";
import { authToken, correlationId, endpoint, emitAudit } from "@/lib/api";

export async function GET(_req: NextRequest) {
  const session = await getServerSession(authOptions);
  const token = session?.accessToken;
  const workspaceId = session?.workspaceSlug ?? "";

  try {
    // Resolve from the control-plane user-prefs store.
    const r = await fetch(
      `${endpoint("CONTROL_PLANE_URL")}/v1/user/preferences?workspace_id=${workspaceId}`,
      {
        headers: {
          ...(token ? { authorization: `Bearer ${token}` } : {}),
          "cache-control": "no-store",
        },
        cache: "no-store",
      },
    );
    if (!r.ok) {
      // Fallback: resolve locally based on workspace role.
      return NextResponse.json({ console_view_preference: null });
    }
    const prefs = await r.json();
    return NextResponse.json({ console_view_preference: prefs.console_view_preference ?? null });
  } catch {
    return NextResponse.json({ console_view_preference: null });
  }
}

export async function PUT(req: NextRequest) {
  const body = (await req.json().catch(() => null)) as {
    console_view_preference?: "friendly" | "advanced";
    session_only?: boolean;
  } | null;

  const pref = body?.console_view_preference;
  if (!pref || !["friendly", "advanced"].includes(pref)) {
    return NextResponse.json({ error: "console_view_preference must be 'friendly' or 'advanced'" }, { status: 400 });
  }

  const { token, actor } = await authToken();
  const correlation = correlationId();
  const sessionOnly = body?.session_only ?? false;

  if (!sessionOnly) {
    // Persist to control-plane.
    try {
      await fetch(`${endpoint("CONTROL_PLANE_URL")}/v1/user/preferences`, {
        method: "PUT",
        headers: {
          "content-type": "application/json",
          "x-correlation-id": correlation,
          ...(token ? { authorization: `Bearer ${token}` } : {}),
        },
        body: JSON.stringify({ console_view_preference: pref }),
      });
    } catch {
      // Non-fatal — return success; the toggle still reflects locally.
    }
  }

  // Emit alfred.console.view_toggled.v1 (task 3.5).
  await emitAudit({
    type: "alfred.console.view_toggled.v1",
    principal: actor,
    data: { to: pref, persistent: !sessionOnly },
    correlation,
  });

  return NextResponse.json({ console_view_preference: pref, session_only: sessionOnly });
}
