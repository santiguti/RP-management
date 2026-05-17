-- +goose Up
CREATE SCHEMA IF NOT EXISTS rp;

CREATE TABLE rp.users (
  id                 BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  ucode              UUID         NOT NULL DEFAULT gen_random_uuid() UNIQUE,
  created_ts         TIMESTAMPTZ  NOT NULL DEFAULT now(),
  created_by_user_id BIGINT       NULL REFERENCES rp.users(id),
  voided_ts          TIMESTAMPTZ  NULL,
  voided_by_user_id  BIGINT       NULL REFERENCES rp.users(id),

  username           TEXT         NOT NULL UNIQUE,
  password_hash      TEXT         NOT NULL,
  full_name          TEXT         NOT NULL,
  role               TEXT         NOT NULL CHECK (role IN ('owner', 'employee')),
  last_login_ts      TIMESTAMPTZ  NULL
);

CREATE TABLE rp.sessions (
  -- sha256 of the random cookie token; the plaintext token lives only in the
  -- client cookie, so a DB leak can't impersonate sessions.
  id            BYTEA        PRIMARY KEY,
  user_id       BIGINT       NOT NULL REFERENCES rp.users(id) ON DELETE CASCADE,
  expires_at    TIMESTAMPTZ  NOT NULL,
  last_used_ts  TIMESTAMPTZ  NOT NULL DEFAULT now(),
  ip            INET         NULL,
  user_agent    TEXT         NULL,
  created_ts    TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX sessions_user_id_idx     ON rp.sessions (user_id);
CREATE INDEX sessions_expires_at_idx  ON rp.sessions (expires_at);

-- +goose Down
DROP TABLE IF EXISTS rp.sessions;
DROP TABLE IF EXISTS rp.users;
DROP SCHEMA IF EXISTS rp;
