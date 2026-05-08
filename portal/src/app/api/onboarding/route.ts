import { fetchOnboardingRequests, resolvePortalIdentity, submitOnboarding } from "@/lib/onboarding";
import { NextResponse } from "next/server";

export async function GET(request: Request) {
  const identity = await resolvePortalIdentity();
  if (!identity) return NextResponse.json({ error: "unauthorized" }, { status: 401 });
  const { searchParams } = new URL(request.url);
  try {
    const requests = await fetchOnboardingRequests(
      { workspace_id: searchParams.get("workspace_id") ?? undefined, status: searchParams.get("status") ?? undefined },
      identity.token,
    );
    return NextResponse.json({ requests });
  } catch (error) {
    return NextResponse.json({ error: error instanceof Error ? error.message : "failed to list onboarding requests" }, { status: 502 });
  }
}

export async function POST(request: Request) {
  const identity = await resolvePortalIdentity();
  if (!identity) return NextResponse.json({ error: "unauthorized" }, { status: 401 });
  try {
    const payload = (await request.json()) as Record<string, unknown>;
    const created = await submitOnboarding(payload, identity.token, identity.user);
    return NextResponse.json(created, { status: 202 });
  } catch (error) {
    return NextResponse.json({ error: error instanceof Error ? error.message : "failed to submit onboarding" }, { status: 400 });
  }
}
