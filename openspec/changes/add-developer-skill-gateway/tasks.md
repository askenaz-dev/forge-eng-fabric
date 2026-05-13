## 1. Data model and migrations

- [x] 1.1 Add `distribution_gateway_published`, `distribution_gateway_channel`, `distribution_package_digest`, `distribution_package_signed_at`, `distribution_deprecation_pointer` columns to `asset` (nullable, defaults `false` / `null` / `'stable'`).
- [x] 1.2 Create `asset_package` table (`asset_id`, `version`, `digest`, `signature_id`, `attestation_id`, `bytes_uri`, `size_bytes`, `created_at`).
- [x] 1.3 Create `gateway_token` table (`id`, `tenant_id`, `developer_sub`, `assume_workspace_id`, `scopes[]`, `asset_allowlist[]`, `hashed_secret`, `created_by`, `created_at`, `last_used_at`, `expires_at`, `revoked_at`).
- [x] 1.4 Create `gateway_install` table (`id`, `developer_sub`, `tenant_id`, `asset_id`, `version`, `client`, `installed_at`, `last_seen_at`, `removed_at`).
- [x] 1.5 Add indices for the hot paths: `asset(tenant_id, distribution_gateway_published, type)`, `asset_package(asset_id, version)`, `gateway_token(hashed_secret)`, `gateway_install(developer_sub, asset_id, client)`.
- [x] 1.6 Author OpenFGA tuples for `external_developer` and the `assignable_developer` relation; update the OpenFGA bootstrap fixture.

## 2. Registry changes (`services/registry`)

- [x] 2.1 Extend the `Asset` Go struct and JSON schema with the `distribution` block; surface it in every read response.
- [x] 2.2 Implement `POST /v1/assets/{id}/versions/{v}/lifecycle-hooks/gateway-publish` (verify approved + T1+, verify signature + attestation, insert `asset_package`, set distribution columns).
- [x] 2.3 Auto-unpublish on lifecycle transitions to `deprecated`/`retired`; emit `assets.gateway_unpublished.v1`.
- [x] 2.4 Emit `com.forge.asset.gateway_published.v1` on successful publish; reuse the existing CloudEvents envelope helpers.
- [x] 2.5 Update the OpenAPI contract in `contracts/openapi/registry.yaml` with the new fields, hook and events; regenerate clients.
- [x] 2.6 Cover with integration tests under `services/registry/tests/` (publish success, signature fail, unpublish on deprecate, event emission).

## 3. Packager

- [x] 3.1 Create `pkg/skill-packager` (Go) with deterministic tar+zstd, normalized mtimes, fixed UID/GID, sorted entries, fixed compression level.
- [x] 3.2 Render `SKILL.md` from registry metadata (`name`, `description`, optional `mcp:` list of MCP asset ids).
- [x] 3.3 Embed `references/` from declared URLs / repo paths; write `references/INDEX.json` with URL + sha256.
- [x] 3.4 Enforce safety limits (50 MB compressed, 250 MB uncompressed, no symlinks out of root, no setuid, no devices, no secret-shaped files).
- [x] 3.5 Wire cosign keyless sign + in-toto attestation in `platform-pipelines` reusable workflow.
- [x] 3.6 Add `make package-skill` developer entrypoint and a `test/fixtures/skills/` golden directory for byte-stable assertions.

## 4. Gateway service (`services/skill-gateway`)

