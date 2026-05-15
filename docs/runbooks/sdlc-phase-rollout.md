# Runbook: SDLC Phase Rollout

**Scope:** Per-tenant feature flag rollout for new SDLC phases and capabilities.  
**Last validated:** 2026-05-14  
**Control-plane endpoint (local):** `http://localhost:8082`  
**Control-plane endpoint (staging/prod):** use the service URL from your environment's secret manager.

---

## Overview

New SDLC phases (`iac`, `observability`) and cross-cutting capabilities are gated behind per-tenant feature flags stored in the `feature_flags` JSONB column of the `tenant` table. This runbook describes the three-stage rollout sequence, the monitoring signals to watch at each stage, and the rollback procedure.

The feature flag API is served by `services/control-plane` at `/v1/tenants/{tenant_id}/feature-flags`. The `{tenant_id}` may be a UUID **or** the tenant name/slug (e.g. `local`).

---

## Sequence

### Stage 1: Platform tenant (local / internal)

Enable all flags on the platform tenant first and validate for **48 hours** before advancing.

The platform tenant is typically named `local` in development and `forge-internal` in staging/production.

```bash
# Resolve the platform tenant UUID
TENANT_ID=$(curl -s http://localhost:8082/v1/tenants | jq -r '.[] | select(.name=="local") | .id')
echo "Platform tenant: $TENANT_ID"
```

Enable all flags:

```bash
curl -s -X PATCH http://localhost:8082/v1/tenants/${TENANT_ID}/feature-flags \
  -H "content-type: application/json" \
  -d '{
    "forge.sdlc.architecture_skills.enabled": true,
    "forge.sdlc.design_skills.enabled": true,
    "forge.sdlc.qa_skills.enabled": true,
    "forge.sdlc.iac.enabled": true,
    "forge.healing.l1_l2.enabled": true,
    "forge.workflow.intent_to_infrastructure.enabled": true,
    "forge.registry.public_origin.enabled": true
  }'
```

Verify:

```bash
curl -s http://localhost:8082/v1/tenants/${TENANT_ID}/feature-flags | jq .
```

**Validation gate (48h):** Run the reference workflow and confirm all phases complete without errors:

```bash
make demo-intent-to-infrastructure
```

Expected outcome: JSON report at `build/demo-intent-to-infrastructure/<timestamp>.json` with `"success": true`.

### Stage 2: Pilot tenants

Enable on **2 pilot tenants**. Monitor for **1 week** before advancing to Stage 3.

The `iac`, `workflow.intent_to_infrastructure`, and `registry.public_origin` flags are pilot-opt-in: enable them only after the pilot tenant's owner has agreed to participate.

```bash
# For each pilot tenant:
PILOT_TENANT_ID=<uuid>

# Required capabilities (all pilot tenants):
curl -s -X PATCH http://localhost:8082/v1/tenants/${PILOT_TENANT_ID}/feature-flags \
  -H "content-type: application/json" \
  -d '{
    "forge.sdlc.architecture_skills.enabled": true,
    "forge.sdlc.design_skills.enabled": true,
    "forge.sdlc.qa_skills.enabled": true,
    "forge.healing.l1_l2.enabled": true
  }'

# Opt-in capabilities (only with pilot tenant agreement):
curl -s -X PATCH http://localhost:8082/v1/tenants/${PILOT_TENANT_ID}/feature-flags \
  -H "content-type: application/json" \
  -d '{
    "forge.sdlc.iac.enabled": true,
    "forge.workflow.intent_to_infrastructure.enabled": true,
    "forge.registry.public_origin.enabled": true
  }'
```

### Stage 3: Global rollout

Enable all flags across every non-archived tenant. Run as a migration or automation job.

