# GitHub App Runbook

This runbook covers the planned Forge GitHub App integration. The Phase 0 repository includes the local manifest at `infra/github-app/manifest.json`; creating the real app requires a GitHub org and publicly reachable callback/webhook URLs.

## Manifest

The local manifest requests minimal read permissions for bootstrap use:

| Permission | Access | Purpose |
|---|---|---|
| `metadata` | `read` | List accessible repositories and installation metadata |
| `contents` | `read` | Read repository contents for future onboarding/discovery flows |
| `pull_requests` | `read` | Support future SDLC integration without write access |

Subscribed events:

| Event | Purpose |
|---|---|
| `installation` | Audit install/uninstall lifecycle |
| `installation_repositories` | Track repository access changes |
| `repository` | Track repository metadata updates |

## Private Key Rotation

1. In GitHub, open the Forge GitHub App settings.
2. Generate a new private key.
3. Store the new key in the environment secret manager for the target environment.
4. Deploy the service configuration that references the new secret version.
5. Verify the Control Plane can create a GitHub App JWT and list installations.
6. Remove the old private key from GitHub only after the new key is verified.
7. Record the rotation in the security change log with timestamp, operator, and affected environments.

## Local Development Notes

Phase 0 does not store a real private key. Do not commit `.pem` files, generated app credentials, webhook secrets, or installation tokens.

Control Plane can list repositories for the latest installation recorded on a workspace:

```sh
curl -H "authorization: Bearer ${TOKEN}" \
  http://localhost:8081/v1/workspaces/${WORKSPACE_ID}/github/repositories
```

If `GITHUB_INSTALLATION_TOKEN` is set, Control Plane calls GitHub's `/installation/repositories` API. Without that token it returns a local fixture repository so Phase 0 smoke tests remain offline-friendly. Results are cached in Redis via `REDIS_URL`; use `?refresh=1` to bypass the cache.

Expected future local variables:

```sh
GITHUB_APP_ID=<app-id>
GITHUB_APP_PRIVATE_KEY_FILE=<path-to-local-dev-key.pem>
GITHUB_WEBHOOK_SECRET=<local-dev-secret>
GITHUB_INSTALLATION_TOKEN=<short-lived-dev-token>
GITHUB_REPOSITORIES_FIXTURE='[{"name":"forge-local","full_name":"forge-local/forge-local","private":true}]'
```

## Incident Response

If a private key is exposed:

1. Revoke the exposed key in GitHub immediately.
2. Generate and deploy a replacement key.
3. Rotate webhook secret if exposure may include app configuration.
4. Review audit logs for unexpected installation token creation.
5. Notify Security and affected tenant owners.
