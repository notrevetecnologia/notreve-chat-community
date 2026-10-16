package channel

import (
	"context"
	"testing"

	"github.com/notrevetecnologia/notreve-chat-community/internal/store"
)

func TestTelegramChatIDFromClient(t *testing.T) {
	cases := []struct {
		client string
		want   int64
	}{
		{"telegram:12345", 12345},
		{"telegram:", 0},
		{"telegram:abc", 0},
		{"+5511999999999", 0},
		{"telegram:-1001234567890", -1001234567890},
		{"", 0},
	}
	for _, c := range cases {
		if got := telegramChatIDFromClient(c.client); got != c.want {
			t.Errorf("telegramChatIDFromClient(%q) = %d, want %d", c.client, got, c.want)
		}
	}
}

func TestRegistryResolvesAdapterByType(t *testing.T) {
	reg := NewRegistry()
	reg.Register(NewWhatsAppAdapter(nil))
	reg.Register(NewTelegramAdapter())
	reg.Register(NewWebChatAdapter())

	if reg.For("whatsapp") == nil || reg.For("telegram") == nil || reg.For("webchat") == nil {
		t.Fatal("adaptadores dos canais do núcleo precisam ser resolvidos")
	}
	if reg.For("instagram") != nil {
		t.Fatal("canal não registrado deve resolver nil")
	}
	if reg.For("whatsapp").Type() != "whatsapp" {
		t.Fatal("Type() deve devolver o identificador do canal")
	}
}

func TestTelegramAdapterSendRejectsInvalidClient(t *testing.T) {
	a := NewTelegramAdapter()
	ch := store.Channel{Token: "tok"}
	err := a.Send(context.Background(), ch, "not-a-telegram-id", Outbound{Text: "oi"})
	if err == nil {
		t.Fatal("identificador inválido deve causar erro")
	}
}

func TestWebChatAdapterIsPassive(t *testing.T) {
	a := NewWebChatAdapter()
	if err := a.Send(context.Background(), store.Channel{}, "k", Outbound{Text: "oi"}); err != nil {
		t.Fatalf("webchat Send deve ser passivo (nil), obtido: %v", err)
	}
	if err := a.SendMedia(context.Background(), store.Channel{}, "k", OutboundMedia{}); err != ErrUnsupported {
		t.Fatalf("webchat mídia deve ser ErrUnsupported, obtido: %v", err)
	}
}

type channelSpecificSender struct {
	Sender
	channelID string
	text      string
}

func (s *channelSpecificSender) SendTextChannel(_ context.Context, ch store.Channel, _ string, text string) (string, error) {
	s.channelID = ch.InstanceID
	s.text = text
	return "msg-1", nil
}

func (s *channelSpecificSender) SendTextChannelReply(_ context.Context, ch store.Channel, _ string, text, _ string) error {
	s.channelID = ch.InstanceID
	s.text = text
	return nil
}

func TestWhatsAppAdapterUsesConversationChannel(t *testing.T) {
	sender := &channelSpecificSender{}
	a := NewWhatsAppAdapter(sender)
	ch := store.Channel{Type: "whatsapp", InstanceID: "instance-from-conversation", BaseURL: "https://api.example", Token: "token"}
	if err := a.Send(context.Background(), ch, "5511999999999", Outbound{Text: "resposta"}); err != nil {
		t.Fatalf("envio falhou: %v", err)
	}
	if sender.channelID != ch.InstanceID || sender.text != "resposta" {
		t.Fatalf("adaptador não preservou o canal da conversa: id=%q text=%q", sender.channelID, sender.text)
	}
}
