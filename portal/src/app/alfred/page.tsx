import { authOptions } from "@/auth";
import { randomUUID } from "crypto";
import { getServerSession } from "next-auth";
import { redirect } from "next/navigation";
import { PageHead } from "@/components/page/PageHead";
import { Card } from "@/components/primitives";

type SearchParams = { result?: string; session_id?: string; error?: string };

const alfredUrl = () => process.env.ALFRED_URL ?? "http://localhost:8090";
const openspecUrl = () => process.env.OPENSPEC_URL ?? "http://localhost:8083";

async function getToken() {
  const session = await getServerSession(authOptions);
  if (!session) redirect("/api/auth/signin");
  return (session as { accessToken?: string }).accessToken;
}

async function submitConsole(formData: FormData) {
  "use server";
  const token = await getToken();
  const workspaceId = required(formData, "workspace_id");
  const text = required(formData, "message");
  const correlationId = randomUUID();

  try {
    if (text.startsWith("/openspec create")) {
      const fields = parseFields(text);
      const linkedArtifacts = linkedArtifactsFromFields(fields);
      const response = await fetch(`${openspecUrl()}/v1/openspecs`, {
        method: "POST",
        headers: { "content-type": "application/json", ...(token ? { authorization: `Bearer ${token}` } : {}) },
        body: JSON.stringify({
          workspace_id: workspaceId,
          title: fields.title ?? "Alfred specification",
          business_intent: fields.intent ?? fields.business_intent ?? "Captured from Alfred Console",
          problem_statement: fields.problem ?? "Created from slash command",
          requirements: { functional: [fields.requirement ?? "Clarify requirements"] },
          linked_artifacts: linkedArtifacts,
          created_by: "alfred-console",
        }),
      });
      if (!response.ok) throw new Error(await response.text());
      const created = (await response.json()) as { openspec_id: string };
      redirect(`/alfred?result=created-openspec&session_id=${created.openspec_id}`);
    }

    if (text.startsWith("/openspec edit")) {
      const fields = parseFields(text);
      const openspecId = fields.id;
      if (!openspecId) throw new Error("/openspec edit requires id=<openspec_id>");
      const response = await fetch(`${openspecUrl()}/v1/openspecs/${encodeURIComponent(openspecId)}`, {
        method: "PATCH",
        headers: { "content-type": "application/json", ...(token ? { authorization: `Bearer ${token}` } : {}) },
        body: JSON.stringify({
          title: fields.title,
          business_intent: fields.intent,
          problem_statement: fields.problem,
          updated_by: "alfred-console",
        }),
      });
      if (!response.ok) throw new Error(await response.text());
      redirect(`/alfred?result=updated-openspec&session_id=${openspecId}`);
    }

    const response = await fetch(`${alfredUrl()}/v1/intents`, {
      method: "POST",
      headers: {
        "content-type": "application/json",
        "x-correlation-id": correlationId,
        ...(token ? { authorization: `Bearer ${token}` } : {}),
      },
      body: JSON.stringify({ workspace_id: workspaceId, text, correlation_id: correlationId }),
    });
    if (!response.ok) throw new Error(await response.text());
    const result = (await response.json()) as { session_id: string };
    redirect(`/alfred?result=intent-submitted&session_id=${result.session_id}`);
  } catch (error) {
    const message = error instanceof Error ? error.message : "unknown error";
    redirect(`/alfred?error=${encodeURIComponent(message.slice(0, 300))}`);
  }
}

export default async function AlfredPage({ searchParams }: { searchParams: SearchParams }) {
  await getToken();
  return (
    <div style={{ maxWidth: 1024, margin: "0 auto" }}>
      <PageHead
        eyebrow="Platform · Alfred"
        title="Alfred"
        titleEm="console"
        sub="Submit natural-language intents or use slash commands to create and edit structured specifications."
      />

      {searchParams.result && <Card style={{ marginBottom: 16 }}><div style={{ padding: 14, color: "var(--thread)" }}>{searchParams.result}: <code>{searchParams.session_id}</code></div></Card>}
      {searchParams.error && <Card style={{ marginBottom: 16 }}><div style={{ padding: 14, color: "var(--rust)" }}>{searchParams.error}</div></Card>}

      <form action={submitConsole} className="space-y-4 rounded border border-neutral-200 bg-white p-5 dark:border-neutral-800 dark:bg-neutral-900">
        <label className="grid gap-1 text-sm">
          <span className="font-medium">Workspace ID</span>
          <input name="workspace_id" required className="rounded border border-neutral-300 bg-transparent px-3 py-2 dark:border-neutral-700" />
        </label>
        <label className="grid gap-1 text-sm">
          <span className="font-medium">Intent or slash command</span>
          <textarea name="message" required rows={8} placeholder="/openspec create title=Payments intent='Reduce payment failures' requirement='Retry failed payments' jira=PAY-123 confluence=https://confluence.example/payments" className="rounded border border-neutral-300 bg-transparent px-3 py-2 font-mono text-sm dark:border-neutral-700" />
        </label>
        <button className="rounded bg-neutral-900 px-4 py-2 text-sm font-medium text-white dark:bg-neutral-100 dark:text-neutral-900">Run Alfred</button>
      </form>

      <div className="grid gap-3 rounded border border-dashed border-neutral-300 bg-white p-5 text-sm dark:border-neutral-800 dark:bg-neutral-900 md:grid-cols-2">
        <div>
          <h3 className="font-medium">Create specification</h3>
          <code className="mt-2 block rounded bg-neutral-100 p-2 text-xs dark:bg-neutral-800">{`/openspec create title="Payments" intent="Reduce failures" requirement="Retry failed payments" jira=PAY-123 confluence=https://confluence.example/payments`}</code>
        </div>
        <div>
          <h3 className="font-medium">Edit specification</h3>
          <code className="mt-2 block rounded bg-neutral-100 p-2 text-xs dark:bg-neutral-800">{`/openspec edit id=payments title="Payments v2" problem="Retries are inconsistent"`}</code>
        </div>
      </div>
    </div>
  );
}

function parseFields(input: string) {
  const fields: Record<string, string> = {};
  const pattern = /(\w+)=("[^"]+"|'[^']+'|\S+)/g;
  for (const match of input.matchAll(pattern)) {
    fields[match[1]] = match[2].replace(/^['"]|['"]$/g, "");
  }
  return fields;
}

function linkedArtifactsFromFields(fields: Record<string, string>) {
  const artifacts: { kind: string; ref: string; direction: "bidirectional"; metadata: { source: string } }[] = [];
  const jira = fields.jira ?? fields.jira_story;
  const confluence = fields.confluence ?? fields.confluence_page;
  if (jira) artifacts.push({ kind: "jira", ref: jira, direction: "bidirectional", metadata: { source: "alfred-console" } });
  if (confluence) artifacts.push({ kind: "confluence", ref: confluence, direction: "bidirectional", metadata: { source: "alfred-console" } });
  return artifacts;
}

function required(formData: FormData, key: string) {
  const value = String(formData.get(key) ?? "").trim();
  if (!value) throw new Error(`${key} is required`);
  return value;
}
