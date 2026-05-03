-- +goose Up
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE openspec_index (
  openspec_id       text PRIMARY KEY,
  workspace_id      uuid NOT NULL,
  title             text NOT NULL,
  business_intent   text NOT NULL,
  version           integer NOT NULL,
  path              text NOT NULL,
  updated_at        timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE openspec_event_outbox (
  id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  event_type     text NOT NULL,
  subject        text NOT NULL,
  payload        jsonb NOT NULL,
  created_at     timestamptz NOT NULL DEFAULT now(),
  published_at   timestamptz
);

CREATE INDEX openspec_index_workspace_idx ON openspec_index(workspace_id);
CREATE INDEX openspec_event_outbox_unpublished_idx ON openspec_event_outbox(created_at) WHERE published_at IS NULL;

-- +goose Down
DROP TABLE IF EXISTS openspec_event_outbox;
DROP TABLE IF EXISTS openspec_index;
