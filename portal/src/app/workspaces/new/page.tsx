import { getServerSession } from "next-auth";
import { redirect } from "next/navigation";
import { authOptions } from "@/auth";
import { PageHead } from "@/components/page/PageHead";
import { Card } from "@/components/primitives";
import { NewWorkspaceForm } from "./NewWorkspaceForm";

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
        <NewWorkspaceForm />
      </Card>
    </div>
  );
}
