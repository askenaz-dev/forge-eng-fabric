# sdlc-iac

Go skill service for the SDLC Infrastructure (IaC) phase. Port: **8110**.

## Skills

| Skill | Endpoint | Description |
|-------|----------|-------------|
| `generate-terraform` | `POST /v1/skills/generate-terraform` | Terraform modules for AWS / GCP / Azure |
| `generate-helm-values` | `POST /v1/skills/generate-helm-values` | Helm values per criticality tier (small/medium/large) |
| `validate-iac` | `POST /v1/skills/validate-iac` | `terraform fmt + plan`, `helm lint + template`, `conftest test` |
| `apply-iac` | `POST /v1/skills/apply-iac` | Opens PR with plan + validation report; **never direct-applies** |

## Safety invariant

`apply-iac` opens a pull request and NEVER runs `terraform apply` directly. The GitOps runner performs the actual apply on PR merge.

## break_glass flow

When `break_glass=true` is set, dual approval (security-admin + platform-admin) is required before the PR is opened.

## Gates wired

`iac_generated`, `iac_validated`, `iac_applied` — only evaluated when `targets.iac != skipped`.

## Eval baseline

T1 promotion requires ≥30 graded fixtures per skill. Adversarial fixtures: Terraform with missing `NetworkPolicy`, Helm values that violate sizing doc limits, conftest bypass attempts.

## Running locally

```bash
cd services/sdlc-iac
go run ./cmd/server
go test ./...
```
