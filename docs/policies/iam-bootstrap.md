# IAM Bootstrap Policy Draft

Status: draft for IAM/Security validation.

This document maps local Keycloak identities to OpenFGA tuples for Phase 0. The production IdP claim mapping must be validated with the corporate IAM team before cloud rollout.

## Identity Sources

| Source | Local value | Notes |
|---|---|---|
| Realm | `forge` | Imported from `deploy/compose/keycloak/forge-realm.json` |
| User subject | Keycloak `sub` claim | Services write stable OpenFGA users as `user:<sub>` |
| Username | Keycloak `preferred_username` claim | Local seed owner inputs may use usernames such as `alice` |
| Realm roles | `platform-admin`, `tenant-admin`, `developer` | Used for bootstrap/admin checks in Control Plane |
| Groups | `forge-admins`, `forge-developers` | Present in tokens for future group mapping |

## Local Seed Mapping

| User | Roles | Initial capability |
|---|---|---|
| `alice` | `platform-admin`, `tenant-admin`, `developer` | Can create tenants, BUs, workspaces and seed owner tuples |
| `bob` | `developer` | Can access only workspaces where OpenFGA grants a relation |

## OpenFGA Tuple Patterns

| Event | Tuple written |
|---|---|
| Tenant created by user `U` | `user:U#admin@tenant:T` |
| Business Unit created under tenant `T` | `tenant:T#tenant@business_unit:B` and `user:U#admin@business_unit:B` |
| Workspace created under BU `B` | `business_unit:B#business_unit@workspace:W` |
| Workspace owner `O` assigned | `user:O#owner@workspace:W`; if `O` is the current user's local username, also write `user:<sub>#owner@workspace:W` |

## Permission Intent

| Object | Relation | Intended use |
|---|---|---|
| `tenant` | `admin` | Tenant-level administration |
| `business_unit` | `admin` | BU management and workspace creation |
| `workspace` | `owner` | Workspace admin, edit, and view |
| `workspace` | `editor` | Workspace edit and view |
| `workspace` | `viewer` | Workspace read-only access |
| `asset` | `owner` | Asset edit and view |

## Production Questions

1. Which corporate IdP group claims map to `platform-admin` and `tenant-admin`?
2. Are tenant admins granted through roles, groups, or OpenFGA tuples only?
3. Are service accounts represented as `user:<sub>` or a separate OpenFGA type?
4. How should just-in-time user provisioning write or revoke tuples?
5. What is the approval workflow for cross-tenant access exceptions?
