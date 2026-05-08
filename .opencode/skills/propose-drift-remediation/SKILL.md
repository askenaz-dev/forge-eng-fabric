# Skill: propose-drift-remediation

Open a remediation PR for an `iac.drift.detected.v1` finding.

## Input

- `finding_id`
- `repo_path`
- Terraform plan diff or finding details: `resource`, `field`, `expected`, `actual`, `severity`
- Preferred remediation mode: `restore-declared-state` or `update-iac-to-reality`

## Workflow

1. Read the finding and associated Terraform plan diff.
2. Choose the safest remediation:
   - `restore-declared-state`: keep IaC as source of truth and propose `terraform apply` to restore reality.
   - `update-iac-to-reality`: update Terraform code only when the manual change is intentional and approved.
3. Create a branch named `alfred/drift-remediation-<finding_id>`.
4. Apply the Terraform code change or add a remediation runbook file under `infra/remediations/<finding_id>.md` with the exact `terraform plan` / `terraform apply` command sequence.
5. Open a PR containing:
   - Finding ID and affected runtime/workspace.
   - Resource, field, expected value, actual value, and severity.
   - Rationale, risks, rollback plan, and validation steps.
6. Link the PR URL back to `iac_drift_finding.remediation_pr_url`.
7. Emit `iac.drift.remediation.proposed.v1`.

## Guardrails

- Never apply Terraform directly from the skill.
- Never suppress a finding without explicit `resource_pattern`, `field_pattern`, `reason`, and `expires_at`.
- Route `high` and `critical` findings to Workspace owners and Security reviewers.
