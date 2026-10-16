INSERT INTO modulos (key, name, description, sort) VALUES
  ('relatorios', 'Relatórios', 'Indicadores gerais de atendimento, equipe, campanhas e satisfação', 2)
ON CONFLICT (key) DO NOTHING;
