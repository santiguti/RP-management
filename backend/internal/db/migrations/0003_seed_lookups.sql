-- +goose Up
INSERT INTO rp.brands (name) VALUES
  ('Samsung'), ('Apple'), ('LG'), ('Philips'), ('Whirlpool'),
  ('BGH'), ('Drean'), ('Liliana'), ('Atma'), ('Yelmo'),
  ('Noblex'), ('RCA'), ('Hisense'), ('Xiaomi'), ('Motorola'),
  ('Otros')
ON CONFLICT (name) DO NOTHING;

INSERT INTO rp.article_types (name) VALUES
  ('celular'), ('notebook'), ('tablet'),
  ('heladera'), ('lavarropas'), ('microondas'), ('aire_acondicionado'),
  ('tv'), ('audio'), ('otro')
ON CONFLICT (name) DO NOTHING;

-- +goose Down
DELETE FROM rp.brands WHERE name IN (
  'Samsung','Apple','LG','Philips','Whirlpool',
  'BGH','Drean','Liliana','Atma','Yelmo',
  'Noblex','RCA','Hisense','Xiaomi','Motorola','Otros'
);

DELETE FROM rp.article_types WHERE name IN (
  'celular','notebook','tablet',
  'heladera','lavarropas','microondas','aire_acondicionado',
  'tv','audio','otro'
);
