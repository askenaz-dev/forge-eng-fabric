-- +goose Up
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE finops_budget (
  id                   uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id          text NOT NULL,
  initiative_openspec   text NOT NULL,
  monthly_limit_usd     numeric(14, 4) NOT NULL,
  thresholds            integer[] NOT NULL DEFAULT ARRAY[50,80,100],
  consumed_usd          numeric(14, 4) NOT NULL DEFAULT 0,
  emitted_thresholds    integer[] NOT NULL DEFAULT ARRAY[]::integer[],
  created_at            timestamptz NOT NULL DEFAULT now(),
  updated_at            timestamptz NOT NULL DEFAULT now(),
  UNIQUE (workspace_id, initiative_openspec)
);

CREATE TABLE finops_cost_record (
  id                   uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id          text NOT NULL,
  initiative_openspec   text NOT NULL,
  env                  text NOT NULL,
  asset                text NOT NULL,
  category             text NOT NULL,
  source               text NOT NULL,
  cost_usd             numeric(14, 4) NOT NULL,
  metadata             jsonb NOT NULL DEFAULT '{}'::jsonb,
  observed_at          timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE finops_event_outbox (
  id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  event_type     text NOT NULL,
  workspace_id   text NOT NULL,
  payload        jsonb NOT NULL,
  created_at     timestamptz NOT NULL DEFAULT now(),
  published_at   timestamptz
);

CREATE INDEX finops_cost_record_initiative_idx ON finops_cost_record(workspace_id, initiative_openspec);
CREATE INDEX finops_event_outbox_unpublished_idx ON finops_event_outbox(created_at) WHERE published_at IS NULL;

-- +goose Down
DROP TABLE IF EXISTS finops_event_outbox;
DROP TABLE IF EXISTS finops_cost_record;
DROP TABLE IF EXISTS finops_budget;
