import { NextResponse } from "next/server";
import { authToken, correlationId, endpoint, proxyJson } from "@/lib/api";

type Step = { ic: string; tone: "ok" | "err" | "em" | "warn"; label: string; ms: string };
type RunDetail = {
  id: string;
  agent: string;
  purpose: string;
  repo: string;
  duration: string;
  policy: string;
  status: string;
  triggered_by: string;
  steps: Step[];
  diff?: { before: string[]; after: string[] };
};

export async function GET(_: Request, { params }: { params: { id: string } }) {
  const { token } = await authToken();
  const correlation = correlationId();
  try {
    const detail = await proxyJson<RunDetail>(
      `${endpoint("SDLC_URL")}/v1/runs/${encodeURIComponent(params.id)}`,
      { token, correlation },
    );
    return NextResponse.json(detail);
  } catch (err) {
    return NextResponse.json({ error: (err as Error).message }, { status: 502 });
  }
}
