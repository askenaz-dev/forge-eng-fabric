# Phase 2 Sign-Off

Status: Approved for bootstrap (role-based). Named approvers pending replacement of the role-based row when the GitHub App registration and reusable CI evidence are captured against a real installation.

> **Reference change**: [`phase-2-app-onboarding`](../../openspec/changes/archive/2026-05-09-phase-2-app-onboarding/).

## Scope Summary

Phase 2 makes Forge able to onboard applications: a customer GitHub App is registered, the reusable CI workflow runs lint/build/test/security gates and emits SBOM/Cosign/Trivy evidence, and built images land in Artifact Registry. Sign-off attests that the app-onboarding loop is operable end-to-end against the customer's GitHub Org.

## Exit Criteria Checklist

- [x] GitHub App registered (App ID, slug, installation ID recorded) — see [github-app runbook](../runbooks/github-app.md).
- [x] Reusable CI workflow at `forge-actions/.github/workflows/forge-ci.yml` available for scaffolded repos.
- [x] SBOM generation step (Syft) wired into the reusable CI workflow.
- [x] Cosign keyless signing step wired into the reusable CI workflow.
- [x] Trivy scan step wired into the reusable CI workflow.
- [x] Artifact Registry Terraform module at `infra/terraform/modules/artifact-registry/` ready to apply.
- [ ] Reusable CI consumed by ≥ 1 scaffolded repository **with evidence attached** — **deferred** until customer onboarding lands.
- [ ] SBOM/Cosign/Trivy evidence captured for ≥ 1 productive image — **deferred** until customer onboarding lands.

## Evidence Links

| Evidence | Type | Location |
|---|---|---|
| GitHub App registration | Screenshot + ID | `docs/governance/evidence/phase-2/github-app-registration.png` (deferred until customer org wired) |
| Reusable CI run | GitHub Actions URL | TBD per scaffolded repo |
| SBOM | Syft output | `docs/governance/evidence/phase-2/sbom-<image>.json` |
| Cosign signature | `.sig` + `.pem` | Attached to release artifact |
| Trivy report | JSON | `docs/governance/evidence/phase-2/trivy-<image>.json` |
| Artifact Registry record | URL | TBD per Tenant project |
| Umbrella chart install | Helm install log | `docs/governance/evidence/phase-3/helm-install-umbrella.log` |

## Deferred Items

| Item | Owner | Target | Tracker |
|---|---|---|---|
| Live customer GitHub App installation in production org | Platform | 2026-Q3 | `platform-gaps-closure` 6.1 |
| Reusable CI integrated with ≥ 3 scaffolded repos | SDLC | 2026-Q3 | follow-up |
| End-to-end SBOM/Cosign/Trivy evidence on a productive image | Platform | 2026-Q3 | follow-up |

## Approvers

When role-based approvers are replaced by named approvers, a signed git tag `phase-2-signoff-<YYYYMMDD>` is created on the merge commit that updates this file.

| Role | Name | Date (ISO-8601) | Decision | Notes |
|---|---|---|---|---|
| Platform Engineering | Role-based bootstrap approver | 2026-05-08 | Approved | Formal named approval deferred. |
| Security | Role-based bootstrap approver | 2026-05-08 | Approved | Formal named approval deferred. |
| Pilot Workspace Owner | Role-based bootstrap approver | 2026-05-08 | Approved | Formal named approval deferred. |

## Final Decision

Approved for bootstrap. Production readiness depends on completing the [Deferred Items](#deferred-items).
