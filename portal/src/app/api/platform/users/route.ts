import { NextRequest, NextResponse } from "next/server";
import { authToken, correlationId, endpoint } from "@/lib/api";

export type PlatformUser = {
  subject: string;
  username?: string;
  email?: string;
  first_seen?: string;
  last_seen?: string;
};

export async function GET(req: NextRequest) {
  const { token } = await authToken();
  const correlation = correlationId();
  const q = req.nextUrl.searchParams.get("q")?.trim() ?? "";
  try {
    const url = new URL(`${endpoint("CONTROL_PLANE_URL")}/v1/platform/users`);
    if (q) url.searchParams.set("q", q);
    const resp = await fetch(url.toString(), {
      headers: {
        ...(token ? { authorization: `Bearer ${token}` } : {}),
        "x-correlation-id": correlation,
      },
      cache: "no-store",
    });
    if (!resp.ok) {
      return NextResponse.json(
        { users: [], error: `control-plane ${resp.status}` },
        { status: 502 },
      );
    }
    const body = await resp.json();
    const users: PlatformUser[] = Array.isArray(body) ? body : body?.users ?? [];
    return NextResponse.json({ users });
  } catch (err) {
    return NextResponse.json({ users: [], error: (err as Error).message }, { status: 502 });
  }
}
