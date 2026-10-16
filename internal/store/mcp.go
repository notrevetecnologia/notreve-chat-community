package store

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/notrevetecnologia/notreve-chat-community/internal/securesecret"
)

const mcpServerCols = `id, tenant_id, nome, descricao, transport, command, args, env, url, headers, ativo, conectado, ultimo_erro, server_name, server_version, capabilities, ultima_conexao, ultima_sincronizacao, created_at, updated_at`
const mcpToolCols = `id, tenant_id, server_id, name, display_name, description, input_schema, ativa, created_at, updated_at`

func decodeJSONMap(raw []byte) map[string]interface{} {
	out := map[string]interface{}{}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &out)
	}
	return out
}

func decodeJSONStringMap(raw []byte) map[string]string {
	out := map[string]string{}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &out)
	}
	return out
}

func decodeJSONStringSlice(raw []byte) []string {
	out := []string{}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &out)
	}
	return out
}

func encodeJSON(value interface{}) []byte {
	encoded, err := json.Marshal(value)
	if err != nil {
		return []byte("{}")
	}
	return encoded
}

func scanMCPServer(scanner skillScanner) (MCPServer, error) {
	var srv MCPServer
	var args, env, headers, capabilities []byte
	err := scanner.Scan(&srv.ID, &srv.TenantID, &srv.Nome, &srv.Descricao, &srv.Transport, &srv.Command,
		&args, &env, &srv.URL, &headers, &srv.Ativo, &srv.Conectado, &srv.UltimoErro, &srv.ServerName, &srv.ServerVersion,
		&capabilities, &srv.UltimaConexao, &srv.UltimaSincronizacao, &srv.CreatedAt, &srv.UpdatedAt)
	if err != nil {
		return srv, err
	}
	srv.Args = decodeJSONStringSlice(args)
	srv.Env = decodeJSONStringMap(env)
	srv.Headers = decodeJSONStringMap(headers)
	srv.Capabilities = decodeJSONMap(capabilities)
	return srv, nil
}

func scanMCPTool(scanner skillScanner) (MCPTool, error) {
	var tool MCPTool
	var schema []byte
	err := scanner.Scan(&tool.ID, &tool.TenantID, &tool.ServerID, &tool.Name, &tool.DisplayName, &tool.Description, &schema, &tool.Ativa, &tool.CreatedAt, &tool.UpdatedAt)
	if err != nil {
		return tool, err
	}
	tool.InputSchema = decodeJSONMap(schema)
	return tool, nil
}

func (s *Store) ListMCPServers(ctx context.Context) ([]MCPServer, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+mcpServerCols+` FROM mcp_servers WHERE tenant_id=$1 ORDER BY nome ASC`, TenantID(ctx))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MCPServer
	for rows.Next() {
		srv, err := scanMCPServer(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, srv)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	links, err := s.mcpServerAgentLinks(ctx)
	if err != nil {
		return nil, err
	}
	counts, err := s.mcpServerToolCounts(ctx)
	if err != nil {
		return nil, err
	}
	for i := range out {
		out[i].AgentIDs = links[out[i].ID]
		out[i].ToolCount = counts[out[i].ID]
	}
	return out, nil
}

func (s *Store) GetMCPServer(ctx context.Context, id int) (MCPServer, error) {
	srv, err := scanMCPServer(s.pool.QueryRow(ctx, `SELECT `+mcpServerCols+` FROM mcp_servers WHERE tenant_id=$1 AND id=$2`, TenantID(ctx), id))
	if err != nil {
		return srv, err
	}
	links, err := s.mcpServerAgentLinks(ctx)
	if err != nil {
		return srv, err
	}
	srv.AgentIDs = links[srv.ID]
	counts, err := s.mcpServerToolCounts(ctx)
	if err != nil {
		return srv, err
	}
	srv.ToolCount = counts[srv.ID]
	return srv, nil
}

func (s *Store) CreateMCPServer(ctx context.Context, srv MCPServer) (int, error) {
	tenantID := TenantID(ctx)
	if err := s.validateAgentReferences(ctx, tenantID, srv.AgentIDs); err != nil {
		return 0, err
	}
	var id int
	err := s.pool.QueryRow(ctx,
		`INSERT INTO mcp_servers (tenant_id, nome, descricao, transport, command, args, env, url, headers, ativo, conectado, ultimo_erro)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,false,'') RETURNING id`,
		tenantID, srv.Nome, srv.Descricao, srv.Transport, srv.Command, encodeJSON(srv.Args), encodeJSON(srv.Env),
		srv.URL, encodeJSON(srv.Headers), srv.Ativo).Scan(&id)
	if err != nil {
		return 0, err
	}
	if err := s.SetMCPServerAgents(ctx, id, srv.AgentIDs); err != nil {
		return id, err
	}
	return id, nil
}