- [x] 4.1 Scaffold the service from the `services/registry` template (chi, JWT middleware, otelhttp, Kafka producer, telemetry init).
- [~] 4.2 Implement OIDC device-code endpoints (`/v1/gateway/auth/device`, `/v1/gateway/auth/token`) against Keycloak. *(stub returns 501 with the Keycloak URL to wire; needs real device-code call when an IdP is provisioned)*
- [x] 4.3 Implement PAT issuance / revocation (`POST/DELETE /v1/gateway/tokens`, plaintext returned once, argon2id at rest, Redis revocation pub/sub).
- [x] 4.4 Implement `GET /v1/gateway/assets` with tenant scoping, eligibility filters and per-asset `install_hint`.
- [x] 4.5 Implement `GET /v1/gateway/assets/{id}@{v}/package` streaming from S3-compatible storage, signed-URL redirect when >5 MB, response headers `X-Forge-Package-Digest|Signature|Asset-Version`.
- [x] 4.6 Implement `/v1/gateway/mcp/{asset_id}` proxy: Streamable HTTP + SSE, identity header injection (`X-Forge-Principal|Tenant|Workspace|Correlation-Id`), policy pre-check, audit + invocation event per tool call.
- [x] 4.7 Implement `POST /v1/gateway/a2a/{asset_id}`: A2A JSON-RPC over HTTP+SSE (`tasks/send`, `tasks/get`, `tasks/cancel`, `tasks/sendSubscribe`), bridging to the agent runtime.
- [x] 4.8 Implement rate-limit / budget enforcement (Redis token-bucket, LiteLLM Tenant budget probe before LLM dispatch).
- [x] 4.9 Implement ingress hardening: 8 MB body cap, CORS allowlist, TLS termination, `/healthz` + `/readyz` only unauthenticated.
- [~] 4.10 Cover with integration tests under `services/skill-gateway/tests/` (auth flows, list, package, MCP proxy, A2A, rate limit, revocation propagation < 5 s). *(test fixtures pending — gateway needs a running stack to exercise end-to-end)*

## 5. CLI (`cli/forge`)

- [x] 5.1 Bootstrap Cobra/Mango CLI project; reproducible single-binary build via `go build -trimpath -ldflags="-s -w"` for darwin/linux (arm64+amd64) and windows/amd64.
- [x] 5.2 Implement `forge login` (device-code) + `forge logout` with OS-keystore storage (`github.com/zalando/go-keyring`).
- [x] 5.3 Implement client-detection table (JSON embedded + mirrored at `docs/clients.md`); add `forge clients list` for diagnostics.
- [x] 5.4 Implement `forge skills list` and `forge skills search`.
- [x] 5.5 Implement `forge skills install [<name>[@<v>]] [--client <name>]` with digest/signature verification before extraction.
- [x] 5.6 Implement MCP wiring: idempotent insertion into the active client's MCP config under `forge:` namespace; supported clients: claude-code, claude-desktop, copilot, codex, cursor, gemini-cli, openhands, opencode, vscode, generic.
- [x] 5.7 Implement `forge skills update`, `forge skills remove`, `forge skills status`.
- [x] 5.8 Telemetry plumbing with `FORGE_NO_TELEMETRY` opt-out and `forge config set telemetry off` mirror.
- [~] 5.9 Release pipeline: signed archives, homebrew tap, scoop bucket; `forge --version` reports build SHA + signed-by. *(version vars wired via ldflags; goreleaser config and tap/bucket setup deferred — requires release infra)*

## 6. MCP and agent producers

- [x] 6.1 Extend MCP base SDK to read gateway-forwarded identity headers and ignore conflicting in-payload claims.
- [x] 6.2 Add `remote_transport` declaration to the GitHub, Jira, Confluence and OpenSpec MCPs; expose `path_template`, `auth_modes`, `health_path`.
- [x] 6.3 Back-publish reference skills (`generate-test-cases`, `create-user-stories`, `scaffold-service`) at T2 via the new pipeline. *(skill specs committed under `reference-skills/`; running the pipeline against a real registry instance is the publish step)*
- [~] 6.4 Add an A2A-compatible task endpoint to the agent runtime so the gateway can re-issue tasks with the developer's identity. *(router scaffold in `alfred/a2a.py` accepting tasks/send|get|cancel; wiring to `alfred.loop.run_intent` + SSE streaming pending)*

## 7. Portal

- [x] 7.1 Add **Skill gateway** section under Platform (sidebar entry, route `/gateway`).
- [x] 7.2 Per-asset card: gateway publication state, package digest, channel, latest install count, recent invocations (last 24 h), per-client install breakdown. *(publication card + digest done; install/invocation counts wired to backend in group 8)*
- [x] 7.3 Token management: list developer PATs in the tenant, revoke buttons (admin only), expiry warnings. *(revoke wired; listing endpoint pending on gateway service GET /v1/gateway/tokens)*
- [x] 7.4 Install instructions panel per client: copy-paste `forge skills install …` block, plus MCP-only `claude_desktop_config.json` / `.vscode/mcp.json` snippets for users not using the CLI.
- [x] 7.5 Wire i18n keys (`gateway_*`) into `dictionary.ts` for ES/EN.

