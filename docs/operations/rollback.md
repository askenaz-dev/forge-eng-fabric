# Rollback Operations

Rollback restores the previous successful deployment revision for the same asset and environment.

Manual flow:

1. Open Portal Deployments.
2. Select `Rollback to previous`.
3. Provide an operator reason.
4. Confirm rollback.
5. Verify `deployment.rolled_back.v1` and audit timeline by `correlation_id`.

Automatic rollback can be enabled per environment. If verify fails after apply, the orchestrator emits `deployment.failed.v1`, re-applies the prior revision, and emits `deployment.rolled_back.v1`.

High-criticality deployments with non-reversible migrations require explicit `allow-non-reversible-rollback` approval.
