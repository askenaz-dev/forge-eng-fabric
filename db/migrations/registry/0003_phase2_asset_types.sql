-- +goose Up
ALTER TABLE asset DROP CONSTRAINT IF EXISTS asset_type_check;
ALTER TABLE asset ADD CONSTRAINT asset_type_check
  CHECK (type IN ('mcp','skill','agent','workflow','prompt_template','application','repo_template'));

-- +goose Down
ALTER TABLE asset DROP CONSTRAINT IF EXISTS asset_type_check;
ALTER TABLE asset ADD CONSTRAINT asset_type_check
  CHECK (type IN ('mcp','skill','agent','workflow','prompt_template'));
