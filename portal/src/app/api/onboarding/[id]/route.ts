import { fetchOnboardingRequest, resolvePortalIdentity } from "@/lib/onboarding";
import { NextResponse } from "next/server";

export async function GET(_request: Request, { params }: { params: { id: string } }) {
  const identity = await resolvePortalIdentity();
  if (!identity) return NextResponse.json({ error: "unauthorized" }, { status: 401 });
  try {
    const onboarding = await fetchOnboardingRequest(params.id, identity.token);
    if (!onboarding) return NextResponse.json({ error: "not found" }, { status: 404 });
    return NextResponse.json(onboarding);
  } catch (error) {
    return NextResponse.json({ error: error instanceof Error ? error.message : "failed to load onboarding" }, { status: 502 });
  }
}
