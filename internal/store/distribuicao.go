package store

import (
	"context"
	"strings"
)

// SetAtendenteHabilidades substitui as habilidades de um atendente.
func (s *Store) SetAtendenteHabilidades(ctx context.Context, atendenteID int, habilidades []string) error {
	skills := normalizeSkillList(habilidades)
	_, err := s.pool.Exec(ctx, `UPDATE atendentes SET habilidades=$1 WHERE tenant_id=$2 AND id=$3`,
		skills, TenantID(ctx), atendenteID)
	return err
}

// GetAtendenteHabilidades retorna as habilidades de um atendente.
func (s *Store) GetAtendenteHabilidades(ctx context.Context, atendenteID int) ([]string, error) {
	var skills []string
	if err := s.pool.QueryRow(ctx,
		`SELECT COALESCE(habilidades, '{}') FROM atendentes WHERE tenant_id=$1 AND id=$2`,
		TenantID(ctx), atendenteID).Scan(&skills); err != nil {
		return nil, err
	}
	return skills, nil
}

// ListAtendentesHabilidades retorna um mapa atendente_id -> habilidades.
func (s *Store) ListAtendentesHabilidades(ctx context.Context) (map[int][]string, error) {
	out := map[int][]string{}
	rows, err := s.pool.Query(ctx, `SELECT id, COALESCE(habilidades, '{}') FROM atendentes WHERE tenant_id=$1`, TenantID(ctx))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id int
		var skills []string
		if err := rows.Scan(&id, &skills); err != nil {
			return nil, err
		}
		out[id] = skills
	}
	return out, rows.Err()
}

// LastCustomerMessageForConversation retorna o conteúdo da última mensagem do
// cliente numa conversa (usado para detectar o nicho/habilidade exigida).
func (s *Store) LastCustomerMessageForConversation(ctx context.Context, conversationID int) (string, error) {
	var content string
	err := s.pool.QueryRow(ctx, `
		SELECT COALESCE(m.content, '') FROM messages m
		WHERE m.tenant_id=$1 AND m.conversation_id=$2 AND m.role IN ('user','customer')
		ORDER BY m.id DESC LIMIT 1`, TenantID(ctx), conversationID).Scan(&content)
	return content, err
}

// normalizeSkillList normaliza uma lista de habilidades (trim, sem duplicatas).
func normalizeSkillList(skills []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(skills))
	for _, skill := range skills {
		skill = strings.ToLower(strings.TrimSpace(skill))
		if skill == "" || seen[skill] {
			continue
		}
		seen[skill] = true
		out = append(out, skill)
	}
	return out
}

// SkillsOfMerge junta as habilidades dos atendentes disponíveis num conjunto.
func SkillsOfMerge(cands []AtendenteDisponivel) []string {
	seen := map[string]bool{}
	var out []string
	for _, cand := range cands {
		for _, skill := range cand.Habilidades {
			skill = strings.ToLower(strings.TrimSpace(skill))
			if skill == "" || seen[skill] {
				continue
			}
			seen[skill] = true
			out = append(out, skill)
		}
	}
	return out
}

// RequiredSkillsForConversation detecta as habilidades exigidas por uma conversa
// a partir do nome do setor e da última mensagem do cliente, usando as
// habilidades conhecidas dos atendentes como matcher.
func RequiredSkillsForConversation(c Conversation, lastMessage string, knownSkills []string) []string {
	text := strings.ToLower(strings.TrimSpace(c.SetorName + " " + lastMessage))
	if text == "" {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	for _, skill := range knownSkills {
		s := strings.ToLower(strings.TrimSpace(skill))
		if s == "" || seen[s] {
			continue
		}
		if skillsMatch(s, text) {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

// skillsMatch verifica se o termo de habilidade aparece no texto (palavra ou
// parte contida).
func skillsMatch(skill, text string) bool {
	if skill == "" || text == "" {
		return false
	}
	if strings.Contains(text, skill) {
		return true
	}
	for _, word := range strings.Fields(text) {
		if word == skill || strings.Trim(word, ".,;:!?") == skill {
			return true
		}
	}
	return false
}
