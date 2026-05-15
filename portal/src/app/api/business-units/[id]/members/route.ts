import { NextRequest, NextResponse } from "next/server";
import { authToken, correlationId, endpoint } from "@/lib/api";

export type BUMember = {
  subject: string;
  workspaces: string[];
};

export async function GET(_req: NextRequest, ctx: { params: { id: string } }) {
  const { token } = await authToken();
  const correlation = correlationId();
  const buID = ctx.params.id;
  if (!buID) {
    return NextResponse.json({ members: [], error: "missing business unit id" }, { status: 400 });
  }
  try {
    const resp = await fetch(
      `${endpoint("CONTROL_PLANE_URL")}/v1/business-units/${encodeURIComponent(buID)}/members`,
      {
        headers: {
          ...(token ? { authorization: `Bearer ${token}` } : {}),
          "x-correlation-id": correlation,
        },
        cache: "no-store",
      },
    );
    if (!resp.ok) {
      return NextResponse.json(
        { members: [], error: `control-plane ${resp.status}` },
        { status: 502 },
      );
    }
    const body = await resp.json();
    const members: BUMember[] = Array.isArray(body) ? body : body?.members ?? [];
    return NextResponse.json({ members });
  } catch (err) {
    return NextResponse.json({ members: [], error: (err as Error).message }, { status: 502 });
  }
}
