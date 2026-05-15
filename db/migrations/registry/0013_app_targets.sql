-- +goose Up
-- sdlc-end-to-end: add per-App SDLC targets map.
-- Each App carries a `targets` JSONB map declaring which SDLC phases are
-- required / optional / opt-in / skipped. See openspec/changes/sdlc-end-to-end
-- specs/application-entity for the full rationale and allowed values.

ALTER TABLE application
  ADD COLUMN IF NOT EXISTS targets jsonb NOT NULL DEFAULT '{
    "architect":    "required",
    "design":       "optional",
    "development":  "required",
    "qa":           "required",
    "security":     "required",
    "devops":       "required",
    "iac":          "opt-in",
    "sre":          "optional",
    "finops":       "opt-in",
    "observability":"opt-in"
  }'::jsonb;

-- Validate allowed values. Phase keys are fixed; values must be in the
-- permitted set. We do not enforce the exact key list here — that is the
-- service layer's job — so that forward-compatible additions are possible.
ALTER TABLE application
  ADD CONSTRAINT application_targets_valid_values CHECK (
    (
      targets->>'architect'    IS NULL OR targets->>'architect'    IN ('required','optional','opt-in','skipped')
    ) AND (
      targets->>'design'       IS NULL OR targets->>'design'       IN ('required','optional','opt-in','skipped')
    ) AND (
      targets->>'development'  IS NULL OR targets->>'development'  IN ('required','optional','opt-in','skipped')
    ) AND (
      targets->>'qa'           IS NULL OR targets->>'qa'           IN ('required','optional','opt-in','skipped')
    ) AND (
      targets->>'security'     IS NULL OR targets->>'security'     IN ('required','optional','opt-in','skipped')
    ) AND (
      targets->>'devops'       IS NULL OR targets->>'devops'       IN ('required','optional','opt-in','skipped')
    ) AND (
      targets->>'iac'          IS NULL OR targets->>'iac'          IN ('required','optional','opt-in','skipped')
    ) AND (
      targets->>'sre'          IS NULL OR targets->>'sre'          IN ('required','optional','opt-in','skipped')
    ) AND (
      targets->>'finops'       IS NULL OR targets->>'finops'       IN ('required','optional','opt-in','skipped')
    ) AND (
      targets->>'observability' IS NULL OR targets->>'observability' IN ('required','optional','opt-in','skipped')
    )
  );

-- +goose Down
ALTER TABLE application
  DROP CONSTRAINT IF EXISTS application_targets_valid_values;
ALTER TABLE application
  DROP COLUMN IF EXISTS targets;
