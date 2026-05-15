#!/usr/bin/env bash
# app_entity_not_null_per_workspace.sh — flip app_id NOT NULL for a single
# pilot workspace. Idempotent. Refuses to run if the per-workspace
# `application_backfill_sentinel` row is missing.
#
# Usage:
#   ./scripts/app_entity_not_null_per_workspace.sh <workspace_id>
#
# Environment:
#   DATABASE_URL  Postgres connection string (must have ALTER TABLE rights)
#   DRY_RUN       set to 1 to print SQL without executing it
#
# This is the M5 cutover step described in
# openspec/changes/app-first-class-entity/design.md and matches the per-table
# work emitted by db/migrations/registry/0009_app_id_not_null.sql but scoped
# to one workspace at a time (so we ship the cutover incrementally per pilot).

set -euo pipefail

if [[ $# -lt 1 ]]; then
  echo "usage: $0 <workspace_id>" >&2
  exit 2
fi

WORKSPACE_ID="$1"
PSQL_CMD=(psql "${DATABASE_URL:?DATABASE_URL is required}" -v ON_ERROR_STOP=1 -X -q -t)

run_sql() {
  if [[ "${DRY_RUN:-0}" == "1" ]]; then
    echo "DRY_RUN: $1"
  else
    "${PSQL_CMD[@]}" -c "$1"
  fi
}

# 1. Gate: assert the per-workspace sentinel exists.
SENTINEL=$("${PSQL_CMD[@]}" -c \
  "SELECT 1 FROM application_backfill_sentinel WHERE workspace_id='${WORKSPACE_ID}'" | tr -d '[:space:]')
if [[ "$SENTINEL" != "1" ]]; then
  echo "missing backfill sentinel for workspace ${WORKSPACE_ID}" >&2
  echo "run the migration job (spec-app-migration execute) before this script" >&2
  exit 3
fi

# 2. Assert zero NULL app_id rows in each anchored table for the workspace.
for table_query in \
    "openspec_index|workspace_id::text=" \
    "app_onboarding_request|workspace_id::text=" \
    "runtime|workspace_id::text=" \
    "asset|workspace_id::text="; do
  IFS='|' read -r table predicate <<< "${table_query}"
  NULL_COUNT=$("${PSQL_CMD[@]}" -c \
    "SELECT count(*) FROM ${table} WHERE app_id IS NULL AND ${predicate}'${WORKSPACE_ID}'" | tr -d '[:space:]')
  if [[ "${NULL_COUNT}" != "0" ]]; then
    echo "table=${table} has ${NULL_COUNT} NULL app_id rows for workspace ${WORKSPACE_ID}" >&2
    exit 4
  fi
done

echo "all anchored tables clean for workspace ${WORKSPACE_ID}; flipping NOT NULL"

# 3. Per-workspace NOT NULL is enforced via a workspace-bound CHECK constraint
# rather than a column-level NOT NULL flip, because column-level flips are
# all-or-nothing. The CHECK is global once every workspace has its row;
# `0009_app_id_not_null.sql` is the global flip.
for table in openspec_index app_onboarding_request runtime asset; do
  CONSTRAINT_NAME="${table}_app_id_not_null_per_ws_${WORKSPACE_ID//-/_}"
  run_sql "ALTER TABLE ${table} ADD CONSTRAINT ${CONSTRAINT_NAME} CHECK (NOT (workspace_id::text='${WORKSPACE_ID}' AND app_id IS NULL)) NOT VALID;"
  run_sql "ALTER TABLE ${table} VALIDATE CONSTRAINT ${CONSTRAINT_NAME};"
done

echo "workspace ${WORKSPACE_ID} cutover complete"