func (s *Store) UpdateMCPServer(ctx context.Context, srv MCPServer) error {
	tenantID := TenantID(ctx)
	if err := s.validateAgentReferences(ctx, tenantID, srv.AgentIDs); err != nil {
		return err
	}
	tag, err := s.pool.Exec(ctx,
		`UPDATE mcp_servers SET nome=$1, descricao=$2, transport=$3, command=$4, args=$5, env=$6, url=$7, headers=$8, ativo=$9, updated_at=now()
		 WHERE tenant_id=$10 AND id=$11`,
		srv.Nome, srv.Descricao, srv.Transport, srv.Command, encodeJSON(srv.Args), encodeJSON(srv.Env),
		srv.URL, encodeJSON(srv.Headers), srv.Ativo, tenantID, srv.ID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return s.SetMCPServerAgents(ctx, srv.ID, srv.AgentIDs)
}

func (s *Store) DeleteMCPServer(ctx context.Context, id int) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM mcp_servers WHERE tenant_id=$1 AND id=$2`, TenantID(ctx), id)
	return err
}

func (s *Store) SetMCPServerAgents(ctx context.Context, serverID int, agentIDs []int) error {
	tenantID := TenantID(ctx)
	if _, err := s.pool.Exec(ctx, `DELETE FROM agent_mcp_servers WHERE tenant_id=$1 AND server_id=$2`, tenantID, serverID); err != nil {
		return err
	}
	for _, agentID := range uniqueInts(agentIDs) {
		if _, err := s.pool.Exec(ctx,
			`INSERT INTO agent_mcp_servers (tenant_id, agent_id, server_id) VALUES ($1,$2,$3)`,
			tenantID, agentID, serverID); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) ListMCPServerTools(ctx context.Context, serverID int) ([]MCPTool, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+mcpToolCols+` FROM mcp_tools WHERE tenant_id=$1 AND server_id=$2 ORDER BY name ASC`, TenantID(ctx), serverID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MCPTool
	for rows.Next() {
		tool, err := scanMCPTool(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, tool)
	}
	return out, rows.Err()
}

// ReplaceMCPServerTools sincroniza as tools de um servidor, preservando o estado
// "ativa" das que já existiam e removendo as que sumiram do servidor.
func (s *Store) ReplaceMCPServerTools(ctx context.Context, serverID int, tools []MCPTool) error {
	tenantID := TenantID(ctx)
	var serverExists bool
	if err := s.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM mcp_servers WHERE tenant_id=$1 AND id=$2)`, tenantID, serverID).Scan(&serverExists); err != nil {
		return err
	}
	if !serverExists {
		return pgx.ErrNoRows
	}
	keep := make([]string, 0, len(tools))
	for _, tool := range tools {
		name := strings.TrimSpace(tool.Name)
		if name == "" {
			continue
		}
		keep = append(keep, name)
		if _, err := s.pool.Exec(ctx,
			`INSERT INTO mcp_tools (tenant_id, server_id, name, display_name, description, input_schema, ativa, updated_at)
			 VALUES ($1,$2,$3,$4,$5,$6,true,now())
			 ON CONFLICT (server_id, name) DO UPDATE SET display_name=EXCLUDED.display_name, description=EXCLUDED.description, input_schema=EXCLUDED.input_schema, updated_at=now()`,
			tenantID, serverID, name, tool.DisplayName, tool.Description, encodeJSON(tool.InputSchema)); err != nil {
			return err
		}
	}
	if len(keep) == 0 {
		_, err := s.pool.Exec(ctx, `DELETE FROM mcp_tools WHERE tenant_id=$1 AND server_id=$2`, tenantID, serverID)
		return err
	}
	_, err := s.pool.Exec(ctx, `DELETE FROM mcp_tools WHERE tenant_id=$1 AND server_id=$2 AND name <> ALL($3)`, tenantID, serverID, keep)
	return err
}

