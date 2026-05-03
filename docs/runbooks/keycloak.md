# Keycloak Runbook

This runbook covers the Phase 0 local docker-compose Keycloak instance.

## Local Endpoints

| Purpose | URL | Notes |
|---|---|---|
| Admin console | http://localhost:8080 | Admin user: `admin`, password: `admin` |
| Forge realm | http://localhost:8080/realms/forge | Imported from `deploy/compose/keycloak/forge-realm.json` |
| OIDC discovery | http://localhost:8080/realms/forge/.well-known/openid-configuration | Used by services and portal |
| JWKS | http://localhost:8080/realms/forge/protocol/openid-connect/certs | Used by Go services for JWT validation |

## Seed Users

| Username | Password | Realm roles | Intended use |
|---|---|---|---|
| `alice` | `alice` | `platform-admin`, `tenant-admin`, `developer` | Smoke test and admin flows |
| `bob` | `bob` | `developer` | Negative/least-privilege checks |

These accounts are local-only fixtures and must not be reused outside development.

## Clients

| Client | Type | Purpose |
|---|---|---|
| `forge-portal` | Public OIDC client | Browser login for the Next.js portal |
| `forge-control-plane` | Bearer-only resource server | Audience expected by Go APIs |
| `forge-cli` | Public OIDC client with direct grant | Local smoke script token acquisition |

## Start And Health Check

```sh
docker compose -f deploy/compose/docker-compose.yaml up -d keycloak
curl -sf http://localhost:8080/health/ready
```

If the ready endpoint does not return healthy, inspect logs:

```sh
docker compose -f deploy/compose/docker-compose.yaml logs keycloak
```

## Get A Local Access Token

Use the direct grant client for smoke-test style API calls:

```sh
curl -sf -X POST http://localhost:8080/realms/forge/protocol/openid-connect/token \
  -H "content-type: application/x-www-form-urlencoded" \
  -d "grant_type=password" \
  -d "client_id=forge-cli" \
  -d "username=alice" \
  -d "password=alice"
```

The response includes `access_token`. Pass it to Forge APIs as `Authorization: Bearer <token>`.

## Common Troubleshooting

| Symptom | Check | Fix |
|---|---|---|
| API returns `bad issuer` | Token `iss` claim | Use `http://localhost:8080/realms/forge` as issuer |
| API returns `bad audience` | Token `aud` claim | Ensure `forge-portal` audience mapper includes `forge-control-plane`; use `forge-cli` for local scripts |
| Portal login redirects fail | Client redirect URI | Verify portal runs on `http://localhost:3000` |
| Seed users missing | Realm import status | Wipe the Keycloak volume and restart only for local reset: `docker compose -f deploy/compose/docker-compose.yaml down -v` |

## Change Process

1. Edit `deploy/compose/keycloak/forge-realm.json`.
2. Restart a fresh local stack if testing import behavior; Keycloak imports realms on first startup for an empty data volume.
3. Keep all seed credentials development-only.
4. Update this runbook if clients, ports, roles, or seed users change.
