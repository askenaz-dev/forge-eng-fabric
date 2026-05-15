-- +goose Up
-- Local mirror of platform identities. Populated lazily by the auth middleware
-- on every authenticated request — no Keycloak admin credentials required.
-- Used by the portal to power user pickers (workspace owners, etc.).
CREATE TABLE platform_user (
  subject     text PRIMARY KEY,
  username    text,
  email       text,
  first_seen  timestamptz NOT NULL DEFAULT now(),
  last_seen   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX ON platform_user (username);
CREATE INDEX ON platform_user (email);

-- +goose Down
DROP TABLE IF EXISTS platform_user;
