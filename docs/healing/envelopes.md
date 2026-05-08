# Healing Envelopes

A healing envelope describes the autonomy boundary for a
`(capability, asset_pattern, env, criticality)` tuple. The engine matches an
incident against the closest envelope before deciding what level to apply.

## Schema

```yaml
healing_envelope:
  id: <generated when omitted>
  tenant_id: <required>
  workspace_id: <optional — env scope vs workspace scope>
  capability: <id>            # e.g. sdlc-devops, observability
  asset_pattern: <regex>      # e.g. application/svc-*
  environment: dev|stage|prod
  criticality: low..critical
  default_level: L1..L5
  allowed_levels: [L1, L2, L3]
  time_windows: [...]         # business-hours-only restrictions for L4
  max_actions_per_hour: <int>
  kill_switch: false
```

## Examples

### Dev environment, low-risk action

```yaml
capability: sdlc-devops
environment: dev
criticality: low
default_level: L4
allowed_levels: [L1, L2, L3, L4, L5]
max_actions_per_hour: 60
```

### Production, critical asset (cap at L3)

```yaml
capability: sdlc-devops
environment: prod
criticality: critical
default_level: L3
allowed_levels: [L1, L2, L3]
max_actions_per_hour: 10
```

## Resolution rules

1. Match by `(capability, environment, criticality)` exactly.
2. Fall back to `(capability, environment)` when criticality is unknown.
3. If no envelope is found, the engine returns `ErrEnvelopeNotFound` and the
   incident is left at L1 by default.
4. The healing action's `allowed_levels_by_env` further constrains the
   envelope cap.
5. The kill switch overrides everything — degrades all to L1.

## CRUD

```
POST /v1/envelopes        — upsert
GET  /v1/envelopes        — list
```

Envelopes are versioned implicitly via `updated_at`. Audit events:
`policy.autonomy_envelope.granted.v1`, `policy.autonomy_envelope.revoked.v1`.

## Default templates

See `services/policy-engine/policy_templates/autonomy_envelopes.yaml`. The
templates `level-by-env`, `level-by-criticality`, and
`require-reversible-for-l5` form the baseline policy a Tenant should adopt.

## Promotion linkage

Envelopes are independent of action-level promotion. Adding a level to an
action's `allowed_levels_by_env` requires the prerequisites in
[docs/healing/levels.md](./levels.md#promotion-d610). The envelope only caps
what level the engine *applies*, never what level an action is *allowed to
have*.
