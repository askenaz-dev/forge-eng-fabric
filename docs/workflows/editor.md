# Visual editor

The Portal "Workflows" module is the visual surface over the canonical
[workflow AST](dsl.md). It lives at `/workflows` and consumes the workflow
registry (`WORKFLOW_REGISTRY_URL`) and runtime (`WORKFLOW_RUNTIME_URL`) at
runtime.

## What the editor gives you

- **Sidebar** lists workflows scoped to a Tenant + Workspace. New workflows
  can be created from the sidebar form.
- **DSL pane** is the source of truth. Edit YAML directly; the canonical AST
  is what gets persisted on save. Import/export YAML files via the buttons
  next to the publish action.
- **Graph preview** renders steps + dependencies live as you edit. The full
  React Flow diagram lights up when the `reactflow` package is installed in
  the portal — the lightweight preview always works.
- **Lint feedback** reports errors inline (duplicate ids, floating
  references, dangling deps). Server-side validation is authoritative; this
  surface is just for fast feedback.
- **Dry-run** posts to the runtime with `dry_run=true`, mocking I/O so no
  real Skills or MCPs are called.
- **Diff viewer** compares any two versions and renders the bump
  classification (major/minor/patch) returned by the registry.
- **Catalog** — the sidebar consumes the asset registry to suggest Skills,
  MCPs and Prompts at the right pin-version. Refs to non-approved assets are
  rejected at publish time.

## Publish flow

1. Edit YAML; lint should be clean.
2. Click `Publish version`. The request goes to
   `POST /v1/workflows/{id}/versions` (workflow-registry).
3. The registry runs schema + lint, then classifies the diff:
   - Removed/required input → MAJOR
   - Removed step or step type changed → MAJOR
   - Added optional input/output, new step → MINOR
   - Description/owners only → PATCH
4. Either provide a version that satisfies the classification, or check
   `Auto-bump version`.
5. Forge-certified status additionally requires a passing eval run and a
   recorded security review (see [Marketplace](../marketplace/index.md)).

## E2E tests

The Playwright suite at `portal/tests/e2e/workflows.spec.ts` validates the
editor module renders. Add cases there as the editor grows; the existing
patterns in `initiatives.spec.ts` are good references.
