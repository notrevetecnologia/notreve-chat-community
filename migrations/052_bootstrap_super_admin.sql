UPDATE usuarios
SET role = 'super_admin'
WHERE id = (
  SELECT id
  FROM usuarios
  WHERE tenant_id = 1
    AND lower(username) = 'admin'
  ORDER BY id
  LIMIT 1
)
AND NOT EXISTS (
  SELECT 1 FROM usuarios WHERE role = 'super_admin'
);
