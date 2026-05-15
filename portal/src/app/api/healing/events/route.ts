import { NextRequest, NextResponse } from "next/server";
import { authToken, correlationId, endpoint } from "@/lib/api";

export async function GET(req: NextRequest) {
  const appId = req.nextUrl.searchParams.get("app_id") ?? "";
  const limit = req.nextUrl.searchParams.get("limit") ?? "10";
  const { token } = await authToken();
  const correlation = correlationId();

  try {
    const params = new URLSearchParams({ limit });
    if (appId) params.set("app_id", appId);
    const r = await fetch(
      `${endpoint("HEALING_URL")}/v1/detections?${params.toString()}`,
      {
        headers: {
          "x-correlation-id": correlation,
          ...(token ? { authorization: `Bearer ${token}` } : {}),
        },
        cache: "no-store",
      },
    );
    if (!r.ok) {
      const text = await r.text();
      return NextResponse.json({ events: [], error: text }, { status: r.status });
    }
    const body = await r.json();
    // Normalize the healing engine detections into the HealingEvent shape.
    const events = (body.detections ?? []).map((d: Record<string, unknown>) => ({
      id: d.id,
      type: "l1",
      app_id: d.app_id ?? "",
      summary: d.summary ?? String(d.signal_source ?? ""),
      severity: d.blast_radius ?? "unknown",
      created_at: d.detected_at ?? "",
    }));
    // Append L2 suggestions.
    const sr = await fetch(
      `${endpoint("HEALING_URL")}/v1/suggestions?${params.toString()}`,
      {
        headers: {
          "x-correlation-id": correlation,
          ...(token ? { authorization: `Bearer ${token}` } : {}),
        },
        cache: "no-store",
      },
    );
    if (sr.ok) {
      const sb = await sr.json();
      for (const s of sb.suggestions ?? []) {
        events.push({
          id: s.id,
          type: "l2",
          app_id: s.app_id ?? "",
          summary: s.summary ?? "",
          severity: s.blast_radius ?? "unknown",
          created_at: s.proposed_at ?? "",
        });
      }
    }
    events.sort((a: { created_at: string }, b: { created_at: string }) =>
      b.created_at.localeCompare(a.created_at),
    );
    return NextResponse.json({ events: events.slice(0, Number(limit)) });
  } catch (err) {
    return NextResponse.json({ events: [], error: (err as Error).message }, { status: 502 });
  }
}
