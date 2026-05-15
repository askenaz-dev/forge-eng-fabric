-- +goose Up
-- design-system-catalog: seed the four built-in templates and the
-- ds-forge-default alias. Templates ship `built_in_template=true` so the
-- sanity validator is bypassed at promotion; their manifests are
-- pre-validated and pinned at platform-owned URLs. Eval scores in this seed
-- match design/systems/eval-scores.json (September 2026 baseline).
--
-- The platform tenant uuid below is the constant used by other seeds; the
-- workspace uuid is derived from the platform tenant per the platform-team
-- bootstrap. If your local DB seeds these elsewhere, point the IDs there.

-- ──────────────────────────────────────────────────────────────────────
-- Constants (kept literal so the migration is portable across DBs that do
-- not allow DO-blocks or temp tables in goose +goose Up).
-- ──────────────────────────────────────────────────────────────────────

INSERT INTO asset (
  id, version, type, name, description, owner_team,
  inputs_schema, outputs_schema,
  workspace_id, tenant_id, visibility, lifecycle_state, trust_level,
  eval_scores, owners, metadata, created_by,
  design_system_manifest, built_in_template
) VALUES (
  'design_system:00000000-0000-0000-0000-000000000001:desing-system-1',
  '1.0.0', 'design_system',
  'desing-system-1', 'Forge default — warm ember palette, Instrument Serif display, Geist body.',
  'forge-platform-design',
  '{}'::jsonb, '{}'::jsonb,
  '00000000-0000-0000-0000-000000000001'::uuid,
  '00000000-0000-0000-0000-000000000001'::uuid,
  'tenant', 'approved', 'T3',
  '{"accessibility":0.93,"brand_fidelity":0.95,"quality":1.0,"safety":1.0,"cost":1.0,"latency":1.0}'::jsonb,
  ARRAY['forge-platform-design'],
  '{"catalog_position":1,"look":"forge_default"}'::jsonb,
  'system:forge-platform',
  '{"tokens":"https://platform.forge.example/design-systems/desing-system-1/1.0.0/tokens.css","tokens_sha256":"b812010d5af20a4f53c1efc4f37b79388698001eab1773c58a8e1a6b4c28ed09","components":"https://platform.forge.example/design-systems/desing-system-1/1.0.0/components.tar.gz","components_sha256":"0000000000000000000000000000000000000000000000000000000000000002","fonts":[{"family":"Instrument Serif","weights":[400],"italic":false,"source":"https://platform.forge.example/fonts/instrument-serif-400.woff2"},{"family":"Geist","weights":[400,500,600],"italic":false,"source":"https://platform.forge.example/fonts/geist.woff2"},{"family":"JetBrains Mono","weights":[400],"italic":false,"source":"https://platform.forge.example/fonts/jetbrains-mono-400.woff2"}],"screenshots":{"light":"https://platform.forge.example/design-systems/desing-system-1/1.0.0/screenshot-light.png","light_sha256":"0000000000000000000000000000000000000000000000000000000000000003","dark":"https://platform.forge.example/design-systems/desing-system-1/1.0.0/screenshot-dark.png","dark_sha256":"0000000000000000000000000000000000000000000000000000000000000004"},"use_case":"Forge default. The platform brand — warm ember palette, serif display, generous whitespace. Best for internal admin surfaces and developer tooling."}'::jsonb,
  true
) ON CONFLICT (id, version) DO NOTHING;

INSERT INTO asset (
  id, version, type, name, description, owner_team,
  inputs_schema, outputs_schema,
  workspace_id, tenant_id, visibility, lifecycle_state, trust_level,
  eval_scores, owners, metadata, created_by,
  design_system_manifest, built_in_template
) VALUES (
  'design_system:00000000-0000-0000-0000-000000000001:desing-system-2',
  '1.0.0', 'design_system',
  'desing-system-2', 'Corporate — navy palette, Inter throughout, tight radii.',
  'forge-platform-design',
  '{}'::jsonb, '{}'::jsonb,
  '00000000-0000-0000-0000-000000000001'::uuid,
  '00000000-0000-0000-0000-000000000001'::uuid,
  'tenant', 'approved', 'T3',
  '{"accessibility":0.94,"brand_fidelity":0.86,"quality":1.0,"safety":1.0,"cost":1.0,"latency":1.0}'::jsonb,
  ARRAY['forge-platform-design'],
  '{"catalog_position":2,"look":"corporate"}'::jsonb,
  'system:forge-platform',
  '{"tokens":"https://platform.forge.example/design-systems/desing-system-2/1.0.0/tokens.css","tokens_sha256":"c1d5f2e49b2f919fffc9dc21d41408bc11338b3534f771d1804efab33ca9b2a4","components":"https://platform.forge.example/design-systems/desing-system-2/1.0.0/components.tar.gz","components_sha256":"0000000000000000000000000000000000000000000000000000000000000012","fonts":[{"family":"Inter","weights":[400,500,600,700],"italic":false,"source":"https://platform.forge.example/fonts/inter.woff2"},{"family":"JetBrains Mono","weights":[400],"italic":false,"source":"https://platform.forge.example/fonts/jetbrains-mono-400.woff2"}],"screenshots":{"light":"https://platform.forge.example/design-systems/desing-system-2/1.0.0/screenshot-light.png","light_sha256":"0000000000000000000000000000000000000000000000000000000000000013","dark":"https://platform.forge.example/design-systems/desing-system-2/1.0.0/screenshot-dark.png","dark_sha256":"0000000000000000000000000000000000000000000000000000000000000014"},"use_case":"Corporate. Navy/grey palette, Inter throughout, tight radii. Built for B2B internal tools that need to feel measured, professional and unflashy."}'::jsonb,
  true
) ON CONFLICT (id, version) DO NOTHING;

