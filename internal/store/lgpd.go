package store

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5"
)

func (s *Store) RecordConsent(ctx context.Context, c LGPDConsent) (int, error) {
	var id int
	err := s.pool.QueryRow(ctx,
		`INSERT INTO lgpd_consents (tenant_id, contato_id, conversation_id, scope, base_legal, aceito, canal, token, ip)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING id`,
		TenantID(ctx), c.ContatoID, c.ConversationID, c.Scope, c.BaseLegal, c.Aceito, c.Canal, c.Token, c.IP).Scan(&id)
	return id, err
}

func (s *Store) ListConsents(ctx context.Context, scope string, limit, offset int) ([]LGPDConsent, int, error) {
	var total int
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM lgpd_consents WHERE tenant_id=$1 AND ($2='' OR lower(scope) LIKE lower('%'||$2||'%'))`, TenantID(ctx), scope).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.pool.Query(ctx,
		`SELECT id, tenant_id, contato_id, conversation_id, scope, base_legal, aceito, canal, token, ip, data
		 FROM lgpd_consents WHERE tenant_id=$1 AND ($2='' OR lower(scope) LIKE lower('%'||$2||'%')) ORDER BY data DESC, id DESC LIMIT $3 OFFSET $4`,
		TenantID(ctx), scope, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []LGPDConsent
	for rows.Next() {
		var c LGPDConsent
		if err := rows.Scan(&c.ID, &c.TenantID, &c.ContatoID, &c.ConversationID, &c.Scope, &c.BaseLegal, &c.Aceito, &c.Canal, &c.Token, &c.IP, &c.Data); err != nil {
			return nil, 0, err
		}
		out = append(out, c)
	}
	return out, total, rows.Err()
}

func (s *Store) CreatePolitica(ctx context.Context, p LGPDPolitica) (int, error) {
	tenantID := TenantID(ctx)
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `UPDATE lgpd_politica SET active=FALSE, updated_at=now() WHERE tenant_id=$1 AND active=TRUE`, tenantID); err != nil {
		return 0, err
	}
	var id int
	err = tx.QueryRow(ctx,
		`INSERT INTO lgpd_politica (tenant_id, version, tipo, texto, url, vigencia_inicio, active)
		 VALUES ($1,$2,$3,$4,$5,$6,TRUE) RETURNING id`,
		tenantID, p.Version, p.Tipo, p.Texto, p.URL, p.VigenciaInicio).Scan(&id)
	if err != nil {
		return 0, err
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return id, nil
}

func (s *Store) GetCurrentPolitica(ctx context.Context) (LGPDPolitica, error) {
	return s.getPolitica(ctx, `WHERE tenant_id=$1 AND active=TRUE ORDER BY version DESC, id DESC LIMIT 1`)
}

func (s *Store) ListPoliticas(ctx context.Context) ([]LGPDPolitica, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, tenant_id, version, tipo, texto, url, vigencia_inicio, vigencia_fim, active, created_at, updated_at
		 FROM lgpd_politica WHERE tenant_id=$1 ORDER BY version DESC, id DESC`, TenantID(ctx))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []LGPDPolitica
	for rows.Next() {
		p, err := scanLGPDPolitica(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) getPolitica(ctx context.Context, where string) (LGPDPolitica, error) {
	return scanLGPDPolitica(s.pool.QueryRow(ctx,
		`SELECT id, tenant_id, version, tipo, texto, url, vigencia_inicio, vigencia_fim, active, created_at, updated_at
		 FROM lgpd_politica `+where, TenantID(ctx)))
}

func scanLGPDPolitica(row pgx.Row) (LGPDPolitica, error) {
	var p LGPDPolitica
	err := row.Scan(&p.ID, &p.TenantID, &p.Version, &p.Tipo, &p.Texto, &p.URL, &p.VigenciaInicio, &p.VigenciaFim, &p.Active, &p.CreatedAt, &p.UpdatedAt)
	return p, err
}

func (s *Store) RecordAudit(ctx context.Context, usuarioID int, acao, recurso string, entidadeID *int, ip string, metadados map[string]interface{}) error {
	blob := []byte("{}")
	if len(metadados) > 0 {
		if encoded, err := json.Marshal(metadados); err == nil {
			blob = encoded
		}
	}
	_, err := s.pool.Exec(ctx,
		`INSERT INTO audit_log (tenant_id, usuario_id, acao, recurso, entidade_id, ip, metadados)
		 VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		TenantID(ctx), usuarioID, acao, recurso, entidadeID, ip, blob)
	return err
}

func (s *Store) ListAudit(ctx context.Context, acao, recurso, q string, limit, offset int) ([]AuditLog, int, error) {
	var total int
	if err := s.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM audit_log a WHERE a.tenant_id=$1 AND ($2='' OR a.acao=$2) AND ($3='' OR a.recurso=$3)
		   AND ($4='' OR lower(a.acao) LIKE lower('%'||$4||'%') OR lower(a.recurso) LIKE lower('%'||$4||'%'))`,
		TenantID(ctx), acao, recurso, q).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.pool.Query(ctx,
		`SELECT a.id, a.tenant_id, a.usuario_id, COALESCE(u.name,''), a.acao, a.recurso, a.entidade_id, a.ip, a.data, a.metadados
		 FROM audit_log a LEFT JOIN usuarios u ON u.id=a.usuario_id AND u.tenant_id=a.tenant_id
		 WHERE a.tenant_id=$1 AND ($2='' OR a.acao=$2) AND ($3='' OR a.recurso=$3)
		   AND ($4='' OR lower(a.acao) LIKE lower('%'||$4||'%') OR lower(a.recurso) LIKE lower('%'||$4||'%'))
		 ORDER BY a.data DESC, a.id DESC LIMIT $5 OFFSET $6`,
		TenantID(ctx), acao, recurso, q, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []AuditLog
	for rows.Next() {
		var a AuditLog
		var meta []byte
		if err := rows.Scan(&a.ID, &a.TenantID, &a.UsuarioID, &a.Usuario, &a.Acao, &a.Recurso, &a.EntidadeID, &a.IP, &a.Data, &meta); err != nil {
			return nil, 0, err
		}
		if len(meta) > 0 {
			_ = json.Unmarshal(meta, &a.Metadados)
		}
		out = append(out, a)
	}
	return out, total, rows.Err()
}

func (s *Store) CountConsents(ctx context.Context) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM lgpd_consents WHERE tenant_id=$1`, TenantID(ctx)).Scan(&n)
	return n, err
}

func (s *Store) CountAuditLogs(ctx context.Context) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM audit_log WHERE tenant_id=$1`, TenantID(ctx)).Scan(&n)
	return n, err
}

