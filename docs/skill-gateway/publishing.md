# Skill gateway — publishing a skill

How an SDLC team gets a skill from `proposed` to **gateway-published**.

## Prereqs

- The asset is registered (POST `/v1/workspaces/{ws}/assets`).
- The asset is at `lifecycle_state = approved` and `trust_level >= T1`. T0 cannot be gateway-published.
- For `skill` and `agent` types: a packaged bundle exists in the object store and is signed.
- For `mcp` type: the MCP declares a `remote_transport` block (HTTP / SSE).

## Step-by-step

### 1. Package the skill

Use the reusable GitHub Actions workflow:

```yaml
jobs:
  publish:
    uses: forge-eng-fabric/.github/.github/workflows/skill-publish.yml@main
    with:
      skill_spec: ./skills/generate-test-cases.json
      asset_id: skill:ws-acme-eng:generate-test-cases
      asset_version: 1.2.0
      channel: stable
    secrets:
      forge_publish_token: ${{ secrets.FORGE_PUBLISH_TOKEN }}
```

Locally:

```bash
make package-skill SPEC=./skills/foo.json OUT=./out/foo.tar.zst
cat ./out/foo.tar.zst.digest    # sha256: ...
```

The packager normalises mtimes, UID/GID, header ordering and zstd parameters so the digest is byte-stable across machines.

### 2. Sign + attest

`cosign sign-blob --yes` produces the signature; `actions/attest-build-provenance` produces the in-toto attestation. Both reference the same digest.

### 3. Upload to the object store

```bash
aws s3 cp out/foo.tar.zst s3://forge-packages/<asset_id>/<version>.tar.zst
```

### 4. Call the registry's gateway-publish hook

```bash
curl --fail-with-body \
  -H "Authorization: Bearer $FORGE_PUBLISH_TOKEN" \
  -H "content-type: application/json" \
  https://registry.forge.internal/v1/assets/<asset_id>/versions/1.2.0/lifecycle-hooks/gateway-publish \
  -d '{
    "channel": "stable",
    "package_digest": "sha256:…",
    "signature_id": "<sig hash>",
    "attestation_id": "<attestation id>",
    "bytes_uri": "s3://forge-packages/<asset_id>/1.2.0.tar.zst",
    "size_bytes": 102400
  }'
```

The registry verifies the asset is `approved + T1+`, writes the `asset_package` row, flips `distribution.gateway_published=true`, and emits `com.forge.asset.gateway_published.v1`.

### 5. Confirm it is installable

```bash
forge skills list
forge skills install <name>
```

## Rotating a version

You cannot republish the same `<asset_id, version>` with different content — the registry returns `409 package_already_published`. Bump SemVer:

```bash
make package-skill SPEC=./skills/foo.json OUT=./out/foo-1.2.1.tar.zst
# … sign + upload + gateway-publish for version 1.2.1
```

Existing installs of 1.2.0 keep working; `forge skills update` upgrades them.

## Unpublishing

Any transition to `deprecated` or `retired` automatically flips `gateway_published=false` and emits `com.forge.asset.gateway_unpublished.v1`. The CLI shows the asset in the deprecated state on the next `list`.

## MCP servers

MCPs are gateway-published the same way as skills, except instead of `package_digest` they declare a `remote_transport`:

```json
{
  "channel": "stable",
  "remote_transport": {
    "http": { "path_template": "/v1/invoke", "upstream_url": "http://github-mcp.forge.internal/v1/invoke" },
    "auth_modes": ["pat", "oidc_bearer"],
    "health_path": "/healthz"
  }
}
```

Stdio-only MCPs are refused with `409 remote_transport_required` — they remain valid inside the platform runtime but cannot be installable through the gateway.
