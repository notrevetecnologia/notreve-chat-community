-- Marca padrão do produto atualizada para a edição comunidade.
ALTER TABLE settings ALTER COLUMN brand_name SET DEFAULT 'Notreve Chat Community';

UPDATE settings SET brand_name = 'Notreve Chat Community'
 WHERE brand_name = 'Notreve Agente IA';

UPDATE platform_branding SET name = 'Notreve Chat Community'
 WHERE name = 'Notreve Agente IA';
