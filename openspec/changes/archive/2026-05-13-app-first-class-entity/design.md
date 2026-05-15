## Context

The Forge engineering fabric organises tenants as `Tenant → Workspace → OpenSpec[..]`. Every artefact produced by the platform — onboarding requests, deployments, runtime registrations, AI assets, dashboards — is anchored at the Workspace level, with OpenSpecs sitting as siblings of those artefacts. This produces three operational pains:

1. **No product identity in the UI**. Users see specs and deployments side-by-side under a Workspace; there is no node to attach a Design System, an environment matrix, a release calendar or a Friendly Mode landing page to.
2. **Specs proliferate without an owner**. Specs created from explorations, prototypes, or one-off Alfred turns survive as orphans because the workspace alone is too coarse to deduplicate them. The Registry shows hundreds of `proposed` specs nobody can claim.
3. **Downstream work is blocked**. Phase 5 needs an App scope for: (a) attaching a Design System template, (b) detecting "is this intent about an existing App or a new one?" via RAG with a high-precision threshold, (c) routing Friendly Mode cards ("Nueva App / Mejorar / Operar") to the right anchor, and (d) running the new end-to-end SDLC workflow `forge.reference.intent-to-infrastructure@1` against a stable target.

Constraints:
- The existing OpenSpec backbone is already in production in pre-prod tenants — backwards-incompatible schema changes need a one-shot migration with a clear orphan-spec policy.
- OpenFGA tuples for `workspace` already exist; we must extend the model without churning existing tuples.
- The portal already ships a Workspace picker; we cannot regress that surface during migration.

Stakeholders: SDLC platform team (owners of OpenSpec backbone and Registry), Portal team (Workspace/Alfred Console surfaces), Workspace admins across pilot tenants who currently rely on the flat model.

## Goals / Non-Goals

**Goals:**
- Introduce **App** as a first-class entity with its own identity, lifecycle, owners and OpenFGA scope.
- Make `app_id` mandatory on every new OpenSpec and on every downstream artefact (onboarding, deployments, runtime registrations, asset publications).
- Provide a deterministic, **audited, hard-delete migration** for specs that cannot be re-parented.
- Establish App as the join point that the next three Phase 5 changes (Design System Catalog, Alfred Console Redesign, SDLC End-to-End) will plug into.
- Keep the public OpenFGA model additive (new `app` type, new relations) with no churn on existing `workspace` tuples.

**Non-Goals:**
- Re-designing the Alfred Console UX. That belongs to the `alfred-console-redesign` change and only depends on App existing.
- Choosing/binding a Design System template. That is the responsibility of `design-system-catalog`.
- Cross-App dependency graphs, App-to-App pipelines, or App templates as a registry asset. Those are explicit Phase 6 candidates.
- Soft-delete or grace-period workflows for orphan specs. The agreed policy is hard delete after audit — no two-phase soft delete here.
- Backfilling `app_id` into archived/retired specs that are no longer queryable from the live workspace surface; they retain their workspace anchor and are excluded from the migration.

## Decisions

### Decision 1 — App is its own aggregate root, not a folder under Workspace

**Choice**: model `application` as a standalone aggregate with its own table, primary key, OpenFGA type, audit stream and events, parented by `workspace_id`.

**Why**: a folder semantics (`workspace.apps: [{...}]`) would have made the entity invisible to OpenFGA (no `app#owner` direct relation), forced every downstream consumer to look up by composite key, and complicated the inevitable "move App across Workspaces" requirement. A standalone aggregate keeps the parent edge as data (`workspace_id`) while letting permissions, events and APIs target App directly.

**Alternatives considered**:
- *Embed App as a JSON column under Workspace*. Rejected — no atomic per-App permissions, and every read of a Workspace pulls all apps.
- *Tag-based "App" derived from a spec label*. Rejected — implicit, not addressable in OpenFGA, and gives no anchor for runtime/deployment links.

