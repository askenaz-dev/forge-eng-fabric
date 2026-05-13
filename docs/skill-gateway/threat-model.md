# Skill gateway — threat model

This is the working threat model for the developer skill gateway. It captures the surfaces, assets, threats and the controls we ship with in this change. Security review sign-off is recorded in `docs/governance/phase-*-signoff.md` once executed.

## Scope

The `services/skill-gateway` HTTP service, public ingress, the `forge` CLI on developer machines, PATs issued by the gateway, and the packaged Agent Skills bundles served by the gateway. Out of scope: the in-platform runtime (already covered by `agentic-execution-platform`), the registry itself (covered by `ai-asset-registry`).

## Assets to protect

| Asset | Why |
|---|---|
| Tenant data accessible through MCP proxy / A2A | Cross-tenant leak is a P0 incident |
| Developer PAT plaintexts | Replay/escalation if leaked |
| OIDC tokens stored in the OS keystore | Persist across sessions; their leak is equivalent to PAT leak |
| Package bundles | Tampered bundle = supply-chain compromise on developer machines |
| Registry-stored attestations / digests | Trust anchor for the whole distribution |

## Attack surfaces

1. Public HTTPS ingress (the only externally reachable surface).
2. The CLI on a developer laptop (storage, browser flow, MCP config writes).
3. The S3-compatible package store (signed URLs, bucket policy).
4. The packager pipeline (sign + attest).

## Threats and mitigations

### T1 — PAT theft (developer laptop compromise)

- Mitigation: 90-day max lifetime, asset allowlist on PATs, immediate revocation via Redis pub/sub, OS keystore-only storage in the CLI (no `~/.forge/token` plaintext), anomaly detection on geographic IP shifts.
- Open: keystore export by other apps on the same user account is a residual risk; document in install guide.

### T2 — PAT replay from a different machine

- Mitigation: anti-abuse fingerprint (IP /24 + UA + dev_sub) feeds a Redis sliding-window. Spikes trip the existing kill-switch and quarantine the PAT for human review.

### T3 — Cross-tenant probing via guessable workspace IDs

- Mitigation: tenant scoping enforced in `services/skill-gateway` before the registry call; OpenFGA `assignable_developer` relation is checked on PAT issuance, not on every request, so the issuance step is the single audit gate.

### T4 — Tampered package on the developer machine

- Mitigation: package bytes addressed by sha256 and signed with cosign + in-toto attestation; the CLI verifies sha256 == X-Forge-Package-Digest before extraction and refuses on mismatch. Bundle attestation is recorded in the registry so the developer can re-verify with `cosign verify-blob`.

### T5 — Prompt-injection through MCP tool inputs

- Mitigation: gateway-forwarded identity headers override any inbound claims; MCP SDK records `header_override=true` in audit when a payload tried to override. The platform guardrails (`agentic-guardrails`) still apply to the underlying tool call.

### T6 — A2A task replay

- Mitigation: every A2A `tasks/send` carries an idempotency key (the task `id`); the gateway records (developer, asset, task_id) and refuses replays within the configurable de-dupe window.

### T7 — Public ingress abuse (DoS, scraping)

- Mitigation: 8 MB body cap, Redis token-bucket per PAT, WAF in front of the ingress, no anonymous endpoints other than `/healthz` and `/readyz`. CORS allowlist for browser clients.

### T8 — Provider provider-key abuse via brokered LLM access

- Mitigation: every LLM call still flows through LiteLLM with the Tenant budget; the gateway refuses with `402 budget_exhausted` *before* model dispatch when the Tenant or per-PAT soft cap is exhausted.

### T9 — Packager pipeline compromise

- Mitigation: keyless cosign sign + in-toto attestation tying the bundle to a specific commit SHA and a specific pipeline run; the registry rejects any publish whose signature does not chain to the platform's trusted Fulcio identity.

## Residual risks (accept / track)

- A compromised developer with `gateway.invoke` can still call any approved asset their workspace permits — that is the expected blast radius and is bounded by per-PAT budget caps and audit.
- The signed-redirect URL exposes the underlying object store URL for ≤10 minutes. Bucket policy MUST deny object overwrite during that window.
- The CLI's `--client generic` path drops files into `~/.agentskills/` regardless of the host environment; documented as user-controlled.

## Open follow-ups

- Pentest engagement scoped before Phase 1 cutover (referenced in tasks 9.4).
- Production WAF ruleset and abuse-rule thresholds owned by Platform Engineering + Security.
- Per-tenant subdomain certificates (decision 4 in `design.md`) — automate via cert-manager.
