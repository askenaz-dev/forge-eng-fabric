-- +goose Up
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE traceability_node (
  id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  type          text NOT NULL,
  external_id   text NOT NULL,
  workspace_id  text NOT NULL,
  metadata      jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now(),
  UNIQUE (workspace_id, type, external_id)
);

CREATE TABLE traceability_link (
  id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  from_node     uuid NOT NULL REFERENCES traceability_node(id) ON DELETE CASCADE,
  to_node       uuid NOT NULL REFERENCES traceability_node(id) ON DELETE CASCADE,
  relation      text NOT NULL,
  source        text NOT NULL,
  source_event  text NOT NULL,
  metadata      jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at    timestamptz NOT NULL DEFAULT now(),
  UNIQUE (from_node, to_node, relation, source_event)
);

CREATE MATERIALIZED VIEW traceability_openspec_graph AS
SELECT
  root.external_id AS openspec_id,
  n.id AS node_id,
  n.type AS node_type,
  n.external_id,
  l.id AS link_id,
  l.relation,
  l.source_event,
  l.created_at AS link_created_at
FROM traceability_node root
JOIN traceability_link l ON l.from_node = root.id OR l.to_node = root.id
JOIN traceability_node n ON n.id = l.from_node OR n.id = l.to_node
WHERE root.type = 'openspec';

CREATE INDEX traceability_node_workspace_idx ON traceability_node(workspace_id);
CREATE INDEX traceability_node_external_idx ON traceability_node(type, external_id);
CREATE INDEX traceability_link_from_idx ON traceability_link(from_node);
CREATE INDEX traceability_link_to_idx ON traceability_link(to_node);
CREATE INDEX traceability_openspec_graph_idx ON traceability_openspec_graph(openspec_id);

-- +goose Down
DROP MATERIALIZED VIEW IF EXISTS traceability_openspec_graph;
DROP TABLE IF EXISTS traceability_link;
DROP TABLE IF EXISTS traceability_node;
