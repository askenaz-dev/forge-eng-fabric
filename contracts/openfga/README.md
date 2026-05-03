# OpenFGA Authorization Model — Phase 0

Minimal RBAC + ReBAC model covering the Phase 0 resource hierarchy:

```
tenant
  └── business_unit
        └── workspace
              ├── asset
              ├── repo
              └── environment
                    └── deployment
```

## Types

- **user** — leaf principal (Keycloak `sub`).
- **tenant** — top of the hierarchy. Relations: `admin`, `member`.
- **business_unit** — belongs to a tenant. Relations: `admin`, `member`. Tenant admins inherit BU admin.
- **workspace** — belongs to a BU. Relations: `owner`, `editor`, `viewer`. BU members inherit `viewer`.
- **asset** — belongs to a workspace. Relations: `owner`, plus computed `can_view`/`can_edit` from workspace.
- **repo** — belongs to a workspace. Relations: `owner`, `viewer`, plus computed `can_view`/`can_edit` from workspace.
- **environment** — belongs to a workspace. Relations: `owner`, `viewer`, plus computed `can_view`/`can_deploy` from workspace.
- **deployment** — belongs to an environment. Relations: `owner`, `viewer`, plus computed `can_view`/`can_admin` from environment.

## Computed permissions

- `workspace#can_view` ← viewer ∨ editor ∨ owner ∨ business_unit#member
- `workspace#can_edit` ← editor ∨ owner
- `workspace#can_admin` ← owner
- `asset#can_view` ← owner ∨ workspace#viewer
- `asset#can_edit` ← owner ∨ workspace#editor
- `repo#can_view` ← viewer ∨ owner ∨ workspace#viewer
- `repo#can_edit` ← owner ∨ workspace#editor
- `environment#can_view` ← viewer ∨ owner ∨ workspace#viewer
- `environment#can_deploy` ← owner ∨ workspace#editor
- `deployment#can_view` ← viewer ∨ owner ∨ environment#viewer
- `deployment#can_admin` ← owner ∨ environment#owner

## Out of scope (deferred to Phase 1+)

- Delegated permissions for agents (Alfred) with TTL.
- Trust-tier–scoped action grants (T0–T5).
- Cross-tenant sharing.
- Delegated repo and deployment permissions beyond the Workspace inheritance model.

## Policy tests

`contracts/openfga/tests/phase0-policy.yaml` contains fixtures for workspace isolation and inheritance into assets, repos, environments and deployments.

## Bootstrap

`deploy/compose/scripts/bootstrap-openfga.sh` loads this model into the running OpenFGA container and writes the resulting `store_id` + `authorization_model_id` to `deploy/compose/data/openfga.env`.
