-- +goose Up
ALTER TABLE tenant ADD COLUMN feature_flags JSONB NOT NULL DEFAULT '{}'::jsonb;

-- +goose Down
ALTER TABLE tenant DROP COLUMN feature_flags;
