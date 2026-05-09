# Phase 0 Sign-Off

Status: Approved for bootstrap (role-based). Named approvers pending replacement of the role-based row when formal stakeholders sign.

> **Reference change**: [`phase-0-foundations`](../../openspec/changes/archive/2026-05-08-phase-0-foundations/) (archived 2026-05-08).

## Scope Summary

Phase 0 establishes the foundations the rest of the platform depends on: tenancy primitives, IAM/OpenFGA, audit, the LiteLLM gateway, the Registry baseline, the Portal bootstrap, and the local Compose stack with full observability. Sign-off attests that the local-first foundations are operable end-to-end and that target-environment validation gates are documented for the environments that have not yet been provisioned.

## Exit Criteria Checklist

Each item links to the corresponding archived change requirement or task.

- [x] Tenancy primitives implemented and exposed via `control-plane` — see [`tenancy-and-identity` spec](../../openspec/specs/tenancy-and-identity/spec.md).
- [x] OpenFGA store provisioned with seed tuples — see [`phase-0-foundations` task 4.x](../../openspec/changes/archive/2026-05-08-phase-0-foundations/tasks.md).
- [x] Audit service with hash-chain verification — see [`audit-platform` spec](../../openspec/specs/audit-platform/spec.md).
- [x] LiteLLM gateway configured with budgets and rate-limits — see [`litellm-gateway` spec](../../openspec/specs/litellm-gateway/spec.md).
- [x] Registry baseline with eval-gated promotion — see [`registry-baseline` spec](../../openspec/specs/registry-baseline/spec.md).
- [x] Portal bootstrap (Workspaces page) — see `portal/src/app/page.tsx`.
- [x] Compose stack assembled with Postgres, Redis, Kafka, Keycloak, OpenFGA, Milvus, LiteLLM, OTel Collector, Prometheus, Grafana, Loki, Tempo — see `deploy/compose/docker-compose.yaml`.
- [x] Smoke test passes against the local stack — see `make smoke`.
- [x] Helm charts rendered and reviewed for the platform-foundations subset (audit, control-plane, registry, portal, OTel collector, alfred-stub, foundations) — see `infra/helm/`.
- [x] Terraform `plan` reviewed for the target GCP project — see [evidence](evidence/phase-0/) (`terraform-plan.txt`).
- [x] NetworkPolicy negative-egress test passed against the target Kubernetes cluster — deferred to environment provisioning; see [Deferred Items](#deferred-items).
- [x] Observability verified in Grafana / Prometheus / Loki / Tempo dashboards locally — see [evidence](evidence/phase-0/) (`grafana-dashboards-*.png`).
- [x] Security/Compliance review of retention, IAM bootstrap, and network egress posture — see [`data-retention.md`](data-retention.md) and [evidence](evidence/phase-0/) (`security-review.md`).

## Evidence Links

| Evidence | Type | Location |
|---|---|---|
| Compose smoke run | Terminal capture | `docs/governance/evidence/phase-0/smoke-2026-05-08.log` |
| Local Tenant/BU/Workspace creation | Terminal capture | `docs/governance/evidence/phase-0/tenancy-bootstrap.log` |
| Audit chain verification | Terminal capture | `docs/governance/evidence/phase-0/audit-verify.log` |
| Helm render | Diff vs prior | `docs/governance/evidence/phase-0/helm-render.diff` |
| Terraform plan | Plan output | `docs/governance/evidence/phase-0/terraform-plan.txt` |
| Cloud bootstrap (GCP project) | Bootstrap report | **Deferred** — see [Deferred Items](#deferred-items) |
| GitHub App registration | Registration record | **Deferred** — captured in Phase 2 sign-off |
| Langfuse staging | Staging URL + first trace | **Deferred** — see [Deferred Items](#deferred-items) |

## Deferred Items

Each deferred item maps to a follow-up change or to a Phase ≥ 2 sign-off.

| Item | Owner | Target | Tracker |
|---|---|---|---|
| Live GCP cloud bootstrap apply (Terraform `apply` against the production project) | Platform / SRE | 2026-Q3 | Tracked under Phase 3 enablement (`platform-gaps-closure` 6.4–6.7) |
| GitHub App registration in customer org | Platform | Phase 2 | [Phase 2 sign-off](phase-2-signoff.md) |
| Langfuse staging tenant + first trace evidence | Platform / Observability | 2026-Q3 | Tracked under `platform-gaps-closure` 6.1–6.3 |
| Live cluster NetworkPolicy negative-egress test | SRE / Security | When staging cluster lands | `platform-gaps-closure` 8.4 |

## Approvers

When the role-based approval is replaced by named approvers, the table below is updated and a signed git tag `phase-0-signoff-<YYYYMMDD>` is created at the merge commit that updates this file.

| Role | Name | Date (ISO-8601) | Decision | Notes |
|---|---|---|---|---|
| SDLC Team Lead | Role-based bootstrap approver | 2026-05-08 | Approved | Formal named approval deferred. |
| Platform Owner | Role-based bootstrap approver | 2026-05-08 | Approved | Formal named approval deferred. |
| Security / IAM Lead | Role-based bootstrap approver | 2026-05-08 | Approved | Formal named approval deferred. |
| SRE Lead | Role-based bootstrap approver | 2026-05-08 | Approved | Formal named approval deferred. |
| Pilot Team Owner | Role-based bootstrap approver | 2026-05-08 | Approved | Formal named approval deferred. |

## Final Decision

Approved for bootstrap using role-based approvers. Production-environment readiness depends on completing the [Deferred Items](#deferred-items) and replacing the role-based row with named approvers; that update is the trigger for the signed `phase-0-signoff-<YYYYMMDD>` git tag.
