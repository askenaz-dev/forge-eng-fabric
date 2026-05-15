# Runbook: Configuring an Artifact-Store Binding

**Spec:** [active-registry-gateways](../../../openspec/changes/active-registry-gateways/) · **ADR:** [0002-artifact-store-adapter](../../governance/adrs/0002-artifact-store-adapter.md) · **Owner:** Platform Engineering

This runbook configures the per-Tenant binding to one of the four supported backends so the CI publish pipeline (`forge-skill-publish` workflow) can route skill bytes through `pkg/artifact-store-adapter` into the Tenant's preferred private store.

## Pre-flight

1. Choose a backend. Capability matrix is in [ADR-0002](../../governance/adrs/0002-artifact-store-adapter.md#driver-capability-matrix).
   - **Nexus** — raw repositories. Supports retention + lifecycle rules. No signed URLs.
   - **Artifactory** — generic repositories. Full capability set.
   - **GitHub Packages (private)** — implemented as Releases assets on a private repo. No retention; signed asset URLs only.
   - **CodeArtifact** — generic format. Signed URLs via STS. No retention; no lifecycle rules.
2. Decide the per-Tenant container naming. The default is `forge-skills-{tenant_id}` for Nexus/Artifactory/CodeArtifact and `{owner}/forge-skills-{tenant_id}` for GitHub Packages.
3. Create the credential in Vault. The runbook samples below assume the credential is reachable at `vault://t1/artifact-store/secret`.

## Step 1 — Create the per-Tenant container

| Backend | Operator action |
|---|---|
| Nexus | Create raw repository `forge-skills-t1` with `writePolicy=ALLOW_ONCE`. Grant the platform service account `nx-repository-view-raw-forge-skills-t1-add` and `*-read`. |
| Artifactory | Create generic repository `forge-skills-t1` with `Block deploys of artifacts that have the same path`. Grant the platform service account `deploy + read`. |
| GitHub Packages | Create a private GitHub repository `<org>/forge-skills-t1`. Issue a fine-grained PAT with `Contents: read+write` scoped to that repo. |
| CodeArtifact | Create a `generic` repository `forge-skills-t1` inside an existing domain. Grant IAM access via an IRSA-attached role with `codeartifact:PublishPackageVersion` + `GetPackageVersionAsset` + `DeletePackageVersions`. |

## Step 2 — Persist the binding

The binding lives on `artifact_store_binding(tenant_id, backend, config_json)`. There is no Portal flow yet; operators apply via the registry admin SQL or a one-off migration. A psql example:

```sql
INSERT INTO artifact_store_binding (tenant_id, backend, config_json)
VALUES (
  '00000000-0000-0000-0000-000000000001',
  'nexus',
  '{"base_url": "https://nexus.acme.example", "repo_prefix": "forge-skills"}'::jsonb
);
```

For other backends, the `config_json` fields differ — see the typed `Config` struct in each driver:

- [`pkg/artifact-store-adapter/nexus/nexus.go`](../../../pkg/artifact-store-adapter/nexus/nexus.go)
- [`pkg/artifact-store-adapter/artifactory/artifactory.go`](../../../pkg/artifact-store-adapter/artifactory/artifactory.go)
- [`pkg/artifact-store-adapter/ghpackages/ghpackages.go`](../../../pkg/artifact-store-adapter/ghpackages/ghpackages.go)
- [`pkg/artifact-store-adapter/codeartifact/codeartifact.go`](../../../pkg/artifact-store-adapter/codeartifact/codeartifact.go)

## Step 3 — Configure the CI workflow

The reusable `forge-skill-publish` workflow reads these GitHub Actions env vars at the Tenant's repo level (or Org level, scoped per repo):

| Variable | Source | Example |
|---|---|---|
| `FORGE_TENANT_ID` | repo var | `00000000-0000-0000-0000-000000000001` |
| `FORGE_ARTIFACT_BACKEND` | repo var | `nexus` |
| `FORGE_ARTIFACT_SETTINGS` | repo var (JSON) | `{"base_url":"https://nexus.acme.example","repo_prefix":"forge-skills"}` |
| `forge_artifact_credential` | repo secret | `nx-user:nx-token` for Nexus basic auth |

The workflow expects the credential schemes documented in [pkg/artifact-store-adapter/cmd/forge-artifact-store/main.go](../../../pkg/artifact-store-adapter/cmd/forge-artifact-store/main.go) (`env://NAME`).

## Step 4 — Smoke test

Trigger a manual publish:

```bash
gh workflow run forge-skill-publish.yml \
  -f skill_spec=skills/my-skill/skill.yaml \
  -f asset_id=skill:foo \
  -f asset_version=0.1.0
```

Watch the **Upload bundle through the artifact-store adapter** step — the JSON output line contains the resolved `artifact_pointer`. The registry's `lifecycle-hooks/gateway-publish` step records this pointer on `active_surface_json.artifact_pointer`.

## Public-backend rejection

If you accidentally set `FORGE_ARTIFACT_BACKEND=npm-public`, the adapter refuses to construct at all:

```
build adapter: public_backend_disallowed: npm-public is not a permitted backend; configure a private store
```

The registry binding CHECK constraint also refuses this value at the SQL layer (see [db/migrations/registry/0007_active_registry_gateways.sql](../../../db/migrations/registry/0007_active_registry_gateways.sql)).

## Failure modes

| Symptom | Cause | Action |
|---|---|---|
| Workflow fails with `build adapter: …` | Backend config or credential wrong | Re-check Vault path + repo var values |
| Workflow fails with `409 version_immutable` | Re-publishing a version that already exists | Bump the version |
| Workflow fails with `409 digest_mismatch` | Bundle bytes changed between sign and upload | Re-run the pipeline; the cosign step regenerates the bundle deterministically |
| Workflow fails with `403 cross_tenant_read_denied` | The credential is scoped to a different Tenant's container | Issue a per-Tenant credential |
| Adapter `Health()` reports `is_public=true` | Misconfigured backend (e.g. public GitHub repo) | Make the backing container private and re-bind |
