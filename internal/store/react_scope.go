package store

import (
	"context"
	"strconv"
)

func contatosFilterClause(args []interface{}, q string, segmentID int, temWhatsapp *bool) (string, []interface{}) {
	query := ""
	if q != "" {
		args = append(args, "%"+q+"%")
		query += ` AND (lower(name) LIKE lower($` + strconv.Itoa(len(args)) + `) OR phone LIKE $` + strconv.Itoa(len(args)) + `)`
	}
	if segmentID > 0 {
		args = append(args, segmentID)
		query += ` AND EXISTS (SELECT 1 FROM contato_segmentos cs WHERE cs.tenant_id=contatos.tenant_id AND cs.contato_id=contatos.id AND cs.segmento_id=$` + strconv.Itoa(len(args)) + `)`
	}
	if temWhatsapp != nil {
		args = append(args, *temWhatsapp)
		query += ` AND tem_whatsapp=$` + strconv.Itoa(len(args))
	}
	return query, args
}

func (s *Store) ListContatosFilteredForTenant(ctx context.Context, q string, segmentID int, temWhatsapp *bool, limit, offset int) ([]Contato, error) {
	args := []interface{}{TenantID(ctx)}
	filter, args := contatosFilterClause(args, q, segmentID, temWhatsapp)
	query := `SELECT id, name, phone, email, notes, tem_whatsapp, tags, campos_extras, created_at, updated_at FROM contatos WHERE tenant_id=$1` + filter
	args = append(args, limit, offset)
	query += ` ORDER BY name ASC, id DESC LIMIT $` + strconv.Itoa(len(args)-1) + ` OFFSET $` + strconv.Itoa(len(args))
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Contato
	for rows.Next() {
		var c Contato
		if err := rows.Scan(&c.ID, &c.Name, &c.Phone, &c.Email, &c.Notes, &c.TemWhatsapp, &c.Tags, &c.CamposExtras, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) CountContatosForTenant(ctx context.Context, q string, segmentID int, temWhatsapp *bool) (int, error) {
	args := []interface{}{TenantID(ctx)}
	filter, args := contatosFilterClause(args, q, segmentID, temWhatsapp)
	var n int
	err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM contatos WHERE tenant_id=$1`+filter, args...).Scan(&n)
	return n, err
}

func (s *Store) GetContatoForTenant(ctx context.Context, id int) (Contato, error) {
	var c Contato
	err := s.pool.QueryRow(ctx,
		`SELECT id, name, phone, email, notes, tem_whatsapp, tags, campos_extras, created_at, updated_at FROM contatos WHERE tenant_id=$1 AND id=$2`, TenantID(ctx), id).
		Scan(&c.ID, &c.Name, &c.Phone, &c.Email, &c.Notes, &c.TemWhatsapp, &c.Tags, &c.CamposExtras, &c.CreatedAt, &c.UpdatedAt)
	return c, err
}

func (s *Store) UpsertContatoGetIDForTenant(ctx context.Context, c Contato) (int, error) {
	var id int
	err := s.pool.QueryRow(ctx,
		`INSERT INTO contatos (tenant_id, name, phone, email, notes, tem_whatsapp, tags, campos_extras) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
			 ON CONFLICT (tenant_id, phone) DO UPDATE SET name=EXCLUDED.name, email=EXCLUDED.email, notes=EXCLUDED.notes, tem_whatsapp=EXCLUDED.tem_whatsapp, updated_at=now()
		 RETURNING id`, TenantID(ctx), c.Name, c.Phone, c.Email, c.Notes, c.TemWhatsapp, c.Tags, contatoCamposExtras(c)).Scan(&id)
	return id, err
}

func (s *Store) CreateContatoForTenant(ctx context.Context, c Contato) (int, error) {
	var id int
	err := s.pool.QueryRow(ctx,
		`INSERT INTO contatos (tenant_id, name, phone, email, notes, tem_whatsapp, tags, campos_extras) VALUES ($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id`,
		TenantID(ctx), c.Name, c.Phone, c.Email, c.Notes, c.TemWhatsapp, c.Tags, contatoCamposExtras(c)).Scan(&id)
	return id, err
}

func (s *Store) UpdateContatoForTenant(ctx context.Context, c Contato) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE contatos SET name=$1, phone=$2, email=$3, notes=$4, tem_whatsapp=$5, tags=$7, campos_extras=$8, updated_at=now() WHERE tenant_id=$9 AND id=$6`,
		c.Name, c.Phone, c.Email, c.Notes, c.TemWhatsapp, c.ID, c.Tags, contatoCamposExtras(c), TenantID(ctx))
	return err
}

func (s *Store) DeleteContatoForTenant(ctx context.Context, id int) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM contatos WHERE tenant_id=$1 AND id=$2`, TenantID(ctx), id)
	return err
}

func (s *Store) ListSegmentosForTenant(ctx context.Context) ([]Segmento, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT sg.id, sg.name, sg.description,`+segmentEffectiveCountSQL+`,
			sg.filter_tem_whatsapp, sg.filter_canal, sg.created_at
		FROM segmentos sg WHERE sg.tenant_id=$1 ORDER BY sg.name ASC`, TenantID(ctx))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Segmento
	for rows.Next() {
		var sg Segmento
		if err := rows.Scan(&sg.ID, &sg.Name, &sg.Description, &sg.Count, &sg.FilterTemWhatsapp, &sg.FilterCanal, &sg.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, sg)
	}
	return out, rows.Err()
}

func (s *Store) SegmentosOfContatoForTenant(ctx context.Context, contatoID int) ([]int, error) {
	rows, err := s.pool.Query(ctx, `SELECT segmento_id FROM contato_segmentos WHERE tenant_id=$1 AND contato_id=$2`, TenantID(ctx), contatoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (s *Store) SetContatoSegmentosForTenant(ctx context.Context, contatoID int, segmentoIDs []int) error {
	if _, err := s.pool.Exec(ctx, `DELETE FROM contato_segmentos WHERE tenant_id=$1 AND contato_id=$2`, TenantID(ctx), contatoID); err != nil {
		return err
	}
	for _, sid := range segmentoIDs {
		if _, err := s.pool.Exec(ctx, `INSERT INTO contato_segmentos (tenant_id, contato_id, segmento_id) SELECT $1,$2,$3 WHERE EXISTS (SELECT 1 FROM segmentos WHERE tenant_id=$1 AND id=$3) ON CONFLICT DO NOTHING`, TenantID(ctx), contatoID, sid); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) ListIntegrationsForTenant(ctx context.Context) ([]Integration, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, name, description, method, url, headers, parameters, requires_identity, allowed_identities, identity_config, identity_verify_url, identity_verify_method, identity_verify_param, identity_match_field, identity_client_id_field, created_at FROM integrations WHERE tenant_id=$1 ORDER BY name ASC`, TenantID(ctx))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Integration
	for rows.Next() {
		i, err := scanIntegration(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, i)
	}
	return out, rows.Err()
}

func (s *Store) GetIntegrationForTenant(ctx context.Context, id int) (Integration, error) {
	return scanIntegration(s.pool.QueryRow(ctx,
		`SELECT id, name, description, method, url, headers, parameters, requires_identity, allowed_identities, identity_config, identity_verify_url, identity_verify_method, identity_verify_param, identity_match_field, identity_client_id_field, created_at FROM integrations WHERE tenant_id=$1 AND id=$2`, TenantID(ctx), id))
}