INSERT INTO asset (
  id, version, type, name, description, owner_team,
  inputs_schema, outputs_schema,
  workspace_id, tenant_id, visibility, lifecycle_state, trust_level,
  eval_scores, owners, metadata, created_by,
  design_system_manifest, built_in_template
) VALUES (
  'design_system:00000000-0000-0000-0000-000000000001:desing-system-3',
  '1.0.0', 'design_system',
  'desing-system-3', 'Minimal — near-monochrome, no display serif, zero ornament.',
  'forge-platform-design',
  '{}'::jsonb, '{}'::jsonb,
  '00000000-0000-0000-0000-000000000001'::uuid,
  '00000000-0000-0000-0000-000000000001'::uuid,
  'tenant', 'approved', 'T3',
  '{"accessibility":0.97,"brand_fidelity":0.82,"quality":1.0,"safety":1.0,"cost":1.0,"latency":1.0}'::jsonb,
  ARRAY['forge-platform-design'],
  '{"catalog_position":3,"look":"minimal"}'::jsonb,
  'system:forge-platform',
  '{"tokens":"https://platform.forge.example/design-systems/desing-system-3/1.0.0/tokens.css","tokens_sha256":"141a82b534d705bd2b48d1bbf4a20e304c65840752f2a8988dad712b061e1d42","components":"https://platform.forge.example/design-systems/desing-system-3/1.0.0/components.tar.gz","components_sha256":"0000000000000000000000000000000000000000000000000000000000000022","fonts":[{"family":"Inter","weights":[400,500,600],"italic":false,"source":"https://platform.forge.example/fonts/inter.woff2"},{"family":"IBM Plex Mono","weights":[400],"italic":false,"source":"https://platform.forge.example/fonts/ibm-plex-mono-400.woff2"}],"screenshots":{"light":"https://platform.forge.example/design-systems/desing-system-3/1.0.0/screenshot-light.png","light_sha256":"0000000000000000000000000000000000000000000000000000000000000023","dark":"https://platform.forge.example/design-systems/desing-system-3/1.0.0/screenshot-dark.png","dark_sha256":"0000000000000000000000000000000000000000000000000000000000000024"},"use_case":"Minimal. Near-monochrome, no display serif, zero ornament. Picks the App content over its chrome. Ideal for content-heavy or editorial surfaces."}'::jsonb,
  true
) ON CONFLICT (id, version) DO NOTHING;

INSERT INTO asset (
  id, version, type, name, description, owner_team,
  inputs_schema, outputs_schema,
  workspace_id, tenant_id, visibility, lifecycle_state, trust_level,
  eval_scores, owners, metadata, created_by,
  design_system_manifest, built_in_template
) VALUES (
  'design_system:00000000-0000-0000-0000-000000000001:desing-system-4',
  '1.0.0', 'design_system',
  'desing-system-4', 'Marketing — Fraunces display serif, hot-pink accent, generous radii.',
  'forge-platform-design',
  '{}'::jsonb, '{}'::jsonb,
  '00000000-0000-0000-0000-000000000001'::uuid,
  '00000000-0000-0000-0000-000000000001'::uuid,
  'tenant', 'approved', 'T3',
  '{"accessibility":0.91,"brand_fidelity":0.94,"quality":1.0,"safety":1.0,"cost":1.0,"latency":1.0}'::jsonb,
  ARRAY['forge-platform-design'],
  '{"catalog_position":4,"look":"marketing"}'::jsonb,
  'system:forge-platform',
  '{"tokens":"https://platform.forge.example/design-systems/desing-system-4/1.0.0/tokens.css","tokens_sha256":"adba7f804b949e7533b8149709602a9579c8d0543654d8b7b8973818f5fe7f49","components":"https://platform.forge.example/design-systems/desing-system-4/1.0.0/components.tar.gz","components_sha256":"0000000000000000000000000000000000000000000000000000000000000032","fonts":[{"family":"Fraunces","weights":[400,700],"italic":true,"source":"https://platform.forge.example/fonts/fraunces.woff2"},{"family":"Inter","weights":[400,500,600],"italic":false,"source":"https://platform.forge.example/fonts/inter.woff2"},{"family":"JetBrains Mono","weights":[400],"italic":false,"source":"https://platform.forge.example/fonts/jetbrains-mono-400.woff2"}],"screenshots":{"light":"https://platform.forge.example/design-systems/desing-system-4/1.0.0/screenshot-light.png","light_sha256":"0000000000000000000000000000000000000000000000000000000000000033","dark":"https://platform.forge.example/design-systems/desing-system-4/1.0.0/screenshot-dark.png","dark_sha256":"0000000000000000000000000000000000000000000000000000000000000034"},"use_case":"Marketing. Fraunces display serif, hot-pink accent, generous radii. Built for public-facing product and landing pages where the brand needs to shout."}'::jsonb,
  true
) ON CONFLICT (id, version) DO NOTHING;

-- Permanent alias used by the App scaffolder, the migration job and the
-- wizard's default branch.
INSERT INTO design_system_alias (alias, asset_id, retargeted_by) VALUES
  ('ds-forge-default', 'design_system:00000000-0000-0000-0000-000000000001:desing-system-1', 'system:forge-platform')
ON CONFLICT (alias) DO UPDATE SET asset_id = EXCLUDED.asset_id;

-- +goose Down
-- Note: the asset table has an immutability trigger; seed rollback drops the
-- alias only. Removing the four built-in assets requires temporarily lifting
-- the trigger and is handled by the runbook documented in
-- openspec/changes/design-system-catalog/runbook.md.
DELETE FROM design_system_alias WHERE alias = 'ds-forge-default';
