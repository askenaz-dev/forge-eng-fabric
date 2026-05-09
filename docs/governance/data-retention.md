---
title: Data Retention and Lifecycle Policy
owner: platform-architecture
reviewers: [security, legal, finops]
last-reviewed: 2026-05-09
next-review: 2026-11-09
classification-of-this-document: internal
---

# Data Retention and Lifecycle Policy

This document is the canonical retention and lifecycle policy for Forge Engineering Fabric. It defines, per data type and per data classification, how long data is kept, where it is archived, when it is deleted, and how legal-hold overrides it. Helm values and runtime configuration are derived from this document; CI checks fail if they drift (see [Verification](#verification) below).

## Scope

This policy covers every in-scope data type generated or stored by Forge:

- Audit events (`audit-service` → Postgres)
- Langfuse LLM traces (Langfuse self-hosted)
- RAG content (Milvus vectors and source documents in object storage)
- Platform logs (Loki)
- Platform traces (Tempo)
- Platform metrics (Prometheus / Mimir)
- Workflow execution history (`workflow-runtime` → Postgres + event store)
- Approval records (`approvals` → Postgres)

It does **not** cover:
- Customer application data running on Workspaces (governed by the Workspace owner's own policy).
- Cloud provider service logs (governed by the cloud provider's IAM and audit-log retention).
- Source code in Git (governed by the Git provider's retention).

## Data Classification

Forge uses four classifications:

| Classification | Examples | Notes |
|---|---|---|
| `public` | Marketing-eligible release notes, public docs | No restrictions |
| `internal` | Workflow definitions, prompt templates, eval reports | Default for platform-internal artifacts |
| `confidential` | Audit events, approval records, FinOps data | PII may be present; access requires Workspace role |
| `restricted` | Secrets, OAuth tokens, customer-furnished credentials | Stored only in secret managers; not in databases or logs |

When a data row spans multiple classifications, the **highest** classification applies.

## Retention Matrix

Retention is keyed on data type, classification, and environment tier. Operational retention is the time data lives in the primary store; archival retention is the additional time data lives in cold storage before final deletion.

| Data type | Class | Local | Staging operational | Staging archive | Production operational | Production archive | Archival destination | Notes |
|---|---|---|---|---|---|---|---|---|
| Audit events | `confidential` | 7 d | 90 d | 7 y in GCS | 365 d | 7 y in GCS | `gs://forge-audit-archive-<env>/` (CMEK, 7-year object-lock) | Monthly Postgres partitions; nightly export of expired partitions to Parquet |
| Audit events | `restricted` | n/a — never persisted | n/a | n/a | n/a | n/a | n/a | Restricted values redacted at audit-write time |
| Langfuse traces | `internal` | 7 d | 30 d | 90 d in GCS | 90 d | 365 d in GCS | `gs://forge-langfuse-archive-<env>/` | Per-Workspace via Langfuse retention API |
| Langfuse traces | `confidential` | 7 d | 30 d | 365 d in GCS | 90 d | 365 d in GCS | `gs://forge-langfuse-archive-<env>/` | |
| RAG vectors (Milvus) | `internal` | 30 d TTL | 90 d TTL | n/a | 365 d TTL | n/a | n/a | Reclassification job re-evaluates retention nightly |
| RAG vectors (Milvus) | `confidential` | 30 d TTL | 90 d TTL | n/a | 180 d TTL | n/a | n/a | |
| RAG source docs (object storage) | `internal` | 30 d | 90 d | 365 d in GCS | 365 d | 3 y in GCS | `gs://forge-rag-archive-<env>/` | |
| Platform logs (Loki) | `internal` | 7 d | 14 d | n/a | 30 d | n/a | n/a | Per-tenant streams |
| Platform logs (Loki) | `confidential` | 7 d | 14 d | 90 d in GCS | 90 d | 365 d in GCS | `gs://forge-loki-archive-<env>/` | Loki ruler-based archival |
| Platform traces (Tempo) | `internal` | 3 d | 7 d | n/a | 14 d | n/a | n/a | Sampled |
| Platform traces (Tempo) | `confidential` | 3 d | 7 d | 30 d in GCS | 30 d | 90 d in GCS | `gs://forge-tempo-archive-<env>/` | |
| Platform metrics (Prometheus / Mimir) | `internal` | 7 d | 14 d | 90 d in GCS | 90 d | 365 d in GCS | `gs://forge-mimir-blocks-<env>/` | Mimir-style long-term blocks |
| Workflow execution history | `confidential` | 14 d | 90 d | 365 d in GCS | 365 d | 7 y in GCS | `gs://forge-workflow-archive-<env>/` | Includes step-level inputs, outputs, decisions |
| Approval records | `confidential` | 14 d | 90 d | 7 y in GCS | 365 d | 7 y in GCS | `gs://forge-approvals-archive-<env>/` | Co-resident with audit |

GCS buckets are CMEK-encrypted with the Workspace's KMS key. Buckets in production carry an object lock (WORM) for the archive retention.

## Archival Destinations

| Destination | Region default | Storage class | Encryption | Object-lock |
|---|---|---|---|---|
| `gs://forge-audit-archive-<env>/` | `us-central1` | `COLDLINE` after 90d | CMEK | 7 y |
| `gs://forge-langfuse-archive-<env>/` | `us-central1` | `NEARLINE` after 30d → `COLDLINE` after 365d | CMEK | none |
| `gs://forge-rag-archive-<env>/` | `us-central1` | `NEARLINE` | CMEK | none |
| `gs://forge-loki-archive-<env>/` | `us-central1` | `COLDLINE` | CMEK | none |
| `gs://forge-tempo-archive-<env>/` | `us-central1` | `COLDLINE` | CMEK | none |
| `gs://forge-mimir-blocks-<env>/` | `us-central1` | `STANDARD` (recent) → `NEARLINE` | CMEK | none |
| `gs://forge-workflow-archive-<env>/` | `us-central1` | `COLDLINE` | CMEK | 7 y |
| `gs://forge-approvals-archive-<env>/` | `us-central1` | `COLDLINE` | CMEK | 7 y |

Tenants can override the region (per their data-residency policy) by setting `tenant.archive.region` at Tenant onboarding; the platform default remains `us-central1` when no override is specified.

## Investigator Query Path

Archived audit events are queryable via BigQuery external tables over Parquet — no restoration to Postgres required.

```sh
bq query --use_legacy_sql=false \
  'SELECT event_id, principal, action, ts FROM `<project>.forge_audit_archive.events_external`
   WHERE ts BETWEEN "2026-01-01" AND "2026-01-31"
     AND tenant_id = "<tenant>"
   LIMIT 100'
```

The external table is created once per environment by Terraform (`infra/terraform/modules/audit-archive/`); the schema is the canonical `audit_event` Parquet schema. Investigators must hold the `audit.investigator` OpenFGA relation on the Tenant; the BigQuery dataset's IAM is bound to the corresponding Google group.

## Legal Hold

A Workspace, BU, or Tenant compliance principal can place an object — or a class of objects — on legal hold. Held objects are skipped by every retention job until the hold is released.

### Hold-set API

```http
POST /v1/retention/holds
Content-Type: application/json

{
  "scope": "workspace",
  "scope_id": "ws_pricing",
  "selector": {"data_type": "audit_event", "match": {"correlation_id": "abc-..."}},
  "reason": "litigation:case-2026-Acme",
  "approver": "compliance@example.com",
  "expires_at": null
}
```

The hold-set event is audited as `retention.hold.set.v1` with the principal, scope, selector, and reason. The retention job emits `retention.hold.skipped.v1` for each affected object on its next run.

### Hold-release API

```http
POST /v1/retention/holds/{hold_id}/release
Content-Type: application/json

{ "reason": "case-closed", "approver": "compliance@example.com" }
```

The release is audited as `retention.hold.released.v1`. The next retention run evaluates the affected objects normally.

## Dry-run Mode

Retention enforcement defaults to dry-run. Each environment requires:

- Helm value `retention.enforce: true` (default `false`).
- Co-approval by Platform and Security on the PR enabling enforcement.
- A link to **at least two weeks of dry-run logs** showing the intended deletions, attached to the enabling PR.

While dry-run is active, retention jobs:
- Compute every record they would delete or archive.
- Log the candidate set with classification, current age, and policy reference.
- Emit `retention.dryrun.summary.v1` audit events with totals.
- Do **not** delete or archive any records.

## Tenant Overrides

Tenants MAY:
- **Lengthen** retention beyond the platform default (e.g., 10-year audit retention for a regulated Tenant). Override is stored in the Tenant config and audited.
- **Place objects on legal hold** (no time limit).

Tenants MAY NOT:
- Shorten retention below the platform minimum. The override API rejects such requests with HTTP 422 and a clear error citing the policy minimum.
- Override the dry-run gate (Platform/Security approval still required to switch to enforcement).
- Override CMEK or object-lock posture on the archive buckets.

## Verification

A CI check (`make retention-policy-check`) parses the Loki/Tempo/Langfuse Helm values and verifies they match the rows in this document. The check runs on every PR that touches `infra/helm/**` or this document.

```sh
make retention-policy-check
```

The check fails on:
- A retention TTL in Helm values that does not match the corresponding row in the matrix above.
- A new data type added to Helm values without a corresponding row here.
- A row here that lacks corresponding Helm values when the relevant chart exists.

See `scripts/check-retention-policy.py` for the implementation.

## Change Procedure

1. Open a PR that updates the row(s) in this document.
2. Update the Helm values in the same PR.
3. Run `make retention-policy-check` locally.
4. Co-approval by **Platform Architecture, Security, and Legal**.
5. The PR description includes prior values for the change log.

## Related Documents

- [Tenancy Model](../concepts/tenancy-model.md)
- [Phase 0 Sign-Off](phase-0-signoff.md)
- [Audit service spec](../../openspec/specs/audit-platform/spec.md)
