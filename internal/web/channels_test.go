package web

import (
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestChannelFromFormMapsEditableWhatsAppFields(t *testing.T) {
	form := url.Values{
		"type":               {"whatsapp"},
		"name":               {" WhatsApp Comercial "},
		"base_url":           {"https://api.example.test"},
		"token":              {"novo-token"},
		"active":             {"on"},
		"respond_grupos":     {"on"},
		"responder_chamadas": {"on"},
		"can_campaign":       {"on"},
	}
	r := httptest.NewRequest("POST", "/canais/1/salvar", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	c := channelFromForm(r)
	if c.Type != "whatsapp" || c.Name != "WhatsApp Comercial" || c.BaseURL != "https://api.example.test" || c.Token != "novo-token" {
		t.Fatalf("dados editáveis do WhatsApp não foram mapeados: %+v", c)
	}
	if !c.Active || !c.RespondGrupos || !c.ResponderChamadas || !c.CanCampaign {
		t.Fatalf("opções do canal não foram mapeadas: %+v", c)
	}
}

func TestChannelFromFormCreatesWhatsAppWithoutAPIFields(t *testing.T) {
	form := url.Values{
		"type":               {"whatsapp"},
		"name":               {"WhatsApp Atendimento"},
		"active":             {"on"},
		"responder_chamadas": {"on"},
	}
	r := httptest.NewRequest("POST", "/canais/criar", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	c := channelFromForm(r)
	if c.BaseURL != "" || c.Token != "" {
		t.Fatalf("WhatsApp novo deveria usar integração automática, obtido base=%q token=%q", c.BaseURL, c.Token)
	}
	if c.Type != "whatsapp" || c.Name != "WhatsApp Atendimento" || !c.Active || !c.ResponderChamadas {
		t.Fatalf("dados básicos do WhatsApp não foram preservados: %+v", c)
	}
}
