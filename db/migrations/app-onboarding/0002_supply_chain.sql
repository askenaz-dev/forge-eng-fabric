-- +goose Up
-- Phase 2: supply-chain & pipeline gate persistence (spec deltas:
-- ci-pipeline-baseline, supply-chain-attestations).

CREATE TABLE pipeline_gate_result (
  id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id    uuid NOT NULL,
  repo_full_name  text NOT NULL,
  pr_number       integer,
  commit_sha      text NOT NULL,
  stage           text NOT NULL,
  tool            text NOT NULL,
  outcome         text NOT NULL CHECK (outcome IN ('pass','warn','fail','skipped')),
  severity_counts jsonb NOT NULL DEFAULT '{}'::jsonb,
  report_url      text,
  policy_version  text,
  created_at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX pipeline_gate_result_pr_idx ON pipeline_gate_result(repo_full_name, pr_number);
CREATE INDEX pipeline_gate_result_sha_idx ON pipeline_gate_result(commit_sha);

CREATE TABLE image_signature (
  id                    uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id          uuid NOT NULL,
  asset_id              text,
  image_repository      text NOT NULL,
  image_digest          text NOT NULL,
  signer_identity       text NOT NULL,
  signer_issuer         text NOT NULL,
  rekor_url             text,
  rekor_log             text NOT NULL DEFAULT 'public'
                          CHECK (rekor_log IN ('public','private')),
  attestation_present   boolean NOT NULL DEFAULT false,
  signature_verified    boolean NOT NULL DEFAULT false,
  attestation_verified  boolean NOT NULL DEFAULT false,
  created_at            timestamptz NOT NULL DEFAULT now(),
  UNIQUE (image_repository, image_digest)
);
CREATE INDEX image_signature_asset_idx ON image_signature(asset_id);

CREATE TABLE sbom_record (
  id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id    uuid NOT NULL,
  asset_id        text,
  image_digest    text NOT NULL,
  format          text NOT NULL CHECK (format IN ('spdx','cyclonedx')),
  uri             text NOT NULL,
  size_bytes      bigint,
  created_at      timestamptz NOT NULL DEFAULT now(),
  UNIQUE (image_digest, format)
);

CREATE TABLE pr_openspec_link (
  id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id    uuid NOT NULL,
  repo_full_name  text NOT NULL,
  pr_number       integer NOT NULL,
  pr_url          text NOT NULL,
  openspec_id     text NOT NULL,
  status          text NOT NULL CHECK (status IN ('linked','missing','warning','invalid')),
  created_at      timestamptz NOT NULL DEFAULT now(),
  UNIQUE (repo_full_name, pr_number, openspec_id)
);
CREATE INDEX pr_openspec_link_pr_idx ON pr_openspec_link(repo_full_name, pr_number);

-- +goose Down
DROP TABLE IF EXISTS pr_openspec_link;
DROP TABLE IF EXISTS sbom_record;
DROP TABLE IF EXISTS image_signature;
DROP TABLE IF EXISTS pipeline_gate_result;
