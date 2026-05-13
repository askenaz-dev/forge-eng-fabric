import { authToken, correlationId, endpoint } from "@/lib/api";

export const dynamic = "force-dynamic";
export const runtime = "nodejs";

// Server-sent events proxy. The Portal connects with `new EventSource(...)`
// and the upstream audit-stream service publishes events as JSON lines on
// `GET /v1/stream?scope=user&principal=...`.

export async function GET() {
  const { token, actor } = await authToken();
  const correlation = correlationId();

  const upstream = `${endpoint("NOTIFICATIONS_URL")}/v1/stream?scope=user&principal=${encodeURIComponent(actor)}`;

  const headers: Record<string, string> = {
    accept: "text/event-stream",
    "x-correlation-id": correlation,
  };
  if (token) headers.authorization = `Bearer ${token}`;

  // Best-effort proxy: if the upstream is down, we emit a single comment so
  // the client immediately closes and retries — better than hanging forever.
  let upstreamResponse: Response | null = null;
  try {
    upstreamResponse = await fetch(upstream, { headers, cache: "no-store" });
  } catch {
    upstreamResponse = null;
  }
  if (!upstreamResponse || !upstreamResponse.ok || !upstreamResponse.body) {
    const stream = new ReadableStream({
      start(controller) {
        controller.enqueue(new TextEncoder().encode(`: notifications upstream unavailable\n\n`));
        controller.close();
      },
    });
    return new Response(stream, {
      headers: {
        "content-type": "text/event-stream",
        "cache-control": "no-cache, no-transform",
        "x-accel-buffering": "no",
      },
    });
  }

  return new Response(upstreamResponse.body, {
    status: 200,
    headers: {
      "content-type": "text/event-stream",
      "cache-control": "no-cache, no-transform",
      "x-accel-buffering": "no",
      "x-correlation-id": correlation,
    },
  });
}
