package channel

import (
	"encoding/json"
	"testing"
)

func TestTelegramHasMedia(t *testing.T) {
	cases := []struct {
		name string
		msg  telegramMessage
		want bool
	}{
		{name: "texto", msg: telegramMessage{Text: "oi"}, want: false},
		{name: "foto", msg: telegramMessage{Photo: []json.RawMessage{{}}}, want: true},
		{name: "legenda", msg: telegramMessage{Caption: "foto", Photo: []json.RawMessage{{}}}, want: true},
		{name: "voz", msg: telegramMessage{Voice: json.RawMessage(`{}`)}, want: true},
		{name: "video", msg: telegramMessage{Video: json.RawMessage(`{}`)}, want: true},
		{name: "documento", msg: telegramMessage{Document: json.RawMessage(`{}`)}, want: true},
		{name: "localizacao", msg: telegramMessage{Location: json.RawMessage(`{}`)}, want: true},
		{name: "null", msg: telegramMessage{Video: json.RawMessage(`null`)}, want: false},
		{name: "sem mensagem", msg: telegramMessage{}, want: false},
	}
	for _, c := range cases {
		if got := c.msg.hasMedia(); got != c.want {
			t.Fatalf("%s: hasMedia = %v, esperado %v", c.name, got, c.want)
		}
	}
	if (*telegramMessage)(nil).hasMedia() {
		t.Fatal("mensagem nula não deveria conter mídia")
	}
}

func TestTelegramMethodURL(t *testing.T) {
	if got := telegramMethodURL("123:TOK", "sendMessage"); got != "https://api.telegram.org/bot123:TOK/sendMessage" {
		t.Fatalf("URL inesperada: %q", got)
	}
}
