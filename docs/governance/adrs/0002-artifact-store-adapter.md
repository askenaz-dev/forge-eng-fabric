# ADR-0002: Artifact-Store Adapter — Pluggable Backends Per Tenant

**Status:** Accepted
**Date:** 2026-05-13
**Author:** Platform Engineering
**Reviewers:** Security, SDLC Leads, DX
**Review date:** 2026-Q4 (or sooner if customer feedback demands per-asset-family bindings)

## Context

Forge needs a private home for skill artifact bytes. The asset registry already holds metadata, digest, signature and lifecycle state for every skill version; what was missing was a governed place to store the bytes themselves so the registry record could point at them.

Three implementation paths were on the table:

1. **Native object-store-backed registry** — build a Forge-owned blob store on top of S3 / GCS / Azure Blob with our own GC, replication, retention, signed URLs, lifecycle rules and audit.
2. **Database-backed payload column** — fold small skill payloads (<1 MB) into the registry DB row.
3. **Adapter over enterprise artifact stores** — define a small Go interface and ship drivers for the four enterprise artifact stores customers already run.

The pattern we adopt mirrors LiteLLM's approach to LLM providers: a thin adapter seam, several drivers, capability flags exposing per-backend differences. Customers configure one binding per Tenant via the `artifact_store_binding` table.

## Decision

**Build `pkg/artifact-store-adapter` with a Go interface and four initial drivers.** Per-Tenant binding selects the active driver:

- `nexus` — Sonatype Nexus Repository, raw repositories
- `artifactory` — JFrog Artifactory generic repositories
- `github-packages-private` — GitHub Releases assets in a private repo
- `codeartifact` — AWS CodeArtifact, `generic` package format

Public NPM (`npmjs.org`) is **not** an offered driver. The binding layer rejects `backend=npm-public` and any driver whose `Health()` probe reports `is_public=true`. Skill artifacts are confidential by default; we never route them through a public surface.

## Rationale

| Criterion | Adapter (chosen) | Native blob store | DB-backed payload |
|---|---|---|---|
| Engineering cost | ~3 weeks (interface + 4 drivers + tests) | ~2 quarters | ~2 weeks |
| Operational surface | Adopt customer's existing tooling | Own GC, replication, signed URLs, retention, DR | Bloats DB, complicates RTO |
| Provenance chain | cosign + in-toto stays unchanged | Same | Same but lives in DB |
| Capability ceiling | Limited to LCD of drivers; flags expose gaps | Full control | Limited to DB primitives |
| Sandbox / multi-tenancy | Repo-per-tenant; well-trodden | Build from scratch | DB row-level security |
| Replacement cost | Swap driver, registry unchanged | Vendor lock-in | Migration is a DB rewrite |

The deciding factor: most customers already run one of these four. Building a native store would mean owning replication, GC, signing, mTLS, quota and DR for a domain that is already a commodity. The adapter gives us a thin, testable seam without the operational tail.

## Driver capability matrix

The Health probe returns capability flags per driver. The registry gates optional features behind these flags — e.g. retention policies are only offered when `supports_retention=true`.

| Capability | Nexus | Artifactory | GitHub Packages | CodeArtifact |
|---|---|---|---|---|
| `is_public` | false | false | false (verified via Health, refused if true) | false |
| `supports_retention` | true | true | false | false |
| `supports_signed_urls` | false | true | true | true |
| `supports_lifecycle_rules` | true | true | false | false |

Nexus and Artifactory are the most feature-complete; GitHub Packages and CodeArtifact have narrower capability surfaces but are appropriate for organizations already standardised on those stacks.

## Per-Tenant repository convention

Each driver maps a Tenant to a per-Tenant logical container:

| Driver | Container | Path within container |
|---|---|---|
| Nexus | raw repository named `forge-skills-{tenant}` | `{asset_id}/{version}/{asset_id}-{version}.tar.zst` |
| Artifactory | generic repository named `forge-skills-{tenant}` | same as Nexus |
| GitHub Packages | private repo `{owner}/forge-skills-{tenant}`, release tag `skill/{asset_id}/{version}`, single asset `{asset_id}-{version}.tar.zst` |
| CodeArtifact | per-Tenant repository `forge-skills-{tenant}` in a shared domain, generic format | `{asset_id}/{version}/{asset_id}-{version}.tar.zst` |

Per-Tenant isolation is enforced two ways:

1. The driver is constructed with credentials that grant access only to the Tenant's container.
2. The adapter's `crossTenantGuard` refuses any Object whose `TenantID` does not match the driver's bound Tenant. Tests under each driver's package exercise this guard.

