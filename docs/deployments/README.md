# Deployments

Deployments are orchestrated through `services/deploy-orchestrator` using stages: preflight, policy, image verification, render, apply, verify, and notify.

Each deployment records `revision_id`, `image_digest`, `runtime_id`, `env`, `openspec_ids`, `pr_sha`, actor, verification status, and `correlation_id` for audit reconstruction.

Use `POST /v1/deployments/{id}/rollback` to restore the previous successful revision. Production rollbacks still pass policy evaluation and record `rollback_record` plus `deployment.rolled_back.v1`.