func (s *Store) SetContatoDocumento(ctx context.Context, contatoID int, documento string) error {
	ciphertext, nonce, err := s.EncryptLGPDSensitive(documento)
	if err != nil {
		_, err = s.pool.Exec(ctx,
			`UPDATE contatos SET documento_cifrado=$1, documento_nonce=NULL, updated_at=now() WHERE tenant_id=$2 AND id=$3`,
			[]byte(documento), TenantID(ctx), contatoID)
		return err
	}
	_, err = s.pool.Exec(ctx,
		`UPDATE contatos SET documento_cifrado=$1, documento_nonce=$2, updated_at=now() WHERE tenant_id=$3 AND id=$4`,
		ciphertext, nonce, TenantID(ctx), contatoID)
	return err
}

func (s *Store) GetContatoDocumento(ctx context.Context, contatoID int) (string, error) {
	var ciphertext, nonce []byte
	err := s.pool.QueryRow(ctx,
		`SELECT documento_cifrado, documento_nonce FROM contatos WHERE tenant_id=$1 AND id=$2`,
		TenantID(ctx), contatoID).Scan(&ciphertext, &nonce)
	if err != nil {
		return "", err
	}
	if len(nonce) == 0 {
		return string(ciphertext), nil
	}
	return s.DecryptLGPDSensitive(ciphertext, nonce)
}
