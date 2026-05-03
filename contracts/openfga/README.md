# OpenFGA Authorization Model — Phase 0

Minimal RBAC + ReBAC model covering the Phase 0 resource hierarchy:

```
tenant
  └── business_unit
        └── workspace
              └── asset
```

## Types

- **user** — leaf principal (Keycloak `sub`).
- **tenant** — top of the hierarchy. Relations: `admin`, `member`.
- **business_unit** — belongs to a tenant. Relations: `admin`, `member`. Tenant admins inherit BU admin.
- **workspace** — belongs to a BU. Relations: `owner`, `editor`, `viewer`. BU members inherit `viewer`.
- **asset** — belongs to a workspace. Relations: `owner`, plus computed `can_view`/`can_edit` from workspace.

## Computed permissions

- `workspace#can_view` ← viewer ∨ editor ∨ owner ∨ business_unit#member
- `workspace#can_edit` ← editor ∨ owner
- `workspace#can_admin` ← owner
- `asset#can_view` ← owner ∨ workspace#viewer
- `asset#can_edit` ← owner ∨ workspace#editor

## Out of scope (deferred to Phase 1+)

- Delegated permissions for agents (Alfred) with TTL.
- Trust-tier–scoped action grants (T0–T5).
- Cross-tenant sharing.
- Repo and skill resource types.

## Bootstrap

`deploy/compose/scripts/bootstrap-openfga.sh` (TODO) loads this model into the running OpenFGA container and writes the resulting `store_id` + `authorization_model_id` to `deploy/compose/data/openfga.env`.
