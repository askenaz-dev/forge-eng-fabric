# sdlc-design-skills

FastAPI skill service for the SDLC Design phase. Port: **8109**.

## Skills

| Skill | Route | Description |
|-------|-------|-------------|
| `generate-ui-blueprint` | `POST /v1/skills/generate-ui-blueprint` | Figma-export JSON blueprint from API contract |
| `generate-component-stubs` | `POST /v1/skills/generate-component-stubs` | React + Vue stubs with Design System token bindings |
| `accessibility-audit` | `POST /v1/skills/accessibility-audit` | Axe-core audit with WCAG 2.1 AA severity classification |

## Input / Output schemas

```json
{ "app_id": "uuid", "openspec_id": "uuid", "correlation_id": "uuid", "payload": { ... } }
```

Outputs:
- `generate-ui-blueprint` → `{ "blueprint_path": "...", "event": "sdlc.ui_blueprint.proposed.v1" }`
- `generate-component-stubs` → `{ "stub_files": [...], "event": "sdlc.component_stubs.committed.v1" }`
- `accessibility-audit` → `{ "audit_report": {...}, "audit_passed": true|false, "event": "sdlc.accessibility_audit.completed.v1" }`

## Gates wired

`ui_blueprint_present`, `component_stubs_committed`, `accessibility_audit_passed`

## Eval baseline

T1 promotion requires ≥30 graded fixtures per skill. Adversarial fixtures cover: hardcoded token values in stubs, non-responsive blueprint layouts, false accessibility passes.

## Running locally

```bash
cd services/sdlc-design-skills
uv run --extra dev uvicorn sdlc_design_skills.app:app --reload --port 8109
uv run --extra dev pytest -q
```
