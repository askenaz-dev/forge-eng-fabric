# Workflow marketplace

The marketplace is a Tenant-scoped catalog for sharing workflows across
Workspaces. It complements the workflow registry: the registry is the
source of truth for AST and versions; the marketplace is the
discoverability and install surface.

Service: [`services/marketplace/`](../../services/marketplace).

## Visibility tiers

| Tier | Who sees it | Who installs it |
|---|---|---|
| `private` | Only the author's user | Author only |
| `workspace` | All members of the source Workspace | Same Workspace |
| `tenant` | All Workspaces in the Tenant after approval | Any Workspace in the Tenant |
| `forge-certified` | All Workspaces in the Tenant after eval+security+approval | Any Workspace in the Tenant |

Promotion to `tenant` opens an Approvals Inbox entry for `tenant-admin`.
Promotion to `forge-certified` requires a passing eval run AND a recorded
security review at submission time, plus `forge-certifier` approval. The
[`policy-engine`](../../services/policy-engine) ships these as named
templates: `require-eval-pass`, `require-security-review`,
`require-tenant-share-approval`, `forge-certification-prerequisites`.

## API

```
POST /v1/marketplace                       # publish/promote a listing
GET  /v1/marketplace                       # browse with filters
GET  /v1/marketplace/{id}                  # listing detail
POST /v1/marketplace/{id}/approve          # approve/reject pending listing
POST /v1/marketplace/install               # install into a Workspace (pinned)
GET  /v1/installs                          # list installs
```

`POST /v1/marketplace/install` always pins to the exact `workflow_id@version`
from the listing and emits `workflow.installed_to_workspace.v1`. Existing
installs are not mutated when a newer version is published; the marketplace
UI shows an upgrade prompt instead.

## UI

`/marketplace` in the Portal lists discoverable workflows for the active
Tenant. The install form scopes installs to a target Workspace and emits
the install event. Filters: visibility, criticality, tags, free-text.
