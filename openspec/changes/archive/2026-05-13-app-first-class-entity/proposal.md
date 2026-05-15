## Why

Today the Forge platform treats **Workspace → OpenSpec** as a flat relationship: every spec is a peer of the workspace and there is no first-class concept of an "Application" that aggregates the specs, deployments, dashboards, runtimes, design system, and SDLC runs that belong to the same product. This forces non-technical users to reason in terms of OpenSpec IDs, slashes and free-floating artefacts, and blocks the rest of Phase 5 work (Design System Catalog assignment, Alfred Console redesign, end-to-end SDLC) which all need an Application identity to anchor on.

## What Changes

- **BREAKING**: Introduce **App** as a new first-class entity in the hierarchy `Tenant → Workspace → App → Specs[]`. Every OpenSpec from now on belongs to exactly one App; an App belongs to exactly one Workspace.
- Add the `application` aggregate to the platform data model with `id`, `name`, `slug`, `workspace_id`, `tenant_id`, `owners`, `lifecycle_state`, `repo_links[]`, `runtime_links[]`, `design_system_ref`, `default_environments[]`, timestamps and an explicit `description`.
- Expose App CRUD via the Registry/Workspace control plane: `POST/GET/PATCH/DELETE /v1/workspaces/{id}/apps` and `/v1/apps/{id}` (read-by-id across the tenant for permitted callers).
- Re-anchor OpenSpec: every spec record SHALL carry `app_id` (required, foreign-keyed). Re-anchor downstream artefacts that today reference only `workspace_id` — onboarding requests, deployments, runtime registrations, asset publications produced by an App — to carry `app_id` as well.
- Introduce an **"Unassigned" bucket per workspace**: a synthetic, system-managed App named `_unassigned` that exists to receive specs that cannot yet be mapped to a real App during the migration window. The bucket SHALL be visible in the UI as read-only, and SHALL be the only App created automatically by the system.
- **Orphan-spec migration policy (DELETE)**: as part of the rollout migration, OpenSpecs that are not associated with any active deployment, repo or runtime SHALL be **hard-deleted**. Only specs that have a clear App candidate (active deployment, linked repo, named in an onboarding request) SHALL be retained and re-parented to that App. The criteria, dry-run output and final list SHALL be recorded in an immutable audit record before deletion.
- Add OpenFGA relations: `app#owner`, `app#editor`, `app#viewer` rooted under the App, inheriting from the Workspace by default, so that App scope works as the new permission anchor.
- Emit App lifecycle CloudEvents: `app.created.v1`, `app.updated.v1`, `app.archived.v1`, `app.deleted.v1`, plus `spec.reparented.v1` for the migration trail.
- Expose App in the Portal navigation as the canonical anchor (App picker in the global topbar, App detail page with tabs *Specs / Runs / Deployments / Runtimes / Settings*).

## Capabilities

### New Capabilities

- `application-entity`: defines the App aggregate, its lifecycle, ownership rules, identity (slug, id) and the relationship with Workspaces and Specs. Hosts the App CRUD API contract, OpenFGA model fragment and audit/event schema.

### Modified Capabilities

- `workspace-management`: Workspace contents requirement updated — an OpenSpec is no longer associated directly to a Workspace but through its parent App. Workspace contents now include `apps` as a top-level association.
- `intent-capture-wizard`: the wizard SHALL ask for / propose an App scope before persisting a draft, and SHALL never produce a spec without an `app_id`.
- `app-onboarding-service`: onboarding requests SHALL accept an `app_id` (existing App) or `app_proposal` (new App created atomically with the repo). The resulting application asset MUST be linked to that App.
- `ai-asset-registry`: every asset published by an App SHALL carry `app_id` in addition to `workspace_id` for filtering and visibility.
- `openspec-backbone`: OpenSpec records SHALL include `app_id` (NOT NULL). The `intent.committed.v1` event SHALL include `app_id`.
- `alfred-control-plane`: Alfred SHALL receive `app_id` in every intent dialogue context and SHALL refuse to create a spec without an explicit App scope.

## Impact

- **Database**: new `application` table; new `app_id` FK on `openspec`, `onboarding_request`, `deployment`, `runtime_registration`, `asset` (nullable initially, made NOT NULL after migration), plus matching indices.
- **Migration**: dry-run + hard-delete of orphan specs; one-time job to materialise an "Unassigned" App per Workspace and to back-fill `app_id` for specs with a clear App candidate. The migration tool SHALL be runnable offline against a snapshot before production execution.
- **APIs**: new `apps` resource on the Registry/Workspace control plane; OpenSpec and Onboarding APIs gain `app_id`.
- **Portal**: new App detail route, App picker in topbar, App selection step in the Intent Capture Wizard. The existing Workspace screens get an "Apps" tab.
- **OpenFGA**: model fragment for `app` type and relations; migration tool to materialise tuples for existing data.
- **Events**: 4 new `app.*.v1` events + `spec.reparented.v1`; existing `intent.committed.v1` and `app.onboarding.*` payloads gain `app_id`.
- **Downstream changes**: this change is a prerequisite for `design-system-catalog`, `alfred-console-redesign` and `sdlc-end-to-end`. Phase 5 cannot land without it.
