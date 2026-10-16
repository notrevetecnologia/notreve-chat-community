package channel

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestResolvePhoneNeverTreatsLIDAsPhone(t *testing.T) {
	if got := resolvePhone("5511999999999@lid"); got != "" {
		t.Fatalf("LID foi tratado como telefone: %q", got)
	}
	if got := resolvePhone("5511999999999@s.whatsapp.net"); got != "5511999999999" {
		t.Fatalf("JID telefônico não foi resolvido: %q", got)
	}
	if got := resolvePhone("5511999999999.0:42@s.whatsapp.net"); got != "5511999999999" {
		t.Fatalf("JID de dispositivo não foi normalizado: %q", got)
	}
}

func TestResolvePhoneAvoidingBareLID(t *testing.T) {
	lid := "5511999999999@lid"
	if got := resolvePhoneAvoidingLID(lid, "5511999999999", "5511888888888@s.whatsapp.net"); got != "5511888888888" {
		t.Fatalf("esperava telefone alternativo, obtido %q", got)
	}
	if got := resolvePhoneAvoidingLID(lid, "5511999999999"); got != "" {
		t.Fatalf("LID sem sufixo foi tratado como telefone: %q", got)
	}
}

func TestResolvePhoneRejectsGroupAndChannelJID(t *testing.T) {
	if got := resolvePhone("120363000000000000@g.us"); got != "" {
		t.Fatalf("JID de grupo foi tratado como telefone: %q", got)
	}
	if got := resolvePhone("123456789@newsletter"); got != "" {
		t.Fatalf("JID de canal foi tratado como telefone: %q", got)
	}
	if got := resolvePhone("status@broadcast"); got != "" {
		t.Fatalf("JID de status foi tratado como telefone: %q", got)
	}
}

func TestIsRealPhoneAcceptsOnlyPlausibleE164(t *testing.T) {
	valid := []string{"5511999999999", "5527996420402", "551133334444"}
	for _, phone := range valid {
		if !isRealPhone(phone) {
			t.Fatalf("telefone válido rejeitado: %q", phone)
		}
	}
	invalid := []string{
		"",
		"0000000000000",
		"55119999999",
		"551199999999999999",
		"0500000000000",
		"5520999999999",
		"abcdefghijkl",
		"120363000000000000",
		"123456789012345",
	}
	for _, phone := range invalid {
		if isRealPhone(phone) {
			t.Fatalf("identificador inválido aceito como telefone: %q", phone)
		}
	}
}

func TestWhatsAppConversationPhoneValid(t *testing.T) {
	if !whatsappConversationPhoneValid("5511999999999") {
		t.Fatal("telefone privado válido deveria ser aceito")
	}
	if !whatsappConversationPhoneValid("120363000000000000@g.us") {
		t.Fatal("JID de grupo deveria ser aceito para preservar grupos")
	}
	for _, phone := range []string{"", "123456789@lid", "123456789@newsletter", "status@broadcast", "0000000000000"} {
		if whatsappConversationPhoneValid(phone) {
			t.Fatalf("identificador não-conversa aceito: %q", phone)
		}
	}
}

func TestReplyJIDPrefersProvidedChatIdentity(t *testing.T) {
	if got := replyJIDOf("0000000000000@lid", "0000000000000@s.whatsapp.net"); got != "0000000000000@lid" {
		t.Fatalf("destino inesperado: %q", got)
	}
	if got := normalizeLID("0000000000000:42@lid"); got != "0000000000000@lid" {
		t.Fatalf("LID de dispositivo não foi normalizado: %q", got)
	}
}

