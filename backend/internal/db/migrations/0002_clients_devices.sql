-- +goose Up
CREATE TABLE rp.clients (
  id                 BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  ucode              UUID         NOT NULL DEFAULT gen_random_uuid() UNIQUE,
  created_ts         TIMESTAMPTZ  NOT NULL DEFAULT now(),
  created_by_user_id BIGINT       NULL REFERENCES rp.users(id),
  voided_ts          TIMESTAMPTZ  NULL,
  voided_by_user_id  BIGINT       NULL REFERENCES rp.users(id),

  name               TEXT         NOT NULL,
  phone              TEXT         NULL,
  email              TEXT         NULL,
  dni_cuit           TEXT         NULL,
  address            TEXT         NULL,
  notes              TEXT         NULL,
  client_type        TEXT         NOT NULL DEFAULT 'particular' CHECK (client_type IN ('particular', 'empresa')),
  search             TSVECTOR     GENERATED ALWAYS AS (
    to_tsvector(
      'spanish',
      name || ' ' || coalesce(phone, '') || ' ' || coalesce(email, '') || ' ' || coalesce(dni_cuit, '')
    )
  ) STORED
);

CREATE UNIQUE INDEX clients_phone_unique ON rp.clients (phone) WHERE phone IS NOT NULL AND voided_ts IS NULL;
CREATE INDEX clients_search_idx ON rp.clients USING GIN (search);
CREATE INDEX clients_name_lower_idx ON rp.clients (lower(name)) WHERE voided_ts IS NULL;

CREATE TABLE rp.brands (
  id                 BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  ucode              UUID         NOT NULL DEFAULT gen_random_uuid() UNIQUE,
  created_ts         TIMESTAMPTZ  NOT NULL DEFAULT now(),
  created_by_user_id BIGINT       NULL REFERENCES rp.users(id),
  voided_ts          TIMESTAMPTZ  NULL,
  voided_by_user_id  BIGINT       NULL REFERENCES rp.users(id),

  name               TEXT         NOT NULL UNIQUE
);

CREATE TABLE rp.device_models (
  id                 BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  ucode              UUID         NOT NULL DEFAULT gen_random_uuid() UNIQUE,
  created_ts         TIMESTAMPTZ  NOT NULL DEFAULT now(),
  created_by_user_id BIGINT       NULL REFERENCES rp.users(id),
  voided_ts          TIMESTAMPTZ  NULL,
  voided_by_user_id  BIGINT       NULL REFERENCES rp.users(id),

  brand_id           BIGINT       NOT NULL REFERENCES rp.brands(id) ON DELETE RESTRICT,
  name               TEXT         NOT NULL,

  UNIQUE (brand_id, name)
);

CREATE INDEX device_models_brand_idx ON rp.device_models (brand_id);

CREATE TABLE rp.article_types (
  id                 BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  ucode              UUID         NOT NULL DEFAULT gen_random_uuid() UNIQUE,
  created_ts         TIMESTAMPTZ  NOT NULL DEFAULT now(),
  created_by_user_id BIGINT       NULL REFERENCES rp.users(id),
  voided_ts          TIMESTAMPTZ  NULL,
  voided_by_user_id  BIGINT       NULL REFERENCES rp.users(id),

  name               TEXT         NOT NULL UNIQUE
);

CREATE TABLE rp.devices (
  id                 BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  ucode              UUID         NOT NULL DEFAULT gen_random_uuid() UNIQUE,
  created_ts         TIMESTAMPTZ  NOT NULL DEFAULT now(),
  created_by_user_id BIGINT       NULL REFERENCES rp.users(id),
  voided_ts          TIMESTAMPTZ  NULL,
  voided_by_user_id  BIGINT       NULL REFERENCES rp.users(id),

  client_id          BIGINT       NOT NULL REFERENCES rp.clients(id) ON DELETE RESTRICT,
  brand_id           BIGINT       NOT NULL REFERENCES rp.brands(id) ON DELETE RESTRICT,
  model_id           BIGINT       NULL REFERENCES rp.device_models(id) ON DELETE SET NULL,
  article_type_id    BIGINT       NOT NULL REFERENCES rp.article_types(id) ON DELETE RESTRICT,
  serial_number      TEXT         NULL,
  color              TEXT         NULL,
  description        TEXT         NULL
);

CREATE INDEX devices_client_idx ON rp.devices (client_id) WHERE voided_ts IS NULL;
CREATE INDEX devices_serial_idx ON rp.devices (serial_number) WHERE serial_number IS NOT NULL AND voided_ts IS NULL;

-- +goose Down
DROP TABLE IF EXISTS rp.devices;
DROP TABLE IF EXISTS rp.device_models;
DROP TABLE IF EXISTS rp.article_types;
DROP TABLE IF EXISTS rp.brands;
DROP TABLE IF EXISTS rp.clients;