```bash
# List all active tenant IDs:
curl -s http://localhost:8082/v1/tenants | jq -r '.[].id' | while read tid; do
  curl -s -X PATCH http://localhost:8082/v1/tenants/${tid}/feature-flags \
    -H "content-type: application/json" \
    -d '{
      "forge.sdlc.architecture_skills.enabled": true,
      "forge.sdlc.design_skills.enabled": true,
      "forge.sdlc.qa_skills.enabled": true,
      "forge.sdlc.iac.enabled": true,
      "forge.healing.l1_l2.enabled": true,
      "forge.workflow.intent_to_infrastructure.enabled": true,
      "forge.registry.public_origin.enabled": true
    }' > /dev/null
  echo "Updated $tid"
done
```

---

## Feature flags per stage

| Flag | Stage 1 | Stage 2 | Stage 3 |
|------|---------|---------|---------|
| forge.sdlc.architecture_skills.enabled | ✓ | ✓ | ✓ |
| forge.sdlc.design_skills.enabled | ✓ | ✓ | ✓ |
| forge.sdlc.qa_skills.enabled | ✓ | ✓ | ✓ |
| forge.sdlc.iac.enabled | ✓ | pilot-opt-in | ✓ |
| forge.healing.l1_l2.enabled | ✓ | ✓ | ✓ |
| forge.workflow.intent_to_infrastructure.enabled | ✓ | pilot-opt-in | ✓ |
| forge.registry.public_origin.enabled | ✓ | pilot-opt-in | ✓ |

---

## Enable a flag for a tenant

```bash
curl -s -X PATCH http://localhost:8082/v1/tenants/{tenant_id}/feature-flags \
  -H "content-type: application/json" \
  -d '{"forge.sdlc.architecture_skills.enabled": true}'
```

The PATCH endpoint merges the supplied keys into the existing map — keys not present in the request body are left unchanged.

## Verify flags

```bash
curl http://localhost:8082/v1/tenants/{tenant_id}/feature-flags | jq .
```

You can also verify by name/slug when the UUID is unknown:

```bash
curl http://localhost:8082/v1/tenants/local/feature-flags | jq .
```

## Rollback

Set the flag to `false`:

```bash
curl -s -X PATCH http://localhost:8082/v1/tenants/{tenant_id}/feature-flags \
  -H "content-type: application/json" \
  -d '{"forge.sdlc.architecture_skills.enabled": false}'
```

To roll back all flags for a tenant at once:

```bash
curl -s -X PATCH http://localhost:8082/v1/tenants/{tenant_id}/feature-flags \
  -H "content-type: application/json" \
  -d '{
    "forge.sdlc.architecture_skills.enabled": false,
    "forge.sdlc.design_skills.enabled": false,
    "forge.sdlc.qa_skills.enabled": false,
    "forge.sdlc.iac.enabled": false,
    "forge.healing.l1_l2.enabled": false,
    "forge.workflow.intent_to_infrastructure.enabled": false,
    "forge.registry.public_origin.enabled": false
  }'
```

Rollback takes effect immediately — no restart required.

---

## Monitoring signals per stage

Import the Grafana dashboard at `docs/dashboards/sdlc-orchestrator.json` before beginning Stage 1.

### Stage 1 (Platform tenant, 48h)

| Signal | Where to look | Threshold to advance |
|--------|--------------|---------------------|
| `sdlc.initiative.phase.failed` event rate | Kafka / Grafana `sdlc-orchestrator` dashboard | < 5% of phase completions |
| `forge.sdlc.iac` phase gate failure rate | `sdlc-orchestrator` logs, `phase=iac gate_outcome=failed` | < 5% of iac phase runs |
| Alfred error rate (`5xx`) | `alfred` Grafana panel, `alfred_request_total{status=~"5.."}` | < 1% of requests |
| Healing L1/L2 false-positive rate | `healing-engine` logs, `healing.action.false_positive` | < 10% of triggered actions |
| `demo-intent-to-infrastructure` e2e success | `build/demo-intent-to-infrastructure/<timestamp>.json`, `"success": true` | 3 consecutive successful runs |

### Stage 2 (Pilot tenants, 1 week)

