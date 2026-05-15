-- +goose Up
-- Phase 5 — M5 cutover: flip openspec_index.app_id to NOT NULL once the
-- migration job has confirmed there are no NULL rows. The check on
-- application(workspace_id) coherence stays in the application layer because
-- the openspec service does not have direct visibility into the registry's
-- application table.

-- +goose StatementBegin
DO $$
DECLARE
  null_count bigint := 0;
BEGIN
  SELECT count(*) INTO null_count FROM openspec_index WHERE app_id IS NULL;
  IF null_count > 0 THEN
    RAISE EXCEPTION 'openspec_index has % rows with NULL app_id', null_count;
  END IF;
END
$$;
-- +goose StatementEnd

ALTER TABLE openspec_index ALTER COLUMN app_id SET NOT NULL;

-- +goose Down
ALTER TABLE openspec_index ALTER COLUMN app_id DROP NOT NULL;
