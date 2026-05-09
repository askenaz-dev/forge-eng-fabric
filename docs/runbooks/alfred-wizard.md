# Alfred Wizard Runbook

The Alfred Wizard is the conversational front-end for non-technical users to assemble an OpenSpec without slash commands. It is feature-flagged and additive — the slash-command [Alfred Console](../../portal/src/app/alfred/page.tsx) at `/alfred` remains the default for power users.

## Feature flags

| Flag | Where | Default | Purpose |
|---|---|---|---|
| `?wizard=1` | URL query at `/alfred/wizard` | unset | Surfaces the wizard route in the Portal session |
| `ALFRED_DIALOGUE_API` | env on `services/alfred` | `disabled` | Surfaces `POST /v1/intent/start`, `/answer`, `/commit` and `GET /v1/intent/{id}` |

Both flags are required to use the wizard end-to-end:

```sh
# Server (services/alfred):
export ALFRED_DIALOGUE_API=enabled

# Browser:
open http://localhost:3000/alfred/wizard?wizard=1
```

## Architecture

```
Portal /alfred/wizard  →  services/alfred /v1/intent/*  →  services/openspec /v1/intent/*
                                                 │
                                                 └─→  LiteLLM (question-generation, capped at 12 turns)
```

The dialogue state and progressive draft live in the openspec service (see [`drafts.py`](../../services/openspec/openspec_service/drafts.py)). Alfred adds the LLM-driven question generation and policy gating; openspec owns the canonical draft → committed transition.

## Common operations

### Starting a draft

```sh
curl -X POST http://localhost:8090/v1/intent/start \
  -H 'content-type: application/json' \
  -H 'authorization: Bearer <token>' \
  -d '{"workspace_id": "<uuid>", "business_intent": "Track purchases and issue tier-based discounts"}'
```

Returns the draft, the completeness report, and the next question.

### Answering

```sh
curl -X POST http://localhost:8090/v1/intent/<draft_id>/answer \
  -H 'content-type: application/json' \
  -d '{"answer": "Operations, Customer Support, and Marketing", "field_updates": {"stakeholders": ["Operations", "Customer Support", "Marketing"]}}'
```

### Committing

```sh
curl -X POST http://localhost:8090/v1/intent/<draft_id>/commit \
  -H 'content-type: application/json' \
  -d '{}'
```

Returns the committed `OpenSpecDocument`. After commit, the draft is immutable and the wizard hands off to the SDLC orchestrator.

### Inspecting the completeness map

```sh
curl http://localhost:8083/v1/openspecs/<draft_id>/completeness
```

The wizard polls this to know which question to ask next; section/field statuses are `complete | partial | empty`.

## Troubleshooting

| Symptom | Cause | Fix |
|---|---|---|
| 404 on `/v1/intent/*` | `ALFRED_DIALOGUE_API` unset on the Alfred service | Set the env var and restart the service |
| 400 "draft not commit-ready" | Missing `business_intent` or no functional requirements | Continue answering until completeness is `complete` |
| Wizard stuck on the same question | LLM call exceeded `MAX_DIALOGUE_TURNS=12` | Click "Commit" — the wizard accepts the partial state |
| Drafts disappear after 14 days | Inactivity expiry job ran | Audit row in `intent.draft.abandoned.v1` records the abandonment |

## Inactivity expiry

Drafts inactive for 14+ days flip to `abandoned` and emit `intent.draft.abandoned.v1`. The job runs nightly via the `retention-jobs` Helm chart; see [`scripts/expire_inactive_drafts.py`](../../scripts/expire_inactive_drafts.py).

## Audit trail

Every wizard turn produces an audit event:

| Event | When |
|---|---|
| `intent.dialogue.started.v1` | Draft created |
| `intent.dialogue.turn.v1` | User answers a question |
| `intent.committed.v1` | Draft committed to a canonical OpenSpec |
| `intent.draft.abandoned.v1` | Draft expired or manually abandoned |

All events carry `correlation_id`, `principal`, and `field_updates` so the audit chain matches the OpenSpec history.
