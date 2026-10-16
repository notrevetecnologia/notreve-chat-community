package channel

import "testing"

func TestWhatsappSanitizeMarkdownMapping(t *testing.T) {
	casos := []struct {
		nome string
		in   string
		want string
	}{
		{"negrito duplo", "Olá **mundo**", "Olá *mundo*"},
		{"negrito underline", "Olá __mundo__", "Olá *mundo*"},
		{"negrito whatsapp preservado", "Olá *mundo*", "Olá *mundo*"},
		{"itálico whatsapp preservado", "Olá _mundo_", "Olá _mundo_"},
		{"negrito e itálico", "***forte***", "*_forte_*"},
		{"tachado", "~~fora~~", "~fora~"},
		{"cabeçalho mantém texto", "# Título do dia", "*Título do dia*"},
		{"link", "veja [o site](https://exemplo.com)", "veja o site (https://exemplo.com)"},
		{"imagem", "![logo](https://exemplo.com/l.png)", "logo - https://exemplo.com/l.png"},
		{"linha", "antes\n---\ndepois", "antes\n─────────────────\ndepois"},
		{"citação preserva texto", "> importante", "importante"},
		{"código inline preservado", "use `**x**` aqui", "use `**x**` aqui"},
		{"bloco de código preservado", "```\n**não mexer**\n```", "```\n**não mexer**\n```"},
	}
	for _, caso := range casos {
		if got := whatsappSanitize(caso.in); got != caso.want {
			t.Errorf("%s: whatsappSanitize(%q) = %q, esperado %q", caso.nome, caso.in, got, caso.want)
		}
	}
}

func TestWhatsappSanitizeKeepsHeaderContent(t *testing.T) {
	got := whatsappSanitize("## Preços\n\nPlano A")
	if got != "*Preços*\n\nPlano A" {
		t.Fatalf("cabeçalho não deveria perder o texto, obteve %q", got)
	}
}
