import { getServerSession } from "next-auth";
import { redirect } from "next/navigation";
import { authOptions } from "@/auth";
import Link from "next/link";
import { PageHead } from "@/components/page/PageHead";
import { Button, Card } from "@/components/primitives";

async function createWorkspace(formData: FormData) {
  "use server";

  const session = await getServerSession(authOptions);
  if (!session) redirect("/api/auth/signin");
  const token = (session as any).accessToken as string | undefined;
  if (!token) throw new Error("missing access token");

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

export default async function NewWorkspacePage() {
  const session = await getServerSession(authOptions);
  if (!session) redirect("/api/auth/signin");

  return (
    <div style={{ maxWidth: 640 }}>
      <PageHead
        eyebrow="Platform · Workspaces"
        title="New"
        titleEm="workspace"
        sub="Create a workspace in an existing Business Unit."
      />

      <Card>
        <form action={createWorkspace} style={{ padding: 16, display: "flex", flexDirection: "column", gap: 14 }}>
          <label className="grid gap-1 text-sm">
            <span style={{ fontWeight: 500 }}>Business Unit ID</span>
            <input name="business_unit_id" required className="top-search" style={{ height: 36 }} />
          </label>
          <label className="grid gap-1 text-sm">
            <span style={{ fontWeight: 500 }}>Workspace name</span>
            <input name="name" required className="top-search" style={{ height: 36 }} />
          </label>
          <label className="grid gap-1 text-sm">
            <span style={{ fontWeight: 500 }}>Description</span>
            <textarea
              name="description"
              rows={3}
              style={{
                background: "var(--bg-card)",
                border: "1px solid var(--border)",
                borderRadius: "var(--r-2)",
                padding: "8px 10px",
                color: "var(--fg)",
                fontFamily: "var(--f-sans)",
                fontSize: 13,
              }}
            />
          </label>
          <label className="grid gap-1 text-sm">
            <span style={{ fontWeight: 500 }}>Owners</span>
            <input name="owners" required placeholder="askenaz,developer" className="top-search" style={{ height: 36 }} />
            <span style={{ fontSize: 11, color: "var(--fg-3)" }}>Comma-separated local usernames or subject identifiers.</span>
          </label>
          <div style={{ display: "flex", alignItems: "center", gap: 12 }}>
            <Button variant="primary" type="submit">Create workspace</Button>
            <Link href="/" style={{ color: "var(--fg-2)", fontSize: 13 }}>Cancel</Link>
          </div>
        </form>
      </Card>
    </div>
  );
}