### Decision 2 — Slug + UUID identity, slug unique per workspace

**Choice**: App identity is `id` (UUID v7) for stable references and `slug` (kebab-case, unique within its workspace) for URLs. URLs look like `/workspaces/{ws_slug}/apps/{app_slug}`.

**Why**: UUIDs survive renames; slugs give us readable URLs and clean deeplinks for Alfred Friendly Mode cards. Scoping uniqueness to the workspace (not tenant) lets two BUs in the same tenant both have an `hr-portal` slug.

### Decision 3 — `app_id` is NOT NULL on `openspec`, enforced after migration

**Choice**: schema migration runs in three steps: (M1) add `app_id` as nullable on `openspec` and downstream artefacts, (M2) backfill, (M3) flip to `NOT NULL` and add the FK to `application(id)`. The cutover gate for M3 is a clean dry-run of the orphan-deletion job and successful backfill audit.

**Why**: a single-step NOT NULL on a live table is unsafe with the volume in pilot tenants. The three-step approach lets us roll back at any point before M3.

**Alternatives considered**: shadow table + dual writes (overkill for this volume), allow `app_id` to stay nullable forever (rejected — defeats the purpose).

### Decision 4 — Orphan-spec policy: hard delete with full audit, no soft delete

**Choice**: a spec is considered an orphan iff ALL of the following are true at migration cutover:
- No active deployment references it.
- No live repo onboarding request mentions it.
- No runtime registration carries its id.
- It is not referenced by a Workspace-pinned set, dashboard, or Alfred conversation captured in the last 90 days.
- It is in lifecycle state `proposed` or `draft` (already `approved` or `committed` specs are never orphans).

Orphans are **hard-deleted** after a dry-run, an explicit per-workspace owner confirmation, and the emission of `spec.purged.v1` with the full record contents stored under the immutable audit retention bucket (so the trail survives the deletion).

**Why**: the user-supplied policy is explicit (`borrado`). Soft delete would leave the orphans cluttering the registry indefinitely and would force us to ship a deletion follow-up. The audit copy under the retention bucket gives us forensic recovery without polluting live state.

**Risk path**: a workspace owner refuses to confirm. In that case the orphans are re-parented to the synthetic `_unassigned` App for that workspace and a follow-up Linear ticket is opened — they are *never* silently deleted.

### Decision 5 — `_unassigned` App is system-managed and read-only

**Choice**: every workspace gets exactly one `_unassigned` App, created by the migration job and thereafter by the workspace-bootstrap pipeline. It is the only App the system creates on a user's behalf. The Portal SHALL render it as read-only; specs inside it can be re-parented out, but new specs cannot be added to it by API consumers.

**Why**: keeps the schema invariant (`app_id NOT NULL`) intact for any spec that arrives without an explicit App context (e.g., a legacy integration not yet updated). Users see clearly that those specs need a real home.

### Decision 6 — OpenFGA model: inherit-from-workspace by default

**Choice**: add `app` as a new OpenFGA type with relations `owner`, `editor`, `viewer`. Default `app#viewer` is computed from `workspace#viewer`, `app#editor` from `workspace#editor`, `app#owner` is explicit. App-level overrides (e.g., a contractor with editor on a single App but no workspace access) are allowed via direct tuples on the App.

**Why**: keeps existing workspace tuples authoritative, lets us layer App-level overrides without churn, and matches the operating model in `delegated-permissions`.

### Decision 7 — App selection sits inside the Intent Capture Wizard as the FIRST step

