import { NextResponse } from "next/server";
import { authToken, correlationId, endpoint, proxyJson } from "@/lib/api";

export type ServiceHealth = {
  id: string;
  kind: string;
  state: "healthy" | "degraded" | "down" | "unknown";
  rps?: number;
  p99?: string;
};

export type ServicesHealthPayload = {
  services: ServiceHealth[];
};

export async function GET() {
  const { token } = await authToken();
  const correlation = correlationId();
  try {
    const data = await proxyJson<ServicesHealthPayload>(
      `${endpoint("OBS_URL")}/v1/services/health`,
      { token, correlation },
    );
    return NextResponse.json(data);
  } catch (err) {
    return NextResponse.json({ services: [], error: (err as Error).message }, { status: 502 });
  }
}
