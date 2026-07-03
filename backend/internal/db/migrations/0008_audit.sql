-- +goose Up
CREATE TABLE rp.audit_log (
  id              BIGINT       GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  ucode           UUID         NOT NULL DEFAULT gen_random_uuid() UNIQUE,
  created_ts      TIMESTAMPTZ  NOT NULL DEFAULT now(),

  actor_user_id   BIGINT       NULL REFERENCES rp.users(id) ON DELETE SET NULL,
  action          TEXT         NOT NULL CHECK (length(action) BETWEEN 1 AND 80),
  entity_type     TEXT         NOT NULL CHECK (length(entity_type) BETWEEN 1 AND 40),
  entity_id       BIGINT       NULL,
  entity_ucode    UUID         NULL,

  before_json     JSONB        NULL,
  after_json      JSONB        NULL,

  ip              TEXT         NULL,
  user_agent      TEXT         NULL
);

CREATE INDEX audit_log_actor_idx  ON rp.audit_log(actor_user_id, created_ts DESC);
CREATE INDEX audit_log_entity_idx ON rp.audit_log(entity_type, entity_id);
CREATE INDEX audit_log_ucode_idx  ON rp.audit_log(entity_type, entity_ucode) WHERE entity_ucode IS NOT NULL;
CREATE INDEX audit_log_ts_brin    ON rp.audit_log USING BRIN(created_ts);

-- +goose Down
DROP TABLE IF EXISTS rp.audit_log;
