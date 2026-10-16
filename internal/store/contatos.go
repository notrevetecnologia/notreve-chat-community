package store

import (
	"context"
	"strings"
)

// NormalizePhoneWithDDI limpa o telefone e normaliza para o formato internacional
// (incluindo o DDI 55 para números locais brasileiros), usado para dedup e
// envio. Remove caracteres não numéricos e zeros à esquerda.
func NormalizePhoneWithDDI(value string) string {
	var b strings.Builder
	for _, r := range value {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	digits := strings.TrimLeft(b.String(), "0")
	if digits == "" {
		return ""
	}
	if len(digits) >= 12 && strings.HasPrefix(digits, "55") {
		return digits
	}
	if len(digits) >= 10 && len(digits) <= 11 {
		return "55" + digits
	}
	return digits
}

// ImportContatoForTenant insere ou atualiza um contato pelo telefone dentro do
// escopo do tenant. Retorna o id e um booleano; true quando o contato foi
// recém-criado e false quando já existia (feito upsert).
func (s *Store) ImportContatoForTenant(ctx context.Context, c Contato) (int, bool, error) {
	var id int
	var inserted bool
	err := s.pool.QueryRow(ctx, `
		INSERT INTO contatos (tenant_id, name, phone, email, notes, tem_whatsapp, tags, campos_extras)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		ON CONFLICT (tenant_id, phone) DO UPDATE
		SET name=EXCLUDED.name, email=EXCLUDED.email, notes=EXCLUDED.notes, tem_whatsapp=EXCLUDED.tem_whatsapp, updated_at=now()
		RETURNING id, (xmax = 0)`,
		TenantID(ctx), c.Name, c.Phone, c.Email, c.Notes, c.TemWhatsapp, c.Tags, contatoCamposExtras(c)).Scan(&id, &inserted)
	return id, inserted, err
}

// AddContatoSegmentoForTenant adiciona um contato a um segmento sem remover os
// relacionamentos existentes.
func (s *Store) AddContatoSegmentoForTenant(ctx context.Context, contatoID, segmentoID int) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO contato_segmentos (tenant_id, contato_id, segmento_id)
		SELECT $1,$2,$3
		WHERE EXISTS (SELECT 1 FROM contatos WHERE tenant_id=$1 AND id=$2)
		  AND EXISTS (SELECT 1 FROM segmentos WHERE tenant_id=$1 AND id=$3)
		ON CONFLICT DO NOTHING`, TenantID(ctx), contatoID, segmentoID)
	return err
}
