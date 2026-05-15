import { NextRequest, NextResponse } from "next/server";

const PLATFORM_OPS_URL = process.env.PLATFORM_OPS_URL ?? "http://localhost:8130";

export async function POST(
  req: NextRequest,
  { params }: { params: { id: string } },
) {
  const actor = req.headers.get("X-Forge-Actor") ?? "portal-user";
  const body = await req.text();
  try {
    const res = await fetch(
      `${PLATFORM_OPS_URL}/v1/noise-rules/${params.id}/approve`,
      {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "X-Forge-Actor": actor,
        },
        body: body || "{}",
      },
    );
    const data = await res.json();
    return NextResponse.json(data, { status: res.status });
  } catch (e) {
    return NextResponse.json({ error: String(e) }, { status: 500 });
  }
}
