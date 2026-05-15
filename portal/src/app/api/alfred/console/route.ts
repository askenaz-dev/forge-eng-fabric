/**
 * Portal proxy for the Alfred Advanced console.
 * Handles /forge and /openspec slash commands, plus free-text intents.
 * (alfred-console-redesign §2.3 — wire view=advanced into every Alfred call)
 */
import { NextRequest, NextResponse } from "next/server";
import { authToken, correlationId, endpoint, emitAudit } from "@/lib/api";
import { randomUUID } from "crypto";

const openspecUrl = () => process.env.OPENSPEC_URL ?? "http://localhost:8083";

export async function POST(req: NextRequest) {
  const body = (await req.json().catch(() => null)) as {
    workspace_id?: string;
    app_id?: string;
    message?: string;
    view?: string;
  } | null;

  const workspaceId = body?.workspace_id || req.cookies.get("forge_workspace")?.value || "";
  const text = (body?.message ?? "").trim();
  if (!workspaceId || !text) {
    return NextResponse.json({ error: "workspace_id and message are required" }, { status: 400 });
  }

  const { token, actor } = await authToken();
  const correlation = correlationId();
  const view = body?.view ?? "advanced";
  const appId = body?.app_id;

  // /forge create (and backward-compat /openspec create)
  if (text.startsWith("/forge create") || text.startsWith("/openspec create")) {
    if (text.startsWith("/openspec")) {
      await emitDeprecatedAlias(token, actor, correlation, text, view);
    }
    const fields = parseFields(text);
    try {
      const r = await fetch(`${openspecUrl()}/v1/openspecs`, {
        method: "POST",
        headers: {
          "content-type": "application/json",
          "x-correlation-id": correlation,
          ...(token ? { authorization: `Bearer ${token}` } : {}),
        },
        body: JSON.stringify({
          workspace_id: workspaceId,
          app_id: appId,
          title: fields.title ?? "Alfred specification",
          business_intent: fields.intent ?? fields.business_intent ?? "Captured from Alfred Console",
          problem_statement: fields.problem ?? "Created from slash command",
          requirements: { functional: [fields.requirement ?? "Clarify requirements"] },
          created_by: "alfred-console",
          view,
        }),
      });
      if (!r.ok) throw new Error(await r.text());
      const created = (await r.json()) as { openspec_id: string };
      return NextResponse.json({ result: "created-openspec", session_id: created.openspec_id });
    } catch (err) {
      return NextResponse.json({ error: (err as Error).message }, { status: 502 });
    }
  }

  // /forge edit (and backward-compat /openspec edit)
  if (text.startsWith("/forge edit") || text.startsWith("/openspec edit")) {
    if (text.startsWith("/openspec")) {
      await emitDeprecatedAlias(token, actor, correlation, text, view);
    }
    const fields = parseFields(text);
    const openspecId = fields.id;
    if (!openspecId) {
      return NextResponse.json({ error: "/forge edit requires id=<openspec_id>" }, { status: 400 });
    }
    try {
      const r = await fetch(`${openspecUrl()}/v1/openspecs/${encodeURIComponent(openspecId)}`, {
        method: "PATCH",
        headers: {
          "content-type": "application/json",
          "x-correlation-id": correlation,
          ...(token ? { authorization: `Bearer ${token}` } : {}),
        },
        body: JSON.stringify({
          title: fields.title,
          business_intent: fields.intent,
          problem_statement: fields.problem,
          updated_by: "alfred-console",
        }),
      });
      if (!r.ok) throw new Error(await r.text());
      return NextResponse.json({ result: "updated-openspec", session_id: openspecId });
    } catch (err) {
      return NextResponse.json({ error: (err as Error).message }, { status: 502 });
    }
  }

  // Free-text / legacy intent submission
  try {
    const r = await fetch(`${endpoint("ALFRED_URL")}/v1/intents`, {
      method: "POST",
      headers: {
        "content-type": "application/json",
        "x-correlation-id": correlation,
        ...(token ? { authorization: `Bearer ${token}` } : {}),
      },
      body: JSON.stringify({
        workspace_id: workspaceId,
        app_id: appId,
        text,
        correlation_id: correlation,
        view,
      }),
    });
    if (!r.ok) throw new Error(await r.text());
    const result = (await r.json()) as { session_id: string };
    return NextResponse.json({ result: "intent-submitted", session_id: result.session_id });
  } catch (err) {
    return NextResponse.json({ error: (err as Error).message }, { status: 502 });
  }
}

async function emitDeprecatedAlias(
  token: string | undefined,
  actor: string,
  correlation: string,
  originalInput: string,
  view: string,
) {
  const mapped = originalInput.replace(/^\/openspec/, "/forge");
  await emitAudit({
    type: "alfred.command.deprecated_alias.v1",
    principal: actor,
    data: { original_input: originalInput, mapped_to: mapped, view },
    correlation,
  }).catch(() => undefined);
}

function parseFields(input: string) {
  const fields: Record<string, string> = {};
  const pattern = /(\w+)=("[^"]+"|'[^']+'|\S+)/g;
  for (const match of input.matchAll(pattern)) {
    fields[match[1]] = match[2].replace(/^['"]|['"]$/g, "");
  }
  return fields;
}
