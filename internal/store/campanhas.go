package store

import (
	"context"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// ---------- Segmentos ----------

func (s *Store) CreateSegmento(ctx context.Context, name, description string) (int, error) {
	var id int
	err := s.pool.QueryRow(ctx, `INSERT INTO segmentos (tenant_id, name, description) VALUES ($1,$2,$3) RETURNING id`, TenantID(ctx), name, description).Scan(&id)
	return id, err
}

func (s *Store) ListSegmentos(ctx context.Context) ([]Segmento, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT sg.id, sg.name, sg.description,`+segmentEffectiveCountSQL+`,
			sg.filter_tem_whatsapp, sg.filter_canal, sg.created_at
		FROM segmentos sg
		WHERE sg.tenant_id=$1 ORDER BY sg.name ASC`, TenantID(ctx))
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

func (s *Store) DeleteSegmento(ctx context.Context, id int) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM segmentos WHERE tenant_id=$1 AND id=$2`, TenantID(ctx), id)
	return err
}

func (s *Store) SegmentosOfContato(ctx context.Context, contatoID int) ([]int, error) {
	rows, err := s.pool.Query(ctx, `SELECT cs.segmento_id FROM contato_segmentos cs
		WHERE cs.tenant_id=$1 AND cs.contato_id=$2
		  AND EXISTS (SELECT 1 FROM contatos c WHERE c.tenant_id=$1 AND c.id=cs.contato_id)`, TenantID(ctx), contatoID)
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

func (s *Store) SetContatoSegmentos(ctx context.Context, contatoID int, segmentoIDs []int) error {
	if _, err := s.pool.Exec(ctx, `DELETE FROM contato_segmentos WHERE tenant_id=$1 AND contato_id=$2`, TenantID(ctx), contatoID); err != nil {
		return err
	}
	for _, sid := range segmentoIDs {
		_, err := s.pool.Exec(ctx, `INSERT INTO contato_segmentos (tenant_id, contato_id, segmento_id)
			SELECT $1,$2,$3
			WHERE EXISTS (SELECT 1 FROM contatos WHERE tenant_id=$1 AND id=$2)
			  AND EXISTS (SELECT 1 FROM segmentos WHERE tenant_id=$1 AND id=$3)
			ON CONFLICT DO NOTHING`, TenantID(ctx), contatoID, sid)
		if err != nil {
			return err
		}
	}
	return nil
}

// ContatosPorSegmento retorna os contatos efetivos do segmento (ou todos quando
// segmentoID=0). Com filtros, inclui também os contatos que casam com o filtro.
func (s *Store) ContatosPorSegmento(ctx context.Context, segmentoID int) ([]Contato, error) {
	if segmentoID == 0 {
		rows, err := s.pool.Query(ctx, `SELECT id, name, phone, email, notes, tem_whatsapp, tags, campos_extras, created_at, updated_at FROM contatos WHERE tenant_id=$1 ORDER BY id ASC`, TenantID(ctx))
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
	sg, err := s.GetSegmento(ctx, segmentoID)
	if err != nil {
		return nil, err
	}
	return s.effectiveSegmentContacts(ctx, sg)
}

// ---------- Grupos de canais ----------

func (s *Store) CreateGrupoCanais(ctx context.Context, name string, channelIDs []int) (int, error) {
	var id int
	err := s.pool.QueryRow(ctx, `INSERT INTO grupo_canais (tenant_id, name) VALUES ($1,$2) RETURNING id`, TenantID(ctx), name).Scan(&id)
	if err != nil {
		return 0, err
	}
	if err := s.setGrupoChannels(ctx, id, channelIDs); err != nil {
		return 0, err
	}
	return id, nil
}

func (s *Store) setGrupoChannels(ctx context.Context, grupoID int, channelIDs []int) error {
	if _, err := s.pool.Exec(ctx, `DELETE FROM grupo_canal WHERE tenant_id=$1 AND grupo_id=$2`, TenantID(ctx), grupoID); err != nil {
		return err
	}
	for _, cid := range channelIDs {
		if _, err := s.pool.Exec(ctx, `INSERT INTO grupo_canal (tenant_id, grupo_id, channel_id)
			SELECT $1,$2,$3
			WHERE EXISTS (SELECT 1 FROM grupo_canais WHERE tenant_id=$1 AND id=$2)
			  AND EXISTS (SELECT 1 FROM channels WHERE tenant_id=$1 AND id=$3)
			ON CONFLICT DO NOTHING`, TenantID(ctx), grupoID, cid); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) ListGruposCanais(ctx context.Context) ([]GrupoCanal, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, name, created_at FROM grupo_canais WHERE tenant_id=$1 ORDER BY name ASC`, TenantID(ctx))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []GrupoCanal
	for rows.Next() {
		var g GrupoCanal
		if err := rows.Scan(&g.ID, &g.Name, &g.CreatedAt); err != nil {
			return nil, err
		}
		g.ChannelIDs, g.ChannelNames = s.grupoChannels(ctx, g.ID)
		out = append(out, g)
	}
	return out, rows.Err()
}

func (s *Store) grupoChannels(ctx context.Context, grupoID int) ([]int, []string) {
	rows, err := s.pool.Query(ctx, `
		SELECT ch.id, ch.name
		FROM grupo_canal gc
		JOIN grupo_canais g ON g.id = gc.grupo_id AND g.tenant_id=gc.tenant_id
		JOIN channels ch ON ch.id = gc.channel_id AND ch.tenant_id=gc.tenant_id
		WHERE gc.tenant_id=$1 AND g.tenant_id=$1 AND gc.grupo_id = $2 ORDER BY ch.name ASC`, TenantID(ctx), grupoID)
	if err != nil {
		return nil, nil
	}
	defer rows.Close()
	var ids []int
	var names []string
	for rows.Next() {
		var id int
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			continue
		}
		ids = append(ids, id)
		names = append(names, name)
	}
	return ids, names
}

// GrupoChannelIDs retorna os ids dos canais de um grupo.
func (s *Store) GrupoChannelIDs(ctx context.Context, grupoID int) ([]int, error) {
	ids, _ := s.grupoChannels(ctx, grupoID)
	return ids, nil
}

// EnviosStatsByChannel retorna (volume enviado, último envio) por canal da campanha.
func (s *Store) EnviosStatsByChannel(ctx context.Context, campanhaID int) (map[int]int, map[int]time.Time, error) {
	volume := map[int]int{}
	last := map[int]time.Time{}
	rows, err := s.pool.Query(ctx, `
		SELECT e.channel_id, COUNT(*), MAX(e.sent_at) FROM campanha_envios e
		WHERE e.tenant_id=$1 AND e.campanha_id=$2 AND e.channel_id IS NOT NULL AND e.status='enviada'
		  AND EXISTS (SELECT 1 FROM campanhas c WHERE c.tenant_id=$1 AND c.id=e.campanha_id)
		GROUP BY e.channel_id`, TenantID(ctx), campanhaID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var cnt int
		var t *time.Time
		if err := rows.Scan(&cid, &cnt, &t); err != nil {
			continue
		}
		volume[cid] = cnt
		if t != nil {
			last[cid] = *t
		}
	}
	return volume, last, rows.Err()
}

func (s *Store) UpdateGrupoCanais(ctx context.Context, id int, name string, channelIDs []int) error {
	if _, err := s.pool.Exec(ctx, `UPDATE grupo_canais SET name=$1 WHERE tenant_id=$3 AND id=$2`, name, id, TenantID(ctx)); err != nil {
		return err
	}
	return s.setGrupoChannels(ctx, id, channelIDs)
}

func (s *Store) DeleteGrupoCanais(ctx context.Context, id int) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM grupo_canais WHERE tenant_id=$1 AND id=$2`, TenantID(ctx), id)
	return err
}

// ---------- Campanhas ----------

func (s *Store) CreateCampanha(ctx context.Context, c Campanha, contatos []Contato) (int, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	tenantID := TenantID(ctx)
	validContacts := make([]Contato, 0, len(contatos))
	for _, contact := range contatos {
		if strings.TrimSpace(contact.Phone) == "" {
			continue
		}
		if err := s.validateContactReference(ctx, tenantID, contact.ID); err != nil {
			return 0, err
		}
		validContacts = append(validContacts, contact)
	}

	var id int
	err = tx.QueryRow(ctx, `
		INSERT INTO campanhas (tenant_id, name, message, canal, grupo_id, channel_id, segmento_id, status, scheduled_at, intervalo_segundos, limite_diario, variar_mensagem, modo_teste, media_type, media_b64, media_mime, media_name, total, criado_por)
		SELECT $1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19
		WHERE ($5::integer IS NULL OR EXISTS (SELECT 1 FROM grupo_canais WHERE tenant_id=$1 AND id=$5))
		  AND ($6::integer IS NULL OR EXISTS (SELECT 1 FROM channels WHERE tenant_id=$1 AND id=$6))
		  AND ($7::integer IS NULL OR EXISTS (SELECT 1 FROM segmentos WHERE tenant_id=$1 AND id=$7))
		RETURNING id`,
		tenantID, c.Name, c.Message, c.Canal, c.GrupoID, c.ChannelID, c.SegmentoID, c.Status, c.ScheduledAt,
		c.IntervaloSegundos, c.LimiteDiario, c.VariarMensagem, c.ModoTeste,
		c.MediaType, c.MediaB64, c.MediaMime, c.MediaName, len(validContacts), c.CriadoPor).Scan(&id)
	if err != nil {
		return 0, err
	}

	for _, ct := range validContacts {
		phone := strings.TrimSpace(ct.Phone)
		tag, err := tx.Exec(ctx, `INSERT INTO campanha_envios (tenant_id, campanha_id, contato_id, phone)
			SELECT $1,$2,$3,$4 WHERE EXISTS (SELECT 1 FROM contatos WHERE tenant_id=$1 AND id=$3)`, tenantID, id, ct.ID, phone)
		if err != nil {
			return 0, err
		} else if tag.RowsAffected() == 0 {
			return 0, pgx.ErrNoRows
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return id, nil
}

const campanhaSelect = `SELECT c.id, c.name, c.message, c.canal, c.grupo_id, COALESCE(g.name,''), c.channel_id,
	COALESCE(ch.name,''), c.segmento_id, COALESCE(sg.name,''), c.status, c.scheduled_at, c.intervalo_segundos,
	c.limite_diario, c.variar_mensagem, c.modo_teste, c.media_type, c.media_b64, c.media_mime, c.media_name,
	c.total, c.enviadas, c.falhas, c.lidas, c.created_at, c.updated_at, c.started_at, c.completed_at
	FROM campanhas c
	LEFT JOIN grupo_canais g ON g.id = c.grupo_id AND g.tenant_id = c.tenant_id
	LEFT JOIN channels ch ON ch.id = c.channel_id AND ch.tenant_id = c.tenant_id
	LEFT JOIN segmentos sg ON sg.id = c.segmento_id AND sg.tenant_id = c.tenant_id`

func scanCampanha(row interface {
	Scan(...interface{}) error
}) (Campanha, error) {
	var c Campanha
	err := row.Scan(&c.ID, &c.Name, &c.Message, &c.Canal, &c.GrupoID, &c.GrupoName, &c.ChannelID, &c.ChannelName,
		&c.SegmentoID, &c.SegmentoName, &c.Status, &c.ScheduledAt, &c.IntervaloSegundos, &c.LimiteDiario,
		&c.VariarMensagem, &c.ModoTeste, &c.MediaType, &c.MediaB64, &c.MediaMime, &c.MediaName,
		&c.Total, &c.Enviadas, &c.Falhas, &c.Lidas, &c.CreatedAt, &c.UpdatedAt, &c.StartedAt, &c.CompletedAt)
	return c, err
}

func (s *Store) ListCampanhas(ctx context.Context) ([]Campanha, error) {
	rows, err := s.pool.Query(ctx, campanhaSelect+` WHERE c.tenant_id=$1 ORDER BY c.created_at DESC`, TenantID(ctx))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Campanha
	for rows.Next() {
		c, err := scanCampanha(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) GetCampanha(ctx context.Context, id int) (Campanha, error) {
	return scanCampanha(s.pool.QueryRow(ctx, campanhaSelect+` WHERE c.tenant_id=$1 AND c.id=$2`, TenantID(ctx), id))
}

func (s *Store) UpdateCampanhaStatus(ctx context.Context, id int, status string) error {
	_, err := s.pool.Exec(ctx, `UPDATE campanhas SET status=$1, updated_at=now() WHERE tenant_id=$3 AND id=$2`, status, id, TenantID(ctx))
	return err
}

func (s *Store) SetCampanhaStarted(ctx context.Context, id int) error {
	_, err := s.pool.Exec(ctx, `UPDATE campanhas SET started_at=now(), updated_at=now() WHERE tenant_id=$2 AND id=$1 AND started_at IS NULL`, id, TenantID(ctx))
	return err
}

// ListCampanhasToProcess: rodando + agendadas cuja hora chegou.
func (s *Store) ListCampanhasToProcess(ctx context.Context) ([]Campanha, error) {
	tenantID, err := tenantJobID(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, campanhaSelect+`
		WHERE c.tenant_id=$1 AND (c.status='rodando' OR (c.status='agendada' AND c.scheduled_at <= now())) ORDER BY c.id ASC`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Campanha
	for rows.Next() {
		c, err := scanCampanha(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

type PendingEnvio struct {
	ID          int
	Phone       string
	ContatoNome string
}

func (s *Store) ListPendingEnvios(ctx context.Context, campanhaID, limit int) ([]PendingEnvio, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT e.id, e.phone, COALESCE(c.name,'') FROM campanha_envios e
		LEFT JOIN contatos c ON c.id = e.contato_id AND c.tenant_id=e.tenant_id
		WHERE e.tenant_id=$1 AND e.campanha_id=$2 AND e.status='pendente'
		  AND EXISTS (SELECT 1 FROM campanhas c2 WHERE c2.tenant_id=$1 AND c2.id=e.campanha_id)
		ORDER BY e.id ASC LIMIT $3`, TenantID(ctx), campanhaID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PendingEnvio
	for rows.Next() {
		var e PendingEnvio
		if err := rows.Scan(&e.ID, &e.Phone, &e.ContatoNome); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *Store) CountPendingEnvios(ctx context.Context, campanhaID int) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM campanha_envios e
		WHERE e.tenant_id=$1 AND e.campanha_id=$2 AND e.status='pendente'
		  AND EXISTS (SELECT 1 FROM campanhas c WHERE c.tenant_id=$1 AND c.id=e.campanha_id)`, TenantID(ctx), campanhaID).Scan(&n)
	return n, err
}

func (s *Store) CountEnviosSentToday(ctx context.Context, campanhaID int) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM campanha_envios e
		WHERE e.tenant_id=$1 AND e.campanha_id=$2 AND e.status='enviada' AND e.sent_at >= date_trunc('day', now())
		  AND EXISTS (SELECT 1 FROM campanhas c WHERE c.tenant_id=$1 AND c.id=e.campanha_id)`, TenantID(ctx), campanhaID).Scan(&n)
	return n, err
}

func (s *Store) MarkEnvioSent(ctx context.Context, envioID int, channelID int, waMsgID string) error {
	_, err := s.pool.Exec(ctx, `UPDATE campanha_envios SET status='enviada', channel_id=$2, wa_msg_id=$3, sent_at=now()
		WHERE tenant_id=$1 AND id=$4
		  AND EXISTS (SELECT 1 FROM campanhas WHERE tenant_id=$1 AND id=campanha_envios.campanha_id)
		  AND EXISTS (SELECT 1 FROM channels WHERE tenant_id=$1 AND id=$2)`, TenantID(ctx), channelID, waMsgID, envioID)
	return err
}

// MarkEnvioLida marca a mensagem como lida (evento ReadReceipt) e soma no contador da campanha.
func (s *Store) MarkEnvioLida(ctx context.Context, waMsgID string) error {
	_, err := s.pool.Exec(ctx, `
		WITH up AS (
			UPDATE campanha_envios SET lida_at = now()
			WHERE tenant_id = $1 AND wa_msg_id = $2 AND lida_at IS NULL
			RETURNING campanha_id
		)
		UPDATE campanhas SET lidas = lidas + (SELECT COUNT(*) FROM up)
		WHERE tenant_id = $1 AND id = (SELECT campanha_id FROM up LIMIT 1)`, TenantID(ctx), waMsgID)
	return err
}

func (s *Store) MarkEnvioFailed(ctx context.Context, envioID int, channelID *int, erro string) error {
	_, err := s.pool.Exec(ctx, `UPDATE campanha_envios SET status='falha', channel_id=$2, erro=$3, sent_at=now()
		WHERE tenant_id=$1 AND id=$4
		  AND EXISTS (SELECT 1 FROM campanhas WHERE tenant_id=$1 AND id=campanha_envios.campanha_id)
		  AND ($2::integer IS NULL OR EXISTS (SELECT 1 FROM channels WHERE tenant_id=$1 AND id=$2))`, TenantID(ctx), channelID, erro, envioID)
	return err
}

func (s *Store) IncrementCampanhaCounters(ctx context.Context, campanhaID, sent, failed int) error {
	_, err := s.pool.Exec(ctx, `UPDATE campanhas SET enviadas=enviadas+$1, falhas=falhas+$2, updated_at=now() WHERE tenant_id=$4 AND id=$3`, sent, failed, campanhaID, TenantID(ctx))
	return err
}

func (s *Store) MarkCampanhaCompleted(ctx context.Context, campanhaID int) error {
	_, err := s.pool.Exec(ctx, `UPDATE campanhas SET status='concluida', completed_at=now(), updated_at=now() WHERE tenant_id=$2 AND id=$1`, campanhaID, TenantID(ctx))
	return err
}

func (s *Store) DeleteCampanha(ctx context.Context, campanhaID int) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM campanhas WHERE tenant_id=$1 AND id=$2`, TenantID(ctx), campanhaID)
	return err
}

func (s *Store) ListCampanhaEnvios(ctx context.Context, campanhaID, limit int) ([]CampanhaEnvio, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT e.id, e.campanha_id, e.contato_id, e.phone, e.channel_id, COALESCE(ch.name,''), e.status, e.erro,
			e.wa_msg_id, e.lida_at, e.created_at, e.sent_at
		FROM campanha_envios e LEFT JOIN channels ch ON ch.id = e.channel_id AND ch.tenant_id=e.tenant_id
		WHERE e.tenant_id=$1 AND e.campanha_id=$2
		  AND EXISTS (SELECT 1 FROM campanhas c WHERE c.tenant_id=$1 AND c.id=e.campanha_id)
		ORDER BY e.id DESC LIMIT $3`, TenantID(ctx), campanhaID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CampanhaEnvio
	for rows.Next() {
		var e CampanhaEnvio
		if err := rows.Scan(&e.ID, &e.CampanhaID, &e.ContatoID, &e.Phone, &e.ChannelID, &e.ChannelName, &e.Status, &e.Erro,
			&e.WAMsgID, &e.LidaAt, &e.CreatedAt, &e.SentAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *Store) ListCampanhaEnviosPage(ctx context.Context, campanhaID int, status string, limit, offset int) ([]CampanhaEnvio, int, error) {
	statusFilter := strings.TrimSpace(status)
	var total int
	if err := s.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM campanha_envios e
		WHERE e.tenant_id=$1 AND e.campanha_id=$2 AND ($3::text = '' OR e.status=$3)`,
		TenantID(ctx), campanhaID, statusFilter).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.pool.Query(ctx, `
		SELECT e.id, e.campanha_id, e.contato_id, e.phone, e.channel_id, COALESCE(ch.name,''), e.status, e.erro,
			e.wa_msg_id, e.lida_at, e.created_at, e.sent_at
		FROM campanha_envios e LEFT JOIN channels ch ON ch.id = e.channel_id AND ch.tenant_id=e.tenant_id
		WHERE e.tenant_id=$1 AND e.campanha_id=$2 AND ($3::text = '' OR e.status=$3)
		  AND EXISTS (SELECT 1 FROM campanhas c WHERE c.tenant_id=$1 AND c.id=e.campanha_id)
		ORDER BY e.id DESC LIMIT $4 OFFSET $5`, TenantID(ctx), campanhaID, statusFilter, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []CampanhaEnvio
	for rows.Next() {
		var e CampanhaEnvio
		if err := rows.Scan(&e.ID, &e.CampanhaID, &e.ContatoID, &e.Phone, &e.ChannelID, &e.ChannelName, &e.Status, &e.Erro,
			&e.WAMsgID, &e.LidaAt, &e.CreatedAt, &e.SentAt); err != nil {
			return nil, 0, err
		}
		out = append(out, e)
	}
	return out, total, rows.Err()
}
