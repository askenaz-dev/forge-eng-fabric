# Phase 3 Sign-Off

Status: Approved for bootstrap (role-based).

## Evidence

- Deploy orchestrator tests cover idempotent deployments, stage events, signed image verification, unsigned image override, manual rollback, auto-rollback, SSE event persistence, and deploys across Minikube, GKE BYO, and Cloud Run Provisioned connector paths.
- Runtime registry tests cover BYO credential encryption, preflight rejection, tenancy enforcement, provisioned runtime registration, state backend requirements, and destroy blocking with active deployments.
- IaC drift tests cover `terraform plan -detailed-exitcode` drift creation, severity classification, suppression validation, and remediation PR proposal events.
- Portal Playwright tests cover runtime onboarding/preflight UI, deployment history/live stages/rollback confirmation, drift findings, and remediation proposal entry points.
- Documentation exists under `docs/runtimes/`, `docs/deployments/`, `docs/iac/drift.md`, and `docs/operations/rollback.md`.

## Verification Commands

- `go test ./...` in `services/deploy-orchestrator`
- `go test ./...` in `services/runtime-registry`
- `go test ./...` in `services/registry`
- `go test ./...` in `services/iac-drift`
- `go test ./...` in `pkg/cosign`
- `npm run test:e2e` in `portal`

## Required Approvals

- Platform: approved for bootstrap by role-based approver on 2026-05-08
- Security: approved for bootstrap by role-based approver on 2026-05-08
- Pilot Workspace owner: approved for bootstrap by role-based approver on 2026-05-08

## Notes

The current evidence uses deterministic local fakes for GKE, Cloud Run, Terraform, and Sigstore integration boundaries. Live cloud credential validation remains a release gate for production rollout.

## Final Decision

Approved for bootstrap using role-based approvers. Formal named stakeholder approval can replace this record before production rollout.
