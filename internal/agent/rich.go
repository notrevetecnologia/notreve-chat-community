package agent

import (
	"fmt"
	"strings"
)

// OptionButton é um botão de uma mensagem rica (kind "buttons").
type OptionButton struct {
	Type        string `json:"type"` // reply | cta_url | cta_call | copy
	Title       string `json:"title"`
	ID          string `json:"id"`
	URL         string `json:"url"`
	PhoneNumber string `json:"phone_number"`
	CopyCode    string `json:"copy_code"`
}

// ListRow é uma linha de uma lista (kind "list").
type ListRow struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	ID          string `json:"id"`
}

// ListSection é uma seção de uma lista.
type ListSection struct {
	Title string    `json:"title"`
	Rows  []ListRow `json:"rows"`
}

// PollOption é uma opção de enquete (kind "poll").
type PollOption struct {
	Title string `json:"title"`
	ID    string `json:"id"`
}

// Poll é o payload de uma enquete.
type Poll struct {
	Question string `json:"question"`
	Options  []PollOption `json:"options"`
	Multiple bool   `json:"multiple,omitempty"`
}

// Location é o payload de uma localização (kind "location").
type Location struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Name      string  `json:"name"`
	Address   string  `json:"address"`
}

// Contact é o payload de um contato (kind "contact").
type Contact struct {
	Name    string `json:"name"`
	Phone   string `json:"phone"`
	VCard   string `json:"vcard,omitempty"`
}

// RichMessage é um conteúdo de mensagem rica que um canal pode renderizar.
// Quando o canal não suporta o tipo (webchat/telegram) ou a API falha, o
// emissor deve cair no Fallback (texto corrido/formatado). A decisão de enviar
// ou não um tipo rico é sempre "best-effort", nunca pode impedir o atendimento.
type RichMessage struct {
	Kind      string        `json:"kind"` // text | buttons | list | poll | contact | location | sticker
	Text      string        `json:"text"`
	Title     string        `json:"title"`
	Footer    string        `json:"footer"`
	Buttons   []OptionButton `json:"buttons,omitempty"`
	Sections  []ListSection  `json:"sections,omitempty"`
	Poll      *Poll          `json:"poll,omitempty"`
	Location  *Location      `json:"location,omitempty"`
	Contact   *Contact       `json:"contact,omitempty"`
	Sticker   string         `json:"sticker,omitempty"` // data URI ou URL
	Fallback  string         `json:"-"`
}

// SupportedRichKinds lista os tipos ricos que o motor sabe produzir.
func (m *RichMessage) Valid() bool {
	switch m.Kind {
	case "buttons", "list", "poll", "contact", "location", "sticker":
		return true
	default:
		return false
	}
}

// OptionsList extrai, de qualquer tipo rico com opções, a lista plana de
// opções (para gerar o texto de fallback numerado).
func (m *RichMessage) OptionsList() []string {
	var out []string
	switch m.Kind {
	case "buttons":
		for _, b := range m.Buttons {
			if b.Title != "" {
				out = append(out, b.Title)
			}
		}
	case "list":
		for _, s := range m.Sections {
			for _, r := range s.Rows {
				if r.Title != "" {
					out = append(out, r.Title)
				}
			}
		}
	case "poll":
		for _, o := range m.Poll.Options {
			if o.Title != "" {
				out = append(out, o.Title)
			}
		}
	}
	return out
}

// FallbackText devolve o texto de fallback de um conteúdo rico: o corpo (Text)
// seguido das opções numeradas (1..n), ou uma descrição para contato/localização.
func (m *RichMessage) FallbackText() string {
	if m.Fallback != "" {
		return m.Fallback
	}
	var b strings.Builder
	body := strings.TrimSpace(m.Text)
	if body == "" {
		body = "Escolha uma opção:"
	}
	b.WriteString(body)
	options := m.OptionsList()
	if len(options) > 0 {
		b.WriteString("\n")
		for i, opt := range options {
			fmt.Fprintf(&b, "%d. %s\n", i+1, opt)
		}
		return strings.TrimRight(b.String(), "\n")
	}
	switch m.Kind {
	case "contact":
		if m.Contact != nil {
			line := "Contato: " + m.Contact.Name
			if m.Contact.Phone != "" {
				line += " (" + m.Contact.Phone + ")"
			}
			if m.Text != "" {
				return m.Text + "\n" + line
			}
			return line
		}
	case "location":
		if m.Location != nil {
			line := "Localização: " + m.Location.Name
			if m.Location.Address != "" {
				line += " — " + m.Location.Address
			}
			if m.Text != "" {
				return m.Text + "\n" + line
			}
			return line
		}
	case "sticker":
		if m.Sticker != "" {
			return "Figurinha enviada." + fallbackSuffix(m.Text)
		}
	}
	return body
}

func fallbackSuffix(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	return "\n" + text
}

// buildRichButtons monta uma mensagem de botões a partir de opções simples
// (rótulo -> id igual ao rótulo), usada pelo menu configurável e marcadores.
func buildRichButtons(text string, options []string) *RichMessage {
	if len(options) == 0 {
		return nil
	}
	buttons := make([]OptionButton, 0, len(options))
	for _, opt := range options {
		opt = strings.TrimSpace(opt)
		if opt == "" {
			continue
		}
		buttons = append(buttons, OptionButton{Type: "reply", Title: opt, ID: opt})
	}
	if len(buttons) == 0 {
		return nil
	}
	return &RichMessage{Kind: "buttons", Text: text, Buttons: buttons}
}


