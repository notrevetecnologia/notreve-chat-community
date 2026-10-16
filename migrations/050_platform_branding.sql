-- Marca global da plataforma (super_admin).
--
-- A Notreve (super_admin) controla a marca exibida no PAINEL e como padrão da
-- plataforma. Cada empresa (admin) pode ter a própria marca apenas nas
-- SUPERFÍCIES voltadas ao cliente final (webchat, mensagens, avaliação), que
-- ficam em settings (brand_name/logo_url/cores por tenant). Esta tabela guarda a
-- marca global da plataforma, com uma única linha (id = 1).
CREATE TABLE IF NOT EXISTS platform_branding (
  id SMALLINT PRIMARY KEY DEFAULT 1,
  name TEXT NOT NULL DEFAULT '',
  logo_url TEXT NOT NULL DEFAULT '',
  favicon_url TEXT NOT NULL DEFAULT '',
  primary_color TEXT NOT NULL DEFAULT '',
  secondary_color TEXT NOT NULL DEFAULT '',
  accent_color TEXT NOT NULL DEFAULT '',
  background_color TEXT NOT NULL DEFAULT '',
  loading_color TEXT NOT NULL DEFAULT '',
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CHECK (id = 1)
);

INSERT INTO platform_branding (id, name, primary_color, secondary_color, accent_color, background_color, loading_color)
VALUES (1, 'Notreve Agente IA', '#2dd27f', '#101a2a', '#7c6cff', '#f3f6fa', '#2dd27f')
ON CONFLICT (id) DO NOTHING;
