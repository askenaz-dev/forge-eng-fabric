import { NextRequest } from "next/server";
import { authToken, correlationId, endpoint } from "@/lib/api";

// Server-streamed SSE proxy. Forwards the gateway bearer token, threads the
// correlation id, and preserves the upstream `id:` line so `Last-Event-ID`
// reconnection works through the proxy.
export async function GET(
  req: NextRequest,
  { params }: { params: { id: string } },
): Promise<Response> {
  const { token } = await authToken();
  const correlation = correlationId();
  const lastEventId = req.headers.get("last-event-id") ?? undefined;
  const upstreamUrl = `${endpoint("ALFRED_URL")}/v1/agent-mode/sessions/${params.id}/stream`;
  const headers: Record<string, string> = {
    accept: "text/event-stream",
    "x-correlation-id": correlation,
  };
  if (token) headers.authorization = `Bearer ${token}`;
  if (lastEventId) headers["last-event-id"] = lastEventId;

  const upstream = await fetch(upstreamUrl, {
    method: "GET",
    headers,
    cache: "no-store",
    signal: req.signal,
  });

  if (!upstream.ok || !upstream.body) {
    return new Response(
      JSON.stringify({ error: `upstream ${upstream.status}` }),
      { status: 502, headers: { "content-type": "application/json" } },
    );
  }

  return new Response(upstream.body, {
    headers: {
      "content-type": "text/event-stream",
      "cache-control": "no-cache, no-transform",
      "x-accel-buffering": "no",
      "x-correlation-id": correlation,
    },
  });
}
