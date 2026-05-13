# Skill gateway — security model

This page summarises the controls; the full analysis lives in [`threat-model.md`](./threat-model.md).

## Identity

| Subject class | Where it comes from | What it can do |
|---|---|---|
| `user` | Keycloak (OIDC) | Internal portal & registry use |
| `external_developer` | First OIDC device-code login at the gateway | Issue PATs against the tenant |
| PAT (forge_pat_…) | Minted by the gateway | Used by the CLI and any tool that holds the secret |

## PAT scopes

PATs accept only the closed set:

- `gateway.read` — list assets, download packages of public/team-visible items
- `gateway.install` — install private-to-tenant packages, write MCP entries
- `gateway.invoke` — MCP proxy and A2A invocation

Any scope outside this set is refused at issuance with `400 invalid_scope`.

## PAT lifecycle

- Maximum lifetime: 90 days. The CLI prompts for rotation 7 days before expiry.
- Storage: argon2id hash at rest server-side; OS keystore on the CLI side (Keychain on macOS, Credential Manager on Windows, libsecret/keyring on Linux). Never `~/.forge/token` in plaintext.
- Revocation: via `DELETE /v1/gateway/tokens/{id}` or the portal Revoke button. Propagates to every gateway replica within 5 seconds via Redis pub/sub.

## Allowlists

A PAT may carry `asset_allowlist: [...]`. When present, even otherwise-scoped requests against assets outside the list are refused with `403 asset_not_in_allowlist`. Use this to scope a CI token to exactly the skill it needs.

## Workspace assumption

Every PAT pins one `assume_workspace_id`. The gateway only honours it when OpenFGA confirms the developer's OIDC identity has the `assignable_developer` relation on that workspace. Cross-workspace mutation attempts are refused with `403 cross_workspace_denied` and emit `guardrail.trip.v1`.

## Supply chain

- Packages are content-addressed (sha256) and cosign-signed; the CLI verifies sha256 == `X-Forge-Package-Digest` before extraction.
- The registry records the cosign signature + in-toto attestation per package; you can re-verify with:

```bash
cosign verify-blob \
  --signature foo.tar.zst.sig \
  --certificate foo.tar.zst.crt \
  --certificate-identity-regexp 'https://github.com/forge-eng-fabric/.*' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  foo.tar.zst
```

## Rate limits and budgets

- Per-PAT rate limit: 60 req/min default, configurable per tenant.
- Per-Tenant monthly LLM budget enforced by LiteLLM. The gateway probes the Tenant budget before any LLM-bearing invocation and refuses with `402 budget_exhausted` when exhausted.
- Per-PAT soft cap warns and throttles but does not block when only one developer is hot.

## Audit

Every gateway request emits `com.forge.gateway.invocation.v1`. Installs emit `com.forge.gateway.installed.v1`. The asset-observability service ingests both and rolls them up as `source=gateway`, queryable on equal footing with `source=runtime` and `source=workflow`.

## Reporting an incident

Mail `security@forge.acme.io` (or open a PagerDuty incident with severity P1) with:

- The affected PAT id (or the developer email if the PAT was lost),
- An estimated leak window,
- Any suspicious invocations the developer has noticed in the portal.

Platform Engineering follows the playbook in [`runbook.md`](./runbook.md) IR-1.