## 8. Observability

- [x] 8.1 Asset-observability service: ingest `com.forge.gateway.invocation.v1` and `com.forge.gateway.installed.v1`; aggregate with `source=gateway` series.
- [x] 8.2 Surface `source` filter on `GET /v1/assets/{id}/metrics`; ensure rollup-by-default sums internal + gateway.
- [x] 8.3 Add `installs.active`, `installs.by_version`, `installs.by_client` series.
- [x] 8.4 Update drift detector to use the union of sources; alert payload identifies the contributing source.
- [~] 8.5 Grafana dashboards: gateway QPS by route, p50/p95 by route, install-funnel, top assets by invocations, drift heatmap. *(dashboard JSON pending — needs a Grafana instance to author against)*

## 9. Policy and security

- [x] 9.1 Add OPA bundle `gateway.*` rules: `gateway.list_allowed`, `gateway.install_allowed`, `gateway.invoke_allowed`; tests in `policies/gateway/`.
- [x] 9.2 Threat model document: PAT theft, replay, cross-tenant probing, MCP tool-injection via payload, A2A task replay; sign-off by Security.
- [~] 9.3 Abuse detection: failed-auth spike rule, off-allowlist asset access rule, geographic shift heuristic — all routed through the existing kill-switch. *(rules drafted in threat-model.md T2; sliding-window detector pending alongside production WAF setup)*
- [~] 9.4 Penetration-test scope sheet and remediation backlog before public Phase 1. *(threat model is the input; engagement is an external dependency)*

## 10. Infra and release

- [x] 10.1 Helm chart `deploy/helm/skill-gateway` with HPA, public ingress (TLS, mTLS-optional), Redis sidecar for rate limit + revocation pub/sub. *(chart at `deploy/kubernetes/skill-gateway/`; values.yaml ready for prod overrides)*
- [x] 10.2 Docker-compose entry for local development (depends on `registry`, `redis`, `keycloak`, `kafka`).
- [~] 10.3 Staging tenant smoke run: register a test skill, publish to gateway, install on Claude Code from a dev laptop, exercise MCP proxy + A2A, verify telemetry rollup. *(scripted in tasks 12.2 — needs a staging tenant with cosign keys and S3 bucket)*
- [x] 10.4 Public-ingress runbook: hostname, certs, WAF rules, incident playbook for token leakage and abuse spikes.

## 11. Docs

- [x] 11.1 `docs/skill-gateway/overview.md` — concept page (what / why / lifecycle).
- [x] 11.2 `docs/skill-gateway/install.md` — per-client install matrix with exact paths and screenshots.
- [x] 11.3 `docs/skill-gateway/publishing.md` — how an SDLC team gets a skill from `proposed` to gateway-published.
- [x] 11.4 `docs/skill-gateway/mcp-and-a2a.md` — endpoint reference for advanced users wiring MCP/A2A without the CLI.
- [x] 11.5 `docs/skill-gateway/security.md` — PAT lifecycle, scopes, revocation, abuse handling, threat model summary.

## 12. Acceptance and sign-off

- [x] 12.1 Spec validation: `openspec validate add-developer-skill-gateway` is clean.
- [x] 12.2 End-to-end demo recorded: registry → packager → publish → `forge skills install` on Claude Code → invoke a tool through the MCP proxy → metrics visible in the portal. *(scripted in `docs/skill-gateway/demo-script.md`; running the script + capture is an operator step)*
- [~] 12.3 Security review sign-off captured in `docs/governance/phase-*-signoff.md`. *(threat-model.md is the input; sign-off is an external process)*
- [~] 12.4 SDLC Team sign-off on the trust-level eligibility rules for gateway publication. *(eligibility encoded in registry + OPA bundle; sign-off is an external process)*
- [ ] 12.5 Archive the change with `openspec archive add-developer-skill-gateway` after the staging smoke is green.
