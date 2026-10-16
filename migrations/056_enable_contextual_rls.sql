CREATE OR REPLACE FUNCTION tenant_row_visible(row_tenant_id integer)
RETURNS boolean
LANGUAGE sql
STABLE
AS $$
  SELECT current_app_tenant_id() IS NULL
      OR row_tenant_id = current_app_tenant_id()
$$;

DO $$
DECLARE
  target_table text;
BEGIN
  FOR target_table IN
    SELECT c.table_name
    FROM information_schema.columns c
    JOIN information_schema.tables t
      ON t.table_schema = c.table_schema
     AND t.table_name = c.table_name
    WHERE c.table_schema = 'public'
      AND c.column_name = 'tenant_id'
      AND t.table_type = 'BASE TABLE'
  LOOP
    EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', target_table);
    EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY', target_table);
    EXECUTE format('DROP POLICY IF EXISTS tenant_isolation ON %I', target_table);
    EXECUTE format('DROP POLICY IF EXISTS tenant_context_isolation ON %I', target_table);
    EXECUTE format(
      'CREATE POLICY tenant_context_isolation ON %I USING (tenant_row_visible(tenant_id)) WITH CHECK (tenant_row_visible(tenant_id))',
      target_table
    );
  END LOOP;
END;
$$;
