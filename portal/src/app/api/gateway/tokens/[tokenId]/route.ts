import { NextRequest, NextResponse } from "next/server";
import { authToken, correlationId, endpoint } from "@/lib/api";

export async function DELETE(_req: NextRequest, { params }: { params: { tokenId: string } }) {
  const { token } = await authToken();
  const correlation = correlationId();
  const gateway = process.env.SKILL_GATEWAY_URL ?? endpoint("REGISTRY_URL").replace(":8082", ":8120");
  const upstream = await fetch(`${gateway}/v1/gateway/tokens/${encodeURIComponent(params.tokenId)}`, {
    method: "DELETE",
    headers: {
      ...(token ? { authorization: `Bearer ${token}` } : {}),
      "x-correlation-id": correlation,
    },
  });
  return new NextResponse(null, { status: upstream.status });
}