## Provenance chain — unchanged

Every published skill bundle continues to carry a cosign signature and an in-toto attestation produced by the CI pipeline. The adapter is part of the publish flow but is not the trust anchor: the registry record holds the signature_id and attestation_id, and consumers re-verify the bundle's digest against the registry row before invoking the skill.

The flow:

1. CI packages the bundle deterministically (`pkg/skill-packager`).
2. CI cosign-signs the bundle with keyless OIDC identity.
3. CI generates an in-toto attestation (SLSA provenance v1).
4. CI invokes `forge-artifact-store put` (this ADR) which routes the bundle through the per-Tenant adapter and prints the resulting `artifact_pointer`.
5. CI POSTs the `lifecycle-hooks/gateway-publish` request with the pointer, signature_id, attestation_id and digest.
6. The registry records the publish event on the asset row and emits `com.forge.asset.gateway_published.v1`.

## Audit and observability

Every adapter operation emits an `adapter.AuditEvent` with op, actor, object, result, reason_code, bytes_transferred and duration_ms. The registry forwards these into the same Kafka topic as every other registry event (`forge.events`), keyed on tenant_id. The `per-asset-observability` pipeline ingests them with `source=adapter`.

OpenTelemetry metrics surface per-Tenant `bytes_in`, `bytes_out`, `latency` and `error_rate` for the four drivers via the standard otelhttp instrumentation.

## Rejected alternatives

### Native blob store on top of S3 / GCS / Azure Blob

Build Forge's own blob store layer. Rejected on the basis that we would be re-implementing well-understood enterprise primitives (replication, retention, GC, signed URLs, audit) that the customer's existing artifact store already covers. The cost of building and operating this is not justified by the marginal control gain over an adapter.

### Database-backed payload column

Store small skill payloads (<1 MB) as a `bytea` column on the asset row. Rejected because it bloats the registry DB, complicates RTO (a large payload column dominates backup/restore time), forces a different code path for "small" vs. "large" payloads and creates a quasi artifact store that policy and observability would each need to learn about separately.

### Public NPM (npmjs.org)

Use the public npm registry as a backend. Rejected on the basis that skill artifacts are confidential by default for our customers (internal tooling, customer data shapes, prompt strategies). Publishing them to a public registry would leak intellectual property and would conflict with the data-classification model in `data-retention-lifecycle`. The binding refuses this backend at construction time.

## Risks / Trade-offs

| Risk | Mitigation |
|---|---|
| Adapter feature mismatch across backends (e.g. retention semantics differ) | Capability flags returned by `Health(ctx)`; registry gates optional features on the flags; per-backend matrix documented in this ADR. |
| GitHub Releases is not, strictly speaking, "GitHub Packages" — the spec naming may confuse operators | Document the mapping in the driver godoc and in the operator runbook. The binding name `github-packages-private` is preserved for migration compatibility. |
| CodeArtifact SigV4 signing implemented in-house rather than via `aws-sdk-go-v2` | Focused signer covers exactly the four request shapes the driver uses; alternative is taking on the aws-sdk-go-v2 dep tree (~100+ Go packages transitive). Re-evaluate if we need more CodeArtifact API surface. |
| Per-Tenant single-binding rules out customers who want per-asset-family bindings (e.g. Nexus for skills + GH Packages for prompt templates) | See tasks.md 11.2 — open question to revisit before Release N+1 based on customer feedback. |
| Credentials passed via env var in CI | Acceptable for current GitHub Actions model; future iteration moves to OIDC-derived short-lived tokens via the `SecretFetcher` seam already in the adapter contract. |

## Open questions

1. **Retention policies** — only Nexus and Artifactory advertise `supports_retention=true`. For GitHub Packages and CodeArtifact, do we add a software-side retention loop (the daily cron deletes versions past the configured TTL) or refuse retention on those backends? Default proposal: refuse with `409 backend_capability_missing`, escalate to a software retention loop if customer feedback demands it.
2. **Signed URLs for portal downloads** — Artifactory and CodeArtifact support signed URLs; Nexus does not. For the Portal's "Download bundle" affordance, do we adopt signed URLs everywhere (and route Nexus through the registry as a proxy) or expose the inconsistency in the UI? Defer to portal UX review.
3. **Cross-tenant federation** — out of scope today. Revisit when the platform-foundations spec adds federation; the adapter contract is tenant-scoped end-to-end which means federation would need an explicit upgrade.