// ListAgentMCPTools devolve as tools ativas dos servidores ativos vinculados ao
// agente, já com os dados de conexão e os segredos decifrados para uso em runtime.
func (s *Store) ListAgentMCPTools(ctx context.Context, agentID int) ([]MCPTool, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT t.id, t.tenant_id, t.server_id, t.name, t.display_name, t.description, t.input_schema, t.ativa, t.created_at, t.updated_at,
		        srv.nome, srv.transport, srv.command, srv.args, srv.env, srv.url, srv.headers
		 FROM mcp_tools t
		 JOIN mcp_servers srv ON srv.id = t.server_id AND srv.tenant_id = t.tenant_id
		 JOIN agent_mcp_servers a ON a.server_id = srv.id AND a.tenant_id = srv.tenant_id
		 WHERE t.tenant_id=$1 AND a.agent_id=$2 AND t.ativa=true AND srv.ativo=true
		 ORDER BY srv.nome ASC, t.name ASC`,
		TenantID(ctx), agentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MCPTool
	for rows.Next() {
		var tool MCPTool
		var schema, args, env, headers []byte
		if err := rows.Scan(&tool.ID, &tool.TenantID, &tool.ServerID, &tool.Name, &tool.DisplayName, &tool.Description, &schema, &tool.Ativa, &tool.CreatedAt, &tool.UpdatedAt,
			&tool.ServerNome, &tool.Transport, &tool.Command, &args, &env, &tool.URL, &headers); err != nil {
			return nil, err
		}
		tool.InputSchema = decodeJSONMap(schema)
		tool.Args = decodeJSONStringSlice(args)
		tool.Env = decodeJSONStringMap(env)
		tool.Headers = decodeJSONStringMap(headers)
		decrypted, err := decryptMCPSecretMap(tool.Env, "mcp.env.")
		if err != nil {
			return nil, err
		}
		tool.Env = decrypted
		decrypted, err = decryptMCPSecretMap(tool.Headers, "mcp.headers.")
		if err != nil {
			return nil, err
		}
		tool.Headers = decrypted
		out = append(out, tool)
	}
	return out, rows.Err()
}

func (s *Store) MarkMCPServerStatus(ctx context.Context, id int, connected bool, serverName, serverVersion string, capabilities map[string]interface{}, errMsg string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE mcp_servers SET conectado=$1, server_name=$2, server_version=$3, capabilities=$4, ultimo_erro=$5,
		        ultima_conexao=now(), ultima_sincronizacao=CASE WHEN $1 THEN now() ELSE ultima_sincronizacao END, updated_at=now()
		 WHERE tenant_id=$6 AND id=$7`,
		connected, serverName, serverVersion, encodeJSON(capabilities), errMsg, TenantID(ctx), id)
	return err
}

func (s *Store) mcpServerAgentLinks(ctx context.Context) (map[int][]int, error) {
	rows, err := s.pool.Query(ctx, `SELECT server_id, agent_id FROM agent_mcp_servers WHERE tenant_id=$1 ORDER BY agent_id ASC`, TenantID(ctx))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int][]int{}
	for rows.Next() {
		var serverID, agentID int
		if err := rows.Scan(&serverID, &agentID); err != nil {
			return nil, err
		}
		out[serverID] = append(out[serverID], agentID)
	}
	return out, rows.Err()
}

func (s *Store) mcpServerToolCounts(ctx context.Context) (map[int]int, error) {
	rows, err := s.pool.Query(ctx, `SELECT server_id, COUNT(*) FROM mcp_tools WHERE tenant_id=$1 GROUP BY server_id`, TenantID(ctx))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int]int{}
	for rows.Next() {
		var serverID, count int
		if err := rows.Scan(&serverID, &count); err != nil {
			return nil, err
		}
		out[serverID] = count
	}
	return out, rows.Err()
}

func mcpSecretKey(name string) bool {
	return integrationSecretHeader(name)
}

func transformMCPSecretMap(values map[string]string, scopePrefix string, decrypt bool) (map[string]string, error) {
	if len(values) == 0 {
		return map[string]string{}, nil
	}
	out := make(map[string]string, len(values))
	for name, value := range values {
		if !mcpSecretKey(name) || strings.TrimSpace(value) == "" {
			out[name] = value
			continue
		}
		scope := scopePrefix + strings.ToLower(strings.TrimSpace(name))
		var err error
		if decrypt {
			out[name], err = securesecret.Decrypt(scope, value)
		} else {
			out[name], err = securesecret.Encrypt(scope, value)
		}
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

func decryptMCPSecretMap(values map[string]string, scopePrefix string) (map[string]string, error) {
	return transformMCPSecretMap(values, scopePrefix, true)
}

func encryptMCPServerSecrets(srv MCPServer) (MCPServer, error) {
	env, err := transformMCPSecretMap(srv.Env, "mcp.env.", false)
	if err != nil {
		return MCPServer{}, err
	}
	headers, err := transformMCPSecretMap(srv.Headers, "mcp.headers.", false)
	if err != nil {
		return MCPServer{}, err
	}
	srv.Env = env
	srv.Headers = headers
	return srv, nil
}

func (s *Store) GetMCPServerSecure(ctx context.Context, id int) (MCPServer, error) {
	srv, err := s.GetMCPServer(ctx, id)
	if err != nil {
		return MCPServer{}, err
	}
	env, err := decryptMCPSecretMap(srv.Env, "mcp.env.")
	if err != nil {
		return MCPServer{}, err
	}
	headers, err := decryptMCPSecretMap(srv.Headers, "mcp.headers.")
	if err != nil {
		return MCPServer{}, err
	}
	srv.Env = env
	srv.Headers = headers
	return srv, nil
}

func (s *Store) CreateMCPServerSecure(ctx context.Context, srv MCPServer) (int, error) {
	stored, err := encryptMCPServerSecrets(srv)
	if err != nil {
		return 0, err
	}
	return s.CreateMCPServer(ctx, stored)
}

func (s *Store) UpdateMCPServerSecure(ctx context.Context, srv MCPServer) error {
	stored, err := encryptMCPServerSecrets(srv)
	if err != nil {
		return err
	}
	return s.UpdateMCPServer(ctx, stored)
}
