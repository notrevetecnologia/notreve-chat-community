package store

import (
	"context"
	"fmt"
	"strings"
)

// segmentEffectiveCountSQL calcula o número de contatos efetivos de um segmento
// (assinados + os que casam com o filtro), referenciando a linha externa `sg`.
const segmentEffectiveCountSQL = `(
	SELECT COUNT(*) FROM (
		SELECT c.id FROM contato_segmentos cs
		JOIN contatos c ON c.id = cs.contato_id AND c.tenant_id = cs.tenant_id
		WHERE cs.tenant_id = sg.tenant_id AND cs.segmento_id = sg.id
		UNION
		SELECT c.id FROM contatos c
		WHERE c.tenant_id = sg.tenant_id
		  AND (sg.filter_tem_whatsapp IS NOT NULL OR sg.filter_canal <> '')
		  AND (sg.filter_tem_whatsapp IS NULL OR c.tem_whatsapp = sg.filter_tem_whatsapp)
		  AND (sg.filter_canal = '' OR EXISTS (
			SELECT 1 FROM conversations cv
			WHERE cv.tenant_id = sg.tenant_id AND cv.customer_phone = c.phone AND cv.channel = sg.filter_canal
		  ))
	) ef
)`

// GetSegmento retorna um segmento com os filtros.
func (s *Store) GetSegmento(ctx context.Context, id int) (Segmento, error) {
	var sg Segmento
	err := s.pool.QueryRow(ctx,
		`SELECT id, name, description, filter_tem_whatsapp, filter_canal, created_at
		 FROM segmentos WHERE tenant_id=$1 AND id=$2`, TenantID(ctx), id).
		Scan(&sg.ID, &sg.Name, &sg.Description, &sg.FilterTemWhatsapp, &sg.FilterCanal, &sg.CreatedAt)
	return sg, err
}

// CreateSegmentoComFiltros cria um segmento com filtros opcionais.
func (s *Store) CreateSegmentoComFiltros(ctx context.Context, name, description string, temWhatsapp *bool, canal string) (int, error) {
	if canal = strings.TrimSpace(canal); canal == "" {
		canal = ""
	}
	var id int
	err := s.pool.QueryRow(ctx,
		`INSERT INTO segmentos (tenant_id, name, description, filter_tem_whatsapp, filter_canal)
		 VALUES ($1,$2,$3,$4,NULLIF($5,'')) RETURNING id`,
		TenantID(ctx), name, description, temWhatsapp, canal).Scan(&id)
	if err != nil {
		return 0, err
	}
	return id, nil
}

// effectiveSegmentContacts retorna os contatos efetivos de um segmento: os
// assinados explicitamente mais os que casam com o filtro (quando houver).
func (s *Store) effectiveSegmentContacts(ctx context.Context, sg Segmento) ([]Contato, error) {
	q := `SELECT DISTINCT c.id, c.name, c.phone, c.email, c.notes, c.tem_whatsapp, c.tags, c.campos_extras, c.created_at, c.updated_at
		FROM contatos c WHERE c.tenant_id = $1`
	args := []interface{}{TenantID(ctx)}
	assigned := `c.id IN (SELECT cs.contato_id FROM contato_segmentos cs WHERE cs.tenant_id = $1 AND cs.segmento_id = $2)`
	args = append(args, sg.ID)
	var filters []string
	if sg.FilterTemWhatsapp != nil {
		args = append(args, *sg.FilterTemWhatsapp)
		filters = append(filters, fmt.Sprintf("c.tem_whatsapp = $%d", len(args)))
	}
	if sg.FilterCanal != "" {
		args = append(args, sg.FilterCanal)
		filters = append(filters, fmt.Sprintf(`EXISTS (
			SELECT 1 FROM conversations cv
			WHERE cv.tenant_id = c.tenant_id AND cv.customer_phone = c.phone AND cv.channel = $%d)`, len(args)))
	}
	if len(filters) > 0 {
		q += " AND (" + assigned + " OR " + strings.Join(filters, " OR ") + ")"
	} else {
		q += " AND " + assigned
	}
	q += " ORDER BY c.name ASC, c.id DESC"
	rows, err := s.pool.Query(ctx, q, args...)
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
