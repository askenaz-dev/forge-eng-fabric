# Data Retention Policy Draft

Status: draft for Security/Compliance validation.

This document defines the Phase 0 baseline retention targets for Forge data classes. It is intentionally conservative and local-first; cloud storage lifecycle rules and archival jobs are tracked separately.

## Data Classifications

| Classification | Examples | Default handling |
|---|---|---|
| `public` | Public docs, generated examples | Store only when needed |
| `internal` | Operational metadata, non-sensitive logs | Standard platform access controls |
| `confidential` | Tenant metadata, workspace metadata, private repository metadata | Tenant-scoped access controls and audit |
| `restricted` | Secrets, credentials, private keys, regulated data | Avoid storing; if unavoidable, require explicit approval and encryption controls |

## Retention Targets

| Data set | Public | Internal | Confidential | Restricted | Notes |
|---|---:|---:|---:|---:|---|
| Audit events | 90 days | 1 year | 7 years | Security-approved only | Phase 0 stores audit in Postgres; object-storage archive is deferred |
| Operational logs | 14 days | 30 days | 90 days | Do not log | Loki TTL enforcement is deferred |
| Metrics | 30 days | 90 days | 180 days | Do not emit | Prometheus retention is local/default in Phase 0 |
| Traces | 7 days | 14 days | 30 days | Do not trace payloads | Tempo retention is local/default in Phase 0 |
| AI traces | 7 days | 30 days | 90 days | Redact or disable | Cost/latency telemetry is preferred over prompt/body retention |
| RAG data | Until source removal | Until source removal | Until source removal plus review | Not allowed by default | Production RAG ingestion starts in Phase 1 |

## Phase 0 Enforcement Status

| Control | Status |
|---|---|
| Audit append-only storage | Implemented with Postgres trigger and hash chain |
| Audit archival to object storage | Deferred pending GCS bucket and retention policy approval |
| Loki/Tempo retention limits | Deferred; local stack uses development defaults |
| Secret redaction in logs | Required by convention; automated scanners deferred |
| RAG data deletion | Deferred until RAG ingestion exists |

## Review Checklist

1. Confirm legal retention requirements for audit events by tenant type.
2. Confirm whether AI traces may include prompts/responses or only metadata.
3. Confirm deletion requirements for RAG-derived embeddings when source documents are removed.
4. Convert approved targets into storage lifecycle policies, database jobs, and dashboard alerts.
