"use server";

import { getServerSession } from "next-auth";
import { redirect } from "next/navigation";
import { authOptions } from "@/auth";

export async function createWorkspace(formData: FormData) {
  const session = await getServerSession(authOptions);
  if (!session) redirect("/api/auth/signin");
  const token = (session as { accessToken?: string }).accessToken;
  // The session cookie can outlive the access token (refresh-token failure
  // clears it via the next-auth jwt callback). Redirect to signin instead of
  // throwing so the user can re-auth cleanly.
  if (!token) redirect("/api/auth/signin");

  const businessUnitId = String(formData.get("business_unit_id") ?? "").trim();
  const name = String(formData.get("name") ?? "").trim();
  const description = String(formData.get("description") ?? "").trim();
  const owners = String(formData.get("owners") ?? "")
    .split(",")
    .map((owner) => owner.trim())
    .filter(Boolean);

  const cp = process.env.CONTROL_PLANE_URL ?? "http://localhost:8081";
  const response = await fetch(`${cp}/v1/business-units/${businessUnitId}/workspaces`, {
    method: "POST",
    headers: { authorization: `Bearer ${token}`, "content-type": "application/json" },
    body: JSON.stringify({ name, description, owners }),
  });
  if (!response.ok) {
    throw new Error(`control-plane ${response.status}: ${await response.text()}`);
  }
  redirect("/");
}