func TestDecodeNotreveCallEvent(t *testing.T) {
	raw := []byte(`{"type":"CallOffer","event":{"CallCreator":"5511999999999@s.whatsapp.net","CallID":"call-123"}}`)
	var payload struct {
		Event json.RawMessage `json:"event"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	evt, err := decodeNotreveCallEvent(payload.Event)
	if err != nil {
		t.Fatal(err)
	}
	creator, callID := evt.values()
	if creator != "5511999999999@s.whatsapp.net" || callID != "call-123" {
		t.Fatalf("dados inesperados: creator=%q callID=%q", creator, callID)
	}
	if got := callRejectJID(creator, "5511999999999"); got != "5511999999999@s.whatsapp.net" {
		t.Fatalf("JID inesperado: %q", got)
	}
}

func TestCallEventPhoneUsesAlternativeCreator(t *testing.T) {
	raw := []byte(`{"CallCreator":"123456789@lid","CallCreatorAlt":"5511999999999@s.whatsapp.net","CallID":"call-123"}`)
	evt, err := decodeNotreveCallEvent(raw)
	if err != nil {
		t.Fatal(err)
	}
	creator, phone := callEventPhone(evt)
	if creator != "5511999999999@s.whatsapp.net" || phone != "5511999999999" {
		t.Fatalf("dados inesperados: creator=%q phone=%q", creator, phone)
	}
	if got := callRejectJID("123456789@lid", phone); got != "123456789@lid" {
		t.Fatalf("JID de rejeição inesperado: %q", got)
	}
}

func TestCallEventPhoneSearchesNestedInfo(t *testing.T) {
	raw := []byte(`{"BasicCallMeta":{"CallCreator":"123456789@lid","CallCreatorAlt":"5511888888888@s.whatsapp.net","CallID":"call-456"}}`)
	evt, err := decodeNotreveCallEvent(raw)
	if err != nil {
		t.Fatal(err)
	}
	creator, phone := callEventPhone(evt)
	if creator != "5511888888888@s.whatsapp.net" || phone != "5511888888888" {
		t.Fatalf("dados inesperados: creator=%q phone=%q", creator, phone)
	}
}

func TestNormalizeNotrevePayloadSnakeCase(t *testing.T) {
	var payload notrevePayload
	if err := json.Unmarshal([]byte(`{"instance_name":"atendimento","user_id":"token","phone_jid":"5511999999999@s.whatsapp.net","addressing_mode":"pn","chat_jid":"5511999999999@s.whatsapp.net","is_from_me":true,"message_type":"text","media_link":"https://example.com/file"}`), &payload); err != nil {
		t.Fatal(err)
	}
	normalizeNotrevePayload(&payload)
	if payload.InstanceName != "atendimento" || payload.UserID != "token" || payload.PhoneJID != "5511999999999@s.whatsapp.net" || payload.AddressingMode != "pn" || payload.ChatJID != "5511999999999@s.whatsapp.net" || !payload.IsFromMe || payload.MessageType != "text" || payload.MediaLink != "https://example.com/file" {
		t.Fatalf("payload não normalizado: %+v", payload)
	}
}

func TestWhatsAppGroupChatUsesFlagOrJID(t *testing.T) {
	tests := []struct {
		name   string
		chat   string
		sender string
		type_  string
		group  bool
		want   bool
	}{
		{name: "flag", group: true, want: true},
		{name: "group jid", chat: "120363000000000000@g.us", want: true},
		{name: "group jid no sender", sender: "120363000000000000@g.us", want: true},
		{name: "community type", type_: "community", want: true},
		{name: "comunidade type", type_: "comunidade", want: true},
		{name: "private jid", chat: "5511999999999@s.whatsapp.net", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evt := notreveEvent{}
			evt.Info.Chat = tt.chat
			evt.Info.Sender = tt.sender
			evt.Info.Type = tt.type_
			evt.Info.IsGroup = tt.group
			if got := whatsappGroupChat(evt); got != tt.want {
				t.Fatalf("whatsappGroupChat() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWhatsAppChannelChatIsReadOnly(t *testing.T) {
	tests := []struct {
		name   string
		chat   string
		sender string
		type_  string
	}{
		{name: "broadcast jid", chat: "status@broadcast"},
		{name: "newsletter jid", chat: "123456789@newsletter"},
		{name: "broadcast jid sem chat", sender: "status@broadcast"},
		{name: "newsletter jid sem chat", sender: "123456789@newsletter"},
		{name: "channel type", type_: "channel"},
		{name: "status type", type_: "status"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evt := notreveEvent{}
			evt.Info.Chat = tt.chat
			evt.Info.Sender = tt.sender
			evt.Info.Type = tt.type_
			if !whatsappChannelChat(evt) {
				t.Fatal("whatsappChannelChat() = false, want true")
			}
		})
	}
}

func TestInstanciaPhoneToLIDDecodesEnvelope(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/user/lid/0000000000000" {
			t.Fatalf("caminho inesperado: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":200,"data":{"jid":"0000000000000@s.whatsapp.net","lid":"0000000000000@lid"},"success":true}`))
	}))
	defer server.Close()

	got, err := InstanciaPhoneToLID(context.Background(), server.URL, "token-teste", "0000000000000")
	if err != nil || got != "0000000000000@lid" {
		t.Fatalf("LID inesperado: %q, erro=%v", got, err)
	}
}
