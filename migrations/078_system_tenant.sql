-- O tenant 1 deixa de ser uma empresa-cliente e passa a ser o tenant de
-- sistema que abriga o super_admin. Ele não aparece em "Empresas".
UPDATE tenants SET name = 'Plataforma', slug = 'plataforma'
 WHERE id = 1;
