package agent

import (
	"regexp"
	"strconv"
	"strings"
)

// Marcadores de mensagem rica que a IA pode emitir no texto da resposta. O
// motor extrai o primeiro marcador encontrado, converte em RichMessage e
// remove o marcador do texto, que vira o corpo. Se nada for extraído (ou for
// inválido), a resposta segue como texto puro — nunca quebra o atendimento.

var markers = []struct {
	re  *regexp.Regexp
	kind string
}{
	{regexp.MustCompile(`(?i)\[OPCOES:([^\]]+)\]`), "buttons"},
	{regexp.MustCompile(`(?i)\[LISTA:([^\]]+)\]`), "list"},
	{regexp.MustCompile(`(?i)\[ENQUETE:([^\]]+)\]`), "poll"},
	{regexp.MustCompile(`(?i)\[CONTATO:([^\]]+)\]`), "contact"},
	{regexp.MustCompile(`(?i)\[LOCALIZACAO:([^\]]+)\]`), "location"},
	{regexp.MustCompile(`(?i)\[STICKER:([^\]]+)\]`), "sticker"},
}

// parseRichMarkers procura o primeiro marcador rico no texto e devolve a
// mensagem rica construída e o texto sem o marcador. Devolve (nil, texto) quando
// não há marcador válido.
func parseRichMarkers(text string) (*RichMessage, string) {
	for _, mk := range markers {
		loc := mk.re.FindStringIndex(text)
		if loc == nil {
			continue
		}
		match := text[loc[0]:loc[1]]
		inner := mk.re.FindStringSubmatch(match)
		if len(inner) < 2 {
			continue
		}
		clean := strings.TrimSpace(text[:loc[0]] + " " + text[loc[1]:])
		if built := buildRichFromMarker(mk.kind, strings.TrimSpace(inner[1])); built != nil {
			return built, clean
		}
	}
	return nil, text
}

func buildRichFromMarker(kind, arg string) *RichMessage {
	switch kind {
	case "buttons":
		body, options := splitBodyOptions(arg)
		rich := buildRichButtons(body, options)
		return rich
	case "list":
		parts := splitSeparator(arg, "|")
		if len(parts) < 2 {
			return nil
		}
		title := parts[0]
		rows := make([]ListRow, 0, len(parts)-1)
		for _, p := range parts[1:] {
			fields := splitSeparator(p, ",")
			label := ""
			desc := ""
			if len(fields) > 0 {
				label = strings.TrimSpace(fields[0])
			}
			if len(fields) > 1 {
				desc = strings.TrimSpace(fields[1])
			}
			if label == "" {
				continue
			}
			rows = append(rows, ListRow{Title: label, Description: desc, ID: label})
		}
		if len(rows) == 0 {
			return nil
		}
		return &RichMessage{Kind: "list", Title: title, Text: title, Sections: []ListSection{{Title: title, Rows: rows}}}
	case "poll":
		parts := splitSeparator(arg, "|")
		if len(parts) < 2 {
			return nil
		}
		question := strings.TrimSpace(parts[0])
		if question == "" {
			return nil
		}
		var options []PollOption
		multiple := false
		if len(parts) > 2 {
			multiple = strings.EqualFold(strings.TrimSpace(parts[2]), "multiple") || strings.EqualFold(strings.TrimSpace(parts[2]), "multi")
		}
		for _, o := range splitSeparator(parts[1], ",") {
			opt := strings.TrimSpace(o)
			if opt == "" {
				continue
			}
			options = append(options, PollOption{Title: opt, ID: opt})
		}
		if len(options) < 2 {
			return nil
		}
		return &RichMessage{Kind: "poll", Text: question, Poll: &Poll{Question: question, Options: options, Multiple: multiple}}
	case "contact":
		fields := splitSeparator(arg, "|")
		if len(fields) < 2 {
			return nil
		}
		name := strings.TrimSpace(fields[0])
		phone := strings.TrimSpace(fields[1])
		if name == "" || phone == "" {
			return nil
		}
		return &RichMessage{Kind: "contact", Text: "Aqui está o contato de " + name, Contact: &Contact{Name: name, Phone: phone}}
	case "location":
		fields := splitSeparator(arg, "|")
		if len(fields) < 3 {
			return nil
		}
		lat, err1 := strconv.ParseFloat(strings.TrimSpace(fields[0]), 64)
		lng, err2 := strconv.ParseFloat(strings.TrimSpace(fields[1]), 64)
		if err1 != nil || err2 != nil {
			return nil
		}
		name := strings.TrimSpace(fields[2])
		return &RichMessage{Kind: "location", Text: "Nossa localização", Location: &Location{Latitude: lat, Longitude: lng, Name: name}}
	case "sticker":
		uri := strings.TrimSpace(arg)
		if uri == "" {
			return nil
		}
		return &RichMessage{Kind: "sticker", Sticker: uri}
	}
	return nil
}

// splitBodyOptions divide "corpo | op1 | op2" em corpo e opções (buttons).
func splitBodyOptions(arg string) (string, []string) {
	parts := splitSeparator(arg, "|")
	if len(parts) <= 1 {
		return "", menuOptionList(arg)
	}
	body := strings.TrimSpace(parts[0])
	options := make([]string, 0, len(parts)-1)
	for _, p := range parts[1:] {
		for _, o := range splitSeparator(p, ",") {
			o = strings.TrimSpace(o)
			if o != "" {
				options = append(options, o)
			}
		}
	}
	return body, options
}

func splitSeparator(s, sep string) []string {
	raw := strings.Split(s, sep)
	out := make([]string, 0, len(raw))
	for _, p := range raw {
		out = append(out, strings.TrimSpace(p))
	}
	return out
}
