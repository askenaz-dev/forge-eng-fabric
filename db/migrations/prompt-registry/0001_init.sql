-- +goose Up
CREATE TABLE prompt_template (
  id                  text NOT NULL,
  version             text NOT NULL,
  owner_team          text NOT NULL,
  template            text NOT NULL,
  variables_schema    jsonb NOT NULL,
  output_schema       jsonb,
  examples            jsonb NOT NULL,
  recommended_model   text NOT NULL,
  cost_class          text NOT NULL,
  eval_suite          text NOT NULL,
  guardrails          jsonb NOT NULL DEFAULT '{}'::jsonb,
  lifecycle_state     text NOT NULL,
  trust_level         text NOT NULL,
  eval_scores         jsonb NOT NULL DEFAULT '{}'::jsonb,
  change_history      jsonb NOT NULL DEFAULT '[]'::jsonb,
  created_at          timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (id, version)
);

-- +goose Down
DROP TABLE IF EXISTS prompt_template;
