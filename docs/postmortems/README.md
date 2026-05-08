# Postmortems

Forge auto-generates a postmortem on every `incident.resolved.v1`. The flow is
implemented by `services/postmortem` (Python) and consumes:

- the incident timeline (events filtered by `incident_id` from the bus);
- the diagnosis report (citations, hypotheses, suggested actions);
- the healing decisions taken;
- the relevant runbook (linked from the asset's OpenSpec).

## Output

```
PostmortemDraft:
  - title
  - body_markdown   (with required sections — see below)
  - action_items[]  (each with owner; eval rejects empty owners)
  - sections[]      (introspection for the eval suite)
  - citations[]     (the diagnosis citations, for traceability)
```

## Required sections

The eval suite enforces the following section headings. Drafts that omit any
of them are rejected and not published.

- `## Summary`
- `## Impact`
- `## Timeline`
- `## Root cause`
- `## What went well`
- `## What went wrong`
- `## Remediation`
- `## Lessons`
- `## Action items`

## Eval criteria

The eval suite (`services/postmortem/postmortem/generator.py:evaluate`) runs:

1. all required sections are present;
2. every diagnosis `source_id` appears at least once in the body — drafts that
   fail to weave evidence into the prose are rejected;
3. every action item has an owner. Anonymous (`@unknown`) is rejected.

A failing eval produces `{ "passed": false, "failures": [...] }` and the
draft is **not** published. The Inbox shows a follow-up task to a human
author.

## Publishing

Successful drafts are published to:

- **Confluence MCP** — markdown body in the tenant's space (URL recorded on
  the draft).
- **Jira MCP** — one issue per action item, with a link back to the
  Confluence page (key recorded on the action item).
- **OpenSpec backbone** — the postmortem URL is linked from the affected
  asset's OpenSpec, completing the SDLC loop.

Events emitted:
- `postmortem.generated.v1` — draft generated (regardless of eval outcome).
- `postmortem.published.v1` — draft passed eval and was fan-out published.

## Versioning

The prompt is versioned: `generate-postmortem@1.0.0`. Bumps require a new
semver tag on the prompt template; older postmortems remain pinned to the
version that produced them.
