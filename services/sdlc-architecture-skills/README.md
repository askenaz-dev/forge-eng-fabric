# sdlc-architecture-skills

FastAPI skill service for the SDLC Architecture phase. Port: **8108**.

## Skills

| Skill | Route | Description |
|-------|-------|-------------|
| `propose-adr` | `POST /v1/skills/propose-adr` | Generate an Architecture Decision Record in MADR format |
| `evaluate-options` | `POST /v1/skills/evaluate-options` | Rank ≥2 implementation options with cited rationale |
| `check-openspec-alignment` | `POST /v1/skills/check-openspec-alignment` | Cross-check ADRs against OpenSpec requirements |
| `generate-api-contract` | `POST /v1/skills/generate-api-contract` | Produce an OpenAPI 3.1 document and run Spectral lint |
| `propose-data-model` | `POST /v1/skills/propose-data-model` | ER schema with PII/sensitivity tags |
| `lightweight-threat-model` | `POST /v1/skills/lightweight-threat-model` | STRIDE-style threat model report |

## Input / Output schemas

All skills share the envelope:

```json
{ "app_id": "uuid", "openspec_id": "uuid", "correlation_id": "uuid", "payload": { ... } }
```

Outputs follow the pattern:
```json
{ "skill": "propose-adr", "output": { ... }, "events": ["sdlc.adr.proposed.v1"] }
```

## Gates wired

`adrs_published`, `api_contract_published`, `data_model_documented`, `threat_model_present`, `security_review_passed`, `openspec_updated`

## Eval baseline

T1 promotion requires ≥30 graded fixtures per skill with ≥0.85 pass rate (see `eval/` directory). Adversarial fixtures cover: citation hallucination, cross-tenant data leakage, oversized ADR (>200 lines), non-MADR format.

## Running locally

```bash
cd services/sdlc-architecture-skills
uv run --extra dev uvicorn sdlc_architecture_skills.app:app --reload --port 8108
uv run --extra dev pytest -q
```
