package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

const skillCols = `id, tenant_id, nome, descricao, instrucao_completa, categoria, ativa, created_at, updated_at`

type skillScanner interface {
	Scan(dest ...interface{}) error
}

func scanSkill(scanner skillScanner) (Skill, error) {
	var sk Skill
	err := scanner.Scan(&sk.ID, &sk.TenantID, &sk.Nome, &sk.Descricao, &sk.InstrucaoCompleta, &sk.Categoria, &sk.Ativa, &sk.CreatedAt, &sk.UpdatedAt)
	return sk, err
}

func (s *Store) ListSkills(ctx context.Context) ([]Skill, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+skillCols+` FROM skills WHERE tenant_id=$1 ORDER BY categoria ASC, nome ASC`, TenantID(ctx))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Skill
	for rows.Next() {
		sk, err := scanSkill(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, sk)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	links, err := s.skillAgentLinks(ctx)
	if err != nil {
		return nil, err
	}
	for i := range out {
		out[i].AgentIDs = links[out[i].ID]
	}
	return out, nil
}

func (s *Store) GetSkill(ctx context.Context, id int) (Skill, error) {
	sk, err := scanSkill(s.pool.QueryRow(ctx, `SELECT `+skillCols+` FROM skills WHERE tenant_id=$1 AND id=$2`, TenantID(ctx), id))
	if err != nil {
		return sk, err
	}
	links, err := s.skillAgentLinks(ctx)
	if err != nil {
		return sk, err
	}
	sk.AgentIDs = links[sk.ID]
	return sk, nil
}

func (s *Store) CreateSkill(ctx context.Context, sk Skill) (int, error) {
	tenantID := TenantID(ctx)
	if err := s.validateAgentReferences(ctx, tenantID, sk.AgentIDs); err != nil {
		return 0, err
	}
	var id int
	err := s.pool.QueryRow(ctx,
		`INSERT INTO skills (tenant_id, nome, descricao, instrucao_completa, categoria, ativa)
		 VALUES ($1,$2,$3,$4,$5,$6) RETURNING id`,
		tenantID, sk.Nome, sk.Descricao, sk.InstrucaoCompleta, sk.Categoria, sk.Ativa).Scan(&id)
	if err != nil {
		return 0, err
	}
	if err := s.SetSkillAgents(ctx, id, sk.AgentIDs); err != nil {
		return id, err
	}
	return id, nil
}

func (s *Store) UpdateSkill(ctx context.Context, sk Skill) error {
	tenantID := TenantID(ctx)
	if err := s.validateAgentReferences(ctx, tenantID, sk.AgentIDs); err != nil {
		return err
	}
	tag, err := s.pool.Exec(ctx,
		`UPDATE skills SET nome=$1, descricao=$2, instrucao_completa=$3, categoria=$4, ativa=$5, updated_at=now()
		 WHERE tenant_id=$6 AND id=$7`,
		sk.Nome, sk.Descricao, sk.InstrucaoCompleta, sk.Categoria, sk.Ativa, tenantID, sk.ID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return s.SetSkillAgents(ctx, sk.ID, sk.AgentIDs)
}

func (s *Store) DeleteSkill(ctx context.Context, id int) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM skills WHERE tenant_id=$1 AND id=$2`, TenantID(ctx), id)
	return err
}

func (s *Store) SetSkillAgents(ctx context.Context, skillID int, agentIDs []int) error {
	tenantID := TenantID(ctx)
	if _, err := s.pool.Exec(ctx, `DELETE FROM agent_skills WHERE tenant_id=$1 AND skill_id=$2`, tenantID, skillID); err != nil {
		return err
	}
	for position, agentID := range uniqueInts(agentIDs) {
		if _, err := s.pool.Exec(ctx,
			`INSERT INTO agent_skills (tenant_id, agent_id, skill_id, posicao, ativa) VALUES ($1,$2,$3,$4,true)`,
			tenantID, agentID, skillID, position); err != nil {
			return err
		}
	}
	return nil
}

// ListAgentSkills devolve as skills ativas vinculadas ao agente, na ordem definida.
func (s *Store) ListAgentSkills(ctx context.Context, agentID int) ([]Skill, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT s.id, s.tenant_id, s.nome, s.descricao, s.instrucao_completa, s.categoria, s.ativa, s.created_at, s.updated_at
		 FROM skills s
		 JOIN agent_skills a ON a.skill_id = s.id AND a.tenant_id = s.tenant_id
		 WHERE s.tenant_id=$1 AND a.agent_id=$2 AND a.ativa=true AND s.ativa=true
		 ORDER BY a.posicao ASC, s.nome ASC`,
		TenantID(ctx), agentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Skill
	for rows.Next() {
		sk, err := scanSkill(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, sk)
	}
	return out, rows.Err()
}

