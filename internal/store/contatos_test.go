package store

import "testing"

func TestNormalizePhoneWithDDI(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"(11) 99999-9999", "5511999999999"},
		{"011 99999-9999", "5511999999999"},
		{"5511999999999", "5511999999999"},
		{"+55 11 99999-9999", "5511999999999"},
		{"0005511999999999", "5511999999999"},
		{"11 8888-8888", "551188888888"},
		{"abc", ""},
		{"", ""},
	}
	for _, c := range cases {
		if got := NormalizePhoneWithDDI(c.in); got != c.want {
			t.Errorf("NormalizePhoneWithDDI(%q)=%q, want %q", c.in, got, c.want)
		}
	}
}

func TestSkillsMatch(t *testing.T) {
	text := "preciso de ajuda com cobranca financeiro"
	if !skillsMatch("financeiro", text) {
		t.Errorf("esperava match de skill financeiro")
	}
	if skillsMatch("tecnico", text) {
		t.Errorf("nao deveria dar match de tecnico")
	}
}

func TestRequiredSkillsForConversation(t *testing.T) {
	convs := Conversation{SetorName: "Financeiro"}
	if got := RequiredSkillsForConversation(convs, "", []string{"financeiro", "tecnico"}); len(got) != 1 || got[0] != "financeiro" {
		t.Errorf("esperava apenas financeiro, obteve %v", got)
	}
	if got := RequiredSkillsForConversation(Conversation{}, "", []string{"financeiro"}); len(got) != 0 {
		t.Errorf("sem contexto, nao deveria haver skill exigida, obteve %v", got)
	}
}
