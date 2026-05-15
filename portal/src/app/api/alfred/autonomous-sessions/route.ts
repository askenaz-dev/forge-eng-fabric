import { NextResponse } from "next/server";

const ALFRED_URL = process.env.ALFRED_URL ?? "http://localhost:8090";

export const dynamic = "force-dynamic";

export async function GET() {
  try {
    const res = await fetch(
      `${ALFRED_URL}/v1/agent-mode/sessions?trigger_source=symptom&limit=50`,
      {
        headers: { "Content-Type": "application/json" },
        next: { revalidate: 0 },
      },
    );
    if (!res.ok) {
      return NextResponse.json(
        { sessions: [], error: `alfred returned ${res.status}` },
        { status: 200 },
      );
    }
    const data = await res.json();
    return NextResponse.json({ sessions: data.sessions ?? data ?? [] });
  } catch (e) {
    return NextResponse.json({ sessions: [], error: String(e) }, { status: 200 });
  }
}