func (s *Store) skillAgentLinks(ctx context.Context) (map[int][]int, error) {
	rows, err := s.pool.Query(ctx, `SELECT skill_id, agent_id FROM agent_skills WHERE tenant_id=$1 ORDER BY posicao ASC`, TenantID(ctx))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int][]int{}
	for rows.Next() {
		var skillID, agentID int
		if err := rows.Scan(&skillID, &agentID); err != nil {
			return nil, err
		}
		out[skillID] = append(out[skillID], agentID)
	}
	return out, rows.Err()
}

func (s *Store) validateAgentReferences(ctx context.Context, tenantID int, agentIDs []int) error {
	ids := uniqueInts(agentIDs)
	if len(ids) == 0 {
		return nil
	}
	var count int
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM agents WHERE tenant_id=$1 AND id=ANY($2)`, tenantID, ids).Scan(&count); err != nil {
		return err
	}
	if count != len(ids) {
		return errors.New("agent does not belong to tenant")
	}
	return nil
}

type defaultSkill struct {
	Nome      string
	Descricao string
	Categoria string
	Instrucao string
}

func defaultSkills() []defaultSkill {
	return []defaultSkill{
		{
			Nome:      "atendimento",
			Descricao: "Fluxo de atendimento ao cliente: acolher, entender, resolver e confirmar.",
			Categoria: "atendimento",
			Instrucao: `ATENDIMENTO AO CLIENTE
1. Acolha a pessoa pelo nome (se já souber) e demonstre que entendeu o pedido.
2. Entenda a necessidade fazendo UMA pergunta objetiva por vez; nunca dispare um questionário.
3. Consulte as informações da empresa e as tools antes de responder sobre dados reais.
4. Resolva o que estiver ao seu alcance; se depender de um setor humano, siga a regra de transferência.
5. Confirme a solução, pergunte se ficou alguma dúvida e só então ofereça encerrar.
Tom: próximo, claro e humano. Nunca diga que é uma IA.`,
		},
		{
			Nome:      "vendas",
			Descricao: "Condução consultiva de vendas: qualificar, apresentar valor e conduzir ao fechamento.",
			Categoria: "comercial",
			Instrucao: `VENDAS CONSULTIVAS
1. Qualifique: entenda a necessidade, o contexto e o que motivou o contato.
2. Não empilhe preços nem catálogos; ofereça apenas o que resolve o problema relatado, usando dados reais da empresa.
3. Apresente valor antes de preço e destaque o benefício para o cliente.
4. Trate objeções com empatia, sem pressionar e sem prometer o que não existe.
5. Conduza para o próximo passo (proposta, contratação, contato comercial) com uma pergunta clara.
6. Nunca invente preço, produto ou condição. Se faltar informação, diga que vai confirmar com o time e use a transferência operacional.`,
		},
		{
			Nome:      "suporte",
			Descricao: "Suporte técnico: diagnosticar, orientar passo a passo e escalar quando necessário.",
			Categoria: "suporte",
			Instrucao: `SUPORTE TÉCNICO
1. Colete o sintoma exato, quando começou e o que já foi tentado (uma pergunta por vez).
2. Reproduza mentalmente o cenário e explique a solução em passos curtos e numerados, em linguagem simples.
3. Peça confirmação de cada passo antes de seguir para o próximo.
4. Se o problema exigir acesso a sistemas internos ou reenvio/conferência, colete os dados mínimos e transfira para o setor de Suporte com o marcador [SETOR:Suporte].
5. Nunca prometa prazo ou solução que não esteja confirmada; registre claramente o que foi orientado.`,
		},
	}
}

// EnsureDefaultSkills garante as skills padrão do tenant. É idempotente: só cria
// quando o tenant ainda não tem nenhuma skill, preservando as edições do dono.
func (s *Store) EnsureDefaultSkills(ctx context.Context) error {
	var count int
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM skills WHERE tenant_id=$1`, TenantID(ctx)).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	for _, item := range defaultSkills() {
		if _, err := s.pool.Exec(ctx,
			`INSERT INTO skills (tenant_id, nome, descricao, instrucao_completa, categoria, ativa)
			 VALUES ($1,$2,$3,$4,$5,true) ON CONFLICT (tenant_id, nome) DO NOTHING`,
			TenantID(ctx), item.Nome, item.Descricao, item.Instrucao, item.Categoria); err != nil {
			return fmt.Errorf("seed skill %q: %w", item.Nome, err)
		}
	}
	return nil
}
