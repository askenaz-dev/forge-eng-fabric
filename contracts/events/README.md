# Forge CloudEvents — v1.0

All Forge platform events conform to **CloudEvents 1.0** (https://github.com/cloudevents/spec/blob/v1.0.2/cloudevents/spec.md) with the following Forge extensions in the context attributes:

| Attribute | Type | Required | Description |
|---|---|---|---|
| `forgetenantid` | string (UUID) | yes | Tenant the event belongs to |
| `forgeworkspaceid` | string (UUID) | when applicable | Workspace scope |
| `forgeactor` | string | yes | Actor that produced the event (`user:<sub>`, `service:<name>`, `agent:alfred`) |
| `forgecorrelationid` | string | yes | Correlation id propagated end-to-end |

Standard CloudEvents attributes used:

| Attribute | Convention |
|---|---|
| `id` | UUID v4 |
| `source` | `forge://service/<service-name>` |
| `specversion` | `1.0` |
| `type` | `com.forge.<domain>.<action>.v<n>` (e.g. `com.forge.workspace.created.v1`) |
| `datacontenttype` | `application/json` |
| `time` | RFC3339 |
| `subject` | resource id (e.g. `workspace/<uuid>`) |

Transport in Kafka follows the CloudEvents binary content mode: each context attribute is a Kafka header `ce_<name>` (lowercase), the body is the `data` payload as JSON.

## Catalog (Phase 0 minimum)

| Type | Source | Subject | Data schema |
|---|---|---|---|
| `com.forge.workspace.created.v1` | control-plane | `workspace/<id>` | `events/workspace.created.v1.json` |
| `com.forge.workspace.updated.v1` | control-plane | `workspace/<id>` | `events/workspace.updated.v1.json` |
| `com.forge.workspace.archived.v1` | control-plane | `workspace/<id>` | `events/workspace.archived.v1.json` |
| `com.forge.asset.created.v1` | registry | `asset/<id>@<version>` | `events/asset.created.v1.json` |
| `com.forge.asset.updated.v1` | registry | `asset/<id>@<version>` | `events/asset.updated.v1.json` |
| `com.forge.audit.events.v1` | audit | `audit/<id>` | `events/audit.events.v1.json` |
| `com.forge.auth.failed.v1` | control-plane | `user/<sub>` | `events/auth.failed.v1.json` |
| `com.forge.github.connected.v1` | control-plane | `github_installation/<id>` | `events/github.connected.v1.json` |

Future phases extend the catalog; types are versioned via `vN` suffix and never reused.
