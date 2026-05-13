import { fetchTemplates, requirePortalIdentity } from "@/lib/onboarding";
import type { RepoTemplate } from "@/lib/onboarding-types";
import { NewAppWizard } from "./wizard";
import { PageHead } from "@/components/page/PageHead";
import { Card } from "@/components/primitives";

export default async function NewAppPage() {
  const identity = await requirePortalIdentity();
  let templates: RepoTemplate[] = [];
  let error: string | null = null;
  try {
    templates = await fetchTemplates(identity.token);
  } catch (e) {
    error = e instanceof Error ? e.message : "failed to load templates";
  }

  return (
    <>
      <PageHead
        eyebrow="Platform · Golden Path"
        title="New"
        titleEm="app"
        sub="Select an approved template, supply Workspace parameters, inspect the generated repository contract, then submit onboarding."
      />
      {error && (
        <Card style={{ marginBottom: 16 }}>
          <div style={{ padding: 14, color: "var(--rust)" }}>{error}</div>
        </Card>
      )}
      <NewAppWizard templates={templates} />
    </>
  );
}