**Choice**: the wizard SHALL begin with an App scope question. Three branches: (a) "extend an existing App" (lists App for the workspace; can preview the App's spec history), (b) "create a new App", (c) "I don't know yet" (parks the draft against `_unassigned` and surfaces a banner asking to pick before commit). Commit SHALL refuse if the App scope is still `_unassigned`.

**Why**: this is the cleanest place to enforce the App invariant without breaking Alfred's natural-language UX. It also gives us the hook point for the RAG-based spec deduplication that `alfred-console-redesign` builds on.

## Risks / Trade-offs

- **[Risk] Hard delete of orphans is irreversible at the live-DB level**. → Mitigation: dry-run report shipped to each workspace owner ≥ 5 business days before the destructive run; full record contents copied to the audit retention bucket with 7-year retention; per-workspace owner confirmation required; runbook for restore-from-audit if a wrongful deletion is identified within 30 days.
- **[Risk] M3 cutover (NOT NULL flip) fails on a workspace with a slow backfill**. → Mitigation: backfill is idempotent and runs per-workspace; the M3 flip is gated by a per-workspace "ready" flag that the backfill sets only after asserting 0 NULL rows for that workspace.
- **[Risk] OpenFGA tuple explosion when materialising App relations**. → Mitigation: rely on computed relations from Workspace instead of materialised tuples; only explicit overrides create direct tuples on App. Expected per-tenant overhead is small (<5% above the current workspace tuple volume).
- **[Risk] Portal regressions for users in mid-flight wizard drafts during the rollout**. → Mitigation: feature-flag the wizard's App step (`forge.app_scope.enabled`); turn on per-workspace only after that workspace's backfill is complete; preserve the legacy slash-command Alfred surface (with implicit `_unassigned` association) for the duration of the rollout.
- **[Trade-off] Slug-uniqueness is workspace-scoped, not tenant-scoped**. → Acceptable: it matches the operating model and makes the URL `/workspaces/{ws}/apps/{app}` unambiguous; cross-workspace deeplinks SHALL use the UUID form `/apps/{id}`.
- **[Trade-off] No App template or App-to-App graph in this change**. → Acceptable for Phase 5; explicitly deferred to Phase 6 once Design System and SDLC pieces have stabilised on the App anchor.

## Migration Plan

1. **M0 — Ship code, dark-launched**: `application` table created; APIs deployed behind `forge.app_entity.enabled=false`; OpenFGA model updated additively; downstream tables gain nullable `app_id`.
2. **M1 — Per-workspace dry-run**: for each pilot workspace, run the orphan-detection job in read-only mode and produce a CSV report (`spec_id, last_activity, classification`) for the workspace owner. No deletes yet.
3. **M2 — Owner confirmation window**: workspace owner signs off on the report; remaining specs are either tagged for re-parenting to a named App or accepted as orphans.
4. **M3 — Materialise Apps and backfill**: create the `_unassigned` App and any user-named Apps; backfill `app_id` for every retained spec; emit `spec.reparented.v1` for each.
5. **M4 — Hard delete orphans**: under workspace-owner-recorded approval, hard delete confirmed orphans; copy full records to audit retention; emit `spec.purged.v1`.
6. **M5 — Flip NOT NULL and enable feature flags**: with backfill clean and orphans removed, flip `openspec.app_id` to `NOT NULL`; enable `forge.app_entity.enabled=true` for the workspace; enable wizard App step.
7. **M6 — Repeat per workspace** until all pilot tenants are migrated, then turn the flag on globally.

**Rollback strategy**: prior to M5, rollback is `flag off + drop app_id column`. After M5, rollback requires (a) re-introducing nullability, (b) restoring purged orphans from the audit retention bucket (manual, target ≤ 4h per workspace).

## Open Questions

- Naming policy for App slugs at creation (e.g., reserve a list of system slugs beyond `_unassigned`?). Default: reserve any slug starting with `_`.
- Does an App carry its own `default_environments` list separate from the Workspace's environment matrix? Default: yes, optional, inherits from Workspace when absent. Confirmation needed from SRE during M1.
- Cross-tenant App lookups: should `/apps/{id}` resolve across tenants for platform-admin principals? Default: no — admin lookups go through a dedicated console.
