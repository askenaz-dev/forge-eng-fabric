import { NextResponse, type NextRequest } from "next/server";
import { authToken, correlationId, emitAudit, endpoint } from "@/lib/api";
import { cookies } from "next/headers";

const ALLOWED = new Set(["compact", "comfortable", "spacious"]);

export async function POST(req: NextRequest) {
  const body = await req.json().catch(() => ({}));
  const next = String(body.density ?? "");
  if (!ALLOWED.has(next)) {
    return NextResponse.json({ error: "invalid density" }, { status: 400 });
  }
  const { token, actor } = await authToken();
  const correlation = correlationId();

  if (token) {
    fetch(`${endpoint("CONTROL_PLANE_URL")}/v1/users/me/preferences`, {
      method: "PATCH",
      headers: {
        "content-type": "application/json",
        authorization: `Bearer ${token}`,
        "x-correlation-id": correlation,
      },
      body: JSON.stringify({ density: next }),
    }).catch(() => undefined);
  }

  const jar = cookies();
  const prev = jar.get("forge_prefs")?.value;
  let merged: Record<string, unknown> = { density: next };
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
    type: "portal.density.changed",
    principal: actor,
    data: { to: next },
    correlation,
  });

  return res;
}
