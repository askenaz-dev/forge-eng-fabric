import { fetchOnboardingTimeline, resolvePortalIdentity } from "@/lib/onboarding";
import { NextResponse } from "next/server";

export async function GET(_request: Request, { params }: { params: { id: string } }) {
  const identity = await resolvePortalIdentity();
  if (!identity) return NextResponse.json({ error: "unauthorized" }, { status: 401 });
  try {
    const events = await fetchOnboardingTimeline(params.id, identity.token);
    return NextResponse.json({ events });
  } catch (error) {
    return NextResponse.json({ error: error instanceof Error ? error.message : "failed to load timeline" }, { status: 502 });
  }
}