| Signal | Where to look | Threshold to advance |
|--------|--------------|---------------------|
| Per-tenant error rate | Loki query: `{service="sdlc-orchestrator", tenant_id=~"<pilot1>|<pilot2>"} |= "error"` | < 5% of requests per tenant |
| `forge.sdlc.iac.enabled` activation count | Kafka consumer group lag on `sdlc.initiative.created.v1` | Stable (no runaway re-delivery) |
| Pilot tenant SDLC completion rate | Grafana `sdlc_initiative_completed_total` by `tenant_id` | > 90% for architecture + qa phases |
| IaC generation latency p95 | Tempo traces for `sdlc-orchestrator`, span `iac.generate` | < 120 s |
| Registry public-origin lookup errors | `registry` logs, `"public_origin" AND level=error` | 0 errors per 24h window |
| Approvals backlog | `approvals` service dashboard, `approvals_pending_total` by tenant | No queue > 24h old |
| Pilot tenant escalations | Slack `#forge-pilot-feedback` | Track; no blocker unaddressed > 4h |

### Stage 3 (Global)

| Signal | Where to look | Threshold for healthy rollout |
|--------|--------------|-------------------------------|
| Global `5xx` rate (control-plane) | Prometheus `http_requests_total{service="control-plane", status=~"5.."}` | < 0.5% for 30 min after each batch |
| SDLC orchestrator queue depth | Kafka consumer group `sdlc-orchestrator` lag | < 1000 messages, not growing |
| Database connection pool exhaustion | `control-plane` logs, `pgxpool: acquire timeout` | 0 occurrences |
| Feature flag propagation delay | Spot-check 5 random tenants via the API within 15 min of batch write | All return expected flags |
| Healing action error budget | Grafana `healing_action_error_rate` | < 2% globally |

### Alerting rules

Existing Grafana alert rules in `infra/grafana/alerts/sdlc-orchestrator.yaml` cover:
- `SDLCPhaseFailureRateHigh` — fires at > 10% phase failure rate per tenant (15 min window)
- `HealingFalsePositiveRateHigh` — fires at > 15% false positive rate (1 h window)
- `ControlPlaneHighErrorRate` — fires at > 1% 5xx rate (5 min window)

Add a silence for the platform tenant during the first 4 hours of Stage 1 to suppress noise from initial flag propagation:

```bash
# Grafana API — add a 4h silence for the platform tenant
curl -X POST http://localhost:3000/api/alertmanager/grafana/api/v2/silences \
  -H "content-type: application/json" \
  -d '{
    "matchers": [{"name": "tenant_id", "value": "<platform_tenant_id>", "isRegex": false}],
    "startsAt": "'$(date -u +%Y-%m-%dT%H:%M:%SZ)'",
    "endsAt": "'$(date -u -d '+4 hours' +%Y-%m-%dT%H:%M:%SZ)'",
    "comment": "Stage 1 flag rollout — initial propagation window",
    "createdBy": "operator"
  }'
```

---

## Contacts and escalation

| Role | Responsibility |
|------|---------------|
| Platform Engineer on-call | First responder; owns rollback decision |
| Platform Architecture | Approves Stage 2 → Stage 3 advancement |
| Pilot tenant tech lead | Confirms pilot validation complete before Stage 3 |
| FinOps | Reviews resource impact of `iac` phase activation at Stage 3 scale |

Escalation channel: `#forge-platform-oncall` in Slack.

---

## Related runbooks

- [`docs/runbooks/intent-to-infrastructure-demo.md`](intent-to-infrastructure-demo.md) — end-to-end workflow validation
- [`docs/runbooks/alfred-console-rollout.md`](alfred-console-rollout.md) — Alfred console v2 rollout (separate flag set)
- [`docs/runbooks/observability.md`](observability.md) — Grafana/Loki/Tempo access

---

## Change log

| Date | Change | Author |
|------|--------|--------|
| 2026-05-14 | Initial version: iac + observability phases, 3-stage rollout | Platform Engineering |
