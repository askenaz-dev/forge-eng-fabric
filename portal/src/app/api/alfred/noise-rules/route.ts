import { NextResponse } from "next/server";

const PLATFORM_OPS_URL = process.env.PLATFORM_OPS_URL ?? "http://localhost:8130";

export const dynamic = "force-dynamic";

export async function GET(req: Request) {
  const { searchParams } = new URL(req.url);
  const status = searchParams.get("status") ?? "draft";
  try {
    const res = await fetch(
      `${PLATFORM_OPS_URL}/v1/noise-rules?status=${encodeURIComponent(status)}`,
      { next: { revalidate: 0 } },
    );
    if (!res.ok) {
      return NextResponse.json(
        { rules: [], error: `platform-ops returned ${res.status}` },
        { status: 200 },
      );
    }
    const data = await res.json();
    return NextResponse.json(data);
  } catch (e) {
    return NextResponse.json({ rules: [], error: String(e) }, { status: 200 });
  }
}
