// Workspace-admin form to register a custom node (v0 of the custom-node SDK).
// Until the ingestion pipeline lands, admins register manually via this form.
// See docs/sdk/custom-nodes.md.

import { authOptions } from "@/auth";
import { getServerSession } from "next-auth";
import { redirect } from "next/navigation";
import { CustomNodeForm } from "./CustomNodeForm";

export default async function CustomNodesAdminPage() {
  const session = await getServerSession(authOptions);
  if (!session) redirect("/api/auth/signin");
  return (
    <section className="space-y-3">
      <header>
        <p className="text-sm uppercase tracking-wide opacity-60">Admin · Custom Nodes</p>
        <h2 className="text-2xl font-semibold">Register a custom node</h2>
      </header>
      <p className="text-sm opacity-80 max-w-2xl">
        Custom nodes extend the AI-Flow palette with publisher-hosted integrations. v0 of the SDK uses manual
        registration per workspace; the ingestion pipeline (publishing, signing, distribution registry) ships in a
        separate change. See{" "}
        <a className="underline" href="/docs/sdk/custom-nodes.md">
          docs/sdk/custom-nodes.md
        </a>
        .
      </p>
      <CustomNodeForm />
    </section>
  );
}
