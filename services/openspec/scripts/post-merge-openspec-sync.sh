#!/usr/bin/env sh
set -eu

# Install as .git/hooks/post-merge or call from CI after updating openspec/.
# The service endpoint refreshes the Postgres-ready index from the filesystem
# source of truth without changing the underlying OpenSpec records.

curl -fsS -X POST "${OPENSPEC_URL:-http://localhost:8083}/v1/sync/filesystem" >/dev/null
