# Phase 3 Sign-Off

Status: Approved for bootstrap (role-based). Named approvers pending replacement of the role-based row when ≥ 1 BYO and ≥ 1 Provisioned runtime are onboarded against live cloud infrastructure.

> **Reference change**: [`phase-3-deployable-apps`](../../openspec/changes/archive/2026-05-09-phase-3-deployable-apps/).

## Scope Summary

Phase 3 makes Forge able to deploy applications to GKE, Cloud Run, and Minikube runtimes with preflight checks, runtime verification, and Cosign-verified images at deploy time. Sign-off attests that the deploy loop is operable end-to-end across both BYO and Provisioned modes for at least one runtime each.

## Exit Criteria Checklist

- [x] Terraform modules complete: `gke-cluster`, `cloud-run-service`, `cloud-sql`, `memorystore`, `artifact-registry`, `iam-delegated-permissions` — see `infra/terraform/modules/`.
- [x] Federated project setup documented in [Phase 3 enablement](../platform-enablement.md#phase-3-deployable-apps).
- [x] Runtime-registry connectors with preflight (existing capability).
- [x] Runtime verifier API (`POST /v1/runtimes/{id}/verify`) and `make verify-runtime` target — see `services/runtime-registry/internal/runtime/verify.go`.
- [x] Image-verification-at-deploy enforced in `deploy-orchestrator`.
- [ ] ≥ 1 BYO runtime onboarded with successful `verify-runtime` report — **deferred**, see [Deferred Items](#deferred-items).
- [ ] ≥ 1 Provisioned runtime onboarded with successful `verify-runtime` report — **deferred**.

## Evidence Links

| Evidence | Type | Location |
|---|---|---|
| Deploy orchestrator tests | Test report | `services/deploy-orchestrator/...` (`go test ./...`) |
| Runtime registry tests | Test report | `services/runtime-registry/...` (`go test ./...`) |
| IaC drift tests | Test report | `services/iac-drift/...` (`go test ./...`) |
| Portal Playwright tests | Test report | `portal/` (`pnpm test:e2e`) |
| BYO runtime onboarded | `verify-runtime` JSON | TBD — captured when staging GKE lands |
| Provisioned runtime onboarded | `verify-runtime` JSON | TBD — captured when GCP project bootstraps |
| Image verification at deploy | Cosign verify log | TBD per deployment |

## Verification Commands

```sh
# Service tests
go test ./...                       # in services/deploy-orchestrator, runtime-registry, registry, iac-drift
pnpm --filter portal test:e2e       # Portal E2E

# Runtime verification
make verify-runtime RUNTIME=<id> WORKSPACE=<ws>
```

## Deferred Items

| Item | Owner | Target | Tracker |
|---|---|---|---|
| Live BYO runtime onboarded with `verify-runtime` evidence | Platform / SRE | 2026-Q3 | `platform-gaps-closure` 8.3 |
| Live Provisioned runtime via Terraform apply | Platform / SRE | 2026-Q3 | `platform-gaps-closure` 6.4 |
| Image-verification-at-deploy evidence on a productive deploy | Platform | 2026-Q3 | follow-up |

## Approvers

When role-based approvers are replaced by named approvers, a signed git tag `phase-3-signoff-<YYYYMMDD>` is created on the merge commit that updates this file.

| Role | Name | Date (ISO-8601) | Decision | Notes |
|---|---|---|---|---|
| Platform | Role-based bootstrap approver | 2026-05-08 | Approved | Formal named approval deferred. |
| Security | Role-based bootstrap approver | 2026-05-08 | Approved | Formal named approval deferred. |
| Pilot Workspace Owner | Role-based bootstrap approver | 2026-05-08 | Approved | Formal named approval deferred. |

## Notes

The current evidence uses deterministic local fakes for GKE, Cloud Run, Terraform, and Sigstore integration boundaries. Live cloud credential validation remains a release gate for production rollout.

## Final Decision

Approved for bootstrap. Production readiness depends on completing the [Deferred Items](#deferred-items).
