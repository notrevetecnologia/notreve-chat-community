package store

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type LogFilter struct {
	Level  string
	Module string
	Search string
	From   *time.Time
	To     *time.Time
}

const logEntryCols = `id, tenant_id, ts, level, module, message, extra, traceback, request_id`

func scanLogEntry(row pgx.Row) (LogEntry, error) {
	var e LogEntry
	var extra []byte
	err := row.Scan(&e.ID, &e.TenantID, &e.Ts, &e.Level, &e.Module, &e.Message, &extra, &e.Traceback, &e.RequestID)
	if err != nil {
		return e, err
	}
	e.Extra = map[string]interface{}{}
	if len(extra) > 0 {
		_ = json.Unmarshal(extra, &e.Extra)
	}
	return e, nil
}

func logFilterClause(args []interface{}, filter LogFilter) (string, []interface{}) {
	query := ""
	if level := strings.ToUpper(strings.TrimSpace(filter.Level)); level != "" {
		args = append(args, level)
		query += ` AND level=$` + strconv.Itoa(len(args))
	}
	if module := strings.TrimSpace(filter.Module); module != "" {
		args = append(args, module)
		query += ` AND module=$` + strconv.Itoa(len(args))
	}
	if search := strings.TrimSpace(filter.Search); search != "" {
		args = append(args, "%"+search+"%")
		placeholder := `$` + strconv.Itoa(len(args))
		query += ` AND (lower(message) LIKE lower(` + placeholder + `) OR lower(module) LIKE lower(` + placeholder + `) OR lower(traceback) LIKE lower(` + placeholder + `))`
	}
	if filter.From != nil {
		args = append(args, *filter.From)
		query += ` AND ts >= $` + strconv.Itoa(len(args))
	}
	if filter.To != nil {
		args = append(args, *filter.To)
		query += ` AND ts <= $` + strconv.Itoa(len(args))
	}
	return query, args
}

func (s *Store) InsertLogEntry(ctx context.Context, entry LogEntry) error {
	tenantID := TenantID(ctx)
	if tenantID < 1 {
		return ErrTenantContextRequired
	}
	extra := entry.Extra
	if extra == nil {
		extra = map[string]interface{}{}
	}
	raw, err := json.Marshal(extra)
	if err != nil {
		raw = []byte("{}")
	}
	_, err = s.pool.Exec(ctx,
		`INSERT INTO log_entries (tenant_id, level, module, message, extra, traceback, request_id)
		 VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		tenantID, entry.Level, entry.Module, entry.Message, raw, entry.Traceback, entry.RequestID)
	return err
}

func (s *Store) ListLogEntriesForTenant(ctx context.Context, filter LogFilter, limit, offset int) ([]LogEntry, error) {
	args := []interface{}{TenantID(ctx)}
	clause, args := logFilterClause(args, filter)
	args = append(args, limit, offset)
	query := `SELECT ` + logEntryCols + ` FROM log_entries WHERE tenant_id=$1` + clause +
		` ORDER BY ts DESC, id DESC LIMIT $` + strconv.Itoa(len(args)-1) + ` OFFSET $` + strconv.Itoa(len(args))
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []LogEntry
	for rows.Next() {
		entry, err := scanLogEntry(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, entry)
	}
	return out, rows.Err()
}

func (s *Store) CountLogEntriesForTenant(ctx context.Context, filter LogFilter) (int, error) {
	args := []interface{}{TenantID(ctx)}
	clause, args := logFilterClause(args, filter)
	var total int
	err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM log_entries WHERE tenant_id=$1`+clause, args...).Scan(&total)
	return total, err
}

func (s *Store) LogStatsForTenant(ctx context.Context) (map[string]interface{}, error) {
	tenantID := TenantID(ctx)
	var total int
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM log_entries WHERE tenant_id=$1`, tenantID).Scan(&total); err != nil {
		return nil, err
	}
	byLevel := map[string]int{}
	rows, err := s.pool.Query(ctx, `SELECT level, COUNT(*) FROM log_entries WHERE tenant_id=$1 GROUP BY level`, tenantID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var level string
		var count int
		if err := rows.Scan(&level, &count); err != nil {
			rows.Close()
			return nil, err
		}
		byLevel[level] = count
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	byModule := map[string]int{}
	rows, err = s.pool.Query(ctx, `SELECT module, COUNT(*) FROM log_entries WHERE tenant_id=$1 GROUP BY module`, tenantID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var module string
		var count int
		if err := rows.Scan(&module, &count); err != nil {
			rows.Close()
			return nil, err
		}
		byModule[module] = count
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	var errorsLastHour int
	if err := s.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM log_entries WHERE tenant_id=$1 AND level IN ('ERROR','CRITICAL') AND ts >= now() - interval '1 hour'`,
		tenantID).Scan(&errorsLastHour); err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"total": total, "by_level": byLevel, "by_module": byModule, "errors_last_hour": errorsLastHour,
	}, nil
}

func (s *Store) DeleteOldLogEntriesForTenant(ctx context.Context, olderThan time.Time) (int64, error) {
	tag, err := s.pool.Exec(ctx, `DELETE FROM log_entries WHERE tenant_id=$1 AND ts < $2`, TenantID(ctx), olderThan)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}
