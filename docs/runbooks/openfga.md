# OpenFGA Runbook

This runbook covers the Phase 0 local docker-compose OpenFGA instance and authorization model bootstrap.

## Local Endpoints

| Purpose | URL | Notes |
|---|---|---|
| HTTP API | http://localhost:8088 | Services use this endpoint |
| gRPC API | localhost:8089 | Exposed for future SDK usage |
| Playground | http://localhost:3010/playground | Local debugging UI |
| Health | http://localhost:8088/healthz | Used by bootstrap script |

## Source Files

| File | Purpose |
|---|---|
| `contracts/openfga/authorization-model.json` | Versioned Phase 0 authorization model |
| `contracts/openfga/README.md` | Model overview and relation summary |
| `deploy/compose/scripts/bootstrap-openfga.sh` | Creates the local store and writes the model |
| `deploy/compose/data/openfga.env` | Generated local store/model IDs consumed by services/scripts |

## Bootstrap The Store And Model

Start OpenFGA with compose, then load the model:

```sh
docker compose -f deploy/compose/docker-compose.yaml up -d openfga
deploy/compose/scripts/bootstrap-openfga.sh
```

The script writes:

```sh
OPENFGA_STORE_ID=<store-id>
OPENFGA_AUTHORIZATION_MODEL_ID=<model-id>
OPENFGA_API_URL=http://localhost:8088
```

to `deploy/compose/data/openfga.env`.

## Relation Model Summary

The Phase 0 hierarchy is:

```text
tenant
  -> business_unit
       -> workspace
            -> asset
```

Important computed permissions:

| Object | Permission | Meaning |
|---|---|---|
| `workspace` | `can_view` | Viewer, editor, owner, or inherited BU member |
| `workspace` | `can_edit` | Editor or owner |
| `workspace` | `can_admin` | Owner |
| `asset` | `can_view` | Asset owner or workspace viewer |
| `asset` | `can_edit` | Asset owner or workspace editor |

## Manual API Checks

Create/check tuples with the generated IDs from `deploy/compose/data/openfga.env`.

```sh
. deploy/compose/data/openfga.env

curl -sf -X POST "${OPENFGA_API_URL}/stores/${OPENFGA_STORE_ID}/check" \
  -H "content-type: application/json" \
  -d "{\"authorization_model_id\":\"${OPENFGA_AUTHORIZATION_MODEL_ID}\",\"tuple_key\":{\"user\":\"user:alice\",\"relation\":\"can_view\",\"object\":\"workspace:<workspace-id>\"}}"
```

Expected response shape:

```json
{"allowed":true}
```

## Common Troubleshooting

| Symptom | Check | Fix |
|---|---|---|
| Services allow everything locally | `OPENFGA_STORE_ID` is empty | Run `deploy/compose/scripts/bootstrap-openfga.sh` and load `deploy/compose/data/openfga.env` |
| `openfga check` returns model errors | Authorization model ID | Re-run bootstrap after changing `contracts/openfga/authorization-model.json` |
| Playground is empty | Store/model not selected | Use IDs from `deploy/compose/data/openfga.env` |
| OpenFGA fails to start | Postgres dependency | Check `docker compose -f deploy/compose/docker-compose.yaml logs postgres openfga openfga-migrate` |

## Change Process

1. Edit `contracts/openfga/authorization-model.json`.
2. Re-run `deploy/compose/scripts/bootstrap-openfga.sh` against a local OpenFGA instance.
3. Update affected service authorization checks and smoke tests.
4. Update this runbook when object types, relations, or bootstrap files change.
