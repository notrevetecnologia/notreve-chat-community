package web

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/notrevetecnologia/notreve-chat-community/internal/store"
)

func TestReactChannelPayloadDoesNotExposeSecrets(t *testing.T) {
	channel := reactChannelFromStore(store.Channel{
		ID: 4, TenantID: 9, Type: "whatsapp", Name: "Comercial", Token: "token-secreto", BaseURL: "https://api.exemplo",
		Active: true, ResponderChamadas: true,
	})
	b, err := json.Marshal(channel)
	if err != nil {
		t.Fatal(err)
	}
	payload := string(b)
	for _, secret := range []string{"token-secreto", "https://api.exemplo", "tenant_id", "instance_id"} {
		if strings.Contains(payload, secret) {
			t.Fatalf("payload de canal expôs dado sensível %q: %s", secret, payload)
		}
	}
}

func TestReactChannelWidgetParsing(t *testing.T) {
	valid := json.RawMessage(`{"titulo":"Fale conosco","dominios":["https://cliente.com.br"]}`)
	encoded, err := parseReactChannelWidget(&valid)
	if err != nil || !strings.Contains(encoded, "Fale conosco") {
		t.Fatalf("widget válido rejeitado: %q %v", encoded, err)
	}
	invalid := json.RawMessage(`[]`)
	if _, err := parseReactChannelWidget(&invalid); err == nil {
		t.Fatal("widget que não é objeto deveria ser rejeitado")
	}
	tooLarge := json.RawMessage(strings.Repeat("x", 64*1024+1))
	if _, err := parseReactChannelWidget(&tooLarge); err == nil {
		t.Fatal("widget acima do limite deveria ser rejeitado")
	}
}

func TestDisconnectReactChannelWhatsAppEndsSession(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":200,"success":true,"data":{}}`))
	}))
	defer server.Close()
	channel := store.Channel{ID: 3, Type: "whatsapp", BaseURL: server.URL + "/", Token: "token-teste"}
	if err := disconnectReactChannel(context.Background(), channel); err != nil {
		t.Fatalf("desconexão do WhatsApp falhou: %v", err)
	}
	if len(paths) != 1 || paths[0] != "/session/logout" {
		t.Fatalf("endpoint de encerramento inesperado: %v", paths)
	}
}

func TestDisconnectReactChannelSkipsChannelsWithoutCredentials(t *testing.T) {
	for _, channel := range []store.Channel{
		{ID: 1, Type: "whatsapp"},
		{ID: 2, Type: "whatsapp", BaseURL: "https://api.exemplo", Token: ""},
		{ID: 3, Type: "webchat"},
	} {
		if err := disconnectReactChannel(context.Background(), channel); err != nil {
			t.Fatalf("canal %d não deveria tentar desconectar: %v", channel.ID, err)
		}
	}
}

func TestReactChannelDeleteDisconnectsBeforeRemoving(t *testing.T) {
	var order []string
	channel := store.Channel{ID: 11, Type: "whatsapp"}
	err := removeReactChannel(context.Background(), channel,
		func(context.Context, store.Channel) error {
			order = append(order, "disconnect")
			return nil
		},
		func(context.Context, int) error {
			order = append(order, "remove")
			return nil
		})
	if err != nil {
		t.Fatalf("remoção falhou: %v", err)
	}
	if strings.Join(order, ",") != "disconnect,remove" {
		t.Fatalf("ordem inesperada de teardown: %v", order)
	}
}

func TestReactChannelDeleteRemovesEvenWhenDisconnectFails(t *testing.T) {
	removed := false
	channel := store.Channel{ID: 12, Type: "telegram", Token: "token"}
	err := removeReactChannel(context.Background(), channel,
		func(context.Context, store.Channel) error { return errors.New("provedor indisponível") },
		func(context.Context, int) error { removed = true; return nil })
	if err != nil {
		t.Fatalf("a falha de desconexão não deveria impedir a remoção: %v", err)
	}
	if !removed {
		t.Fatal("canal deveria ter sido removido mesmo com falha na desconexão")
	}
}

func TestReactChannelDeletePropagatesRemovalFailure(t *testing.T) {
	channel := store.Channel{ID: 13, Type: "webchat"}
	err := removeReactChannel(context.Background(), channel, nil, func(context.Context, int) error {
		return errors.New("db indisponível")
	})
	if err == nil {
		t.Fatal("falha de remoção deveria ser propagada")
	}
}

func TestReactChannelTypeAndNameValidation(t *testing.T) {
	if !validateReactChannelType("whatsapp") || !validateReactChannelType("webchat") || !validateReactChannelType("telegram") {
		t.Fatal("tipos de canal suportados foram rejeitados")
	}
	if validateReactChannelType("instagram") {
		t.Fatal("tipo ainda não suportado deveria ser rejeitado")
	}
	if validateReactChannelName(" ") == nil {
		t.Fatal("nome vazio deveria ser rejeitado")
	}
	if validateReactChannelName(strings.Repeat("a", 121)) == nil {
		t.Fatal("nome acima do limite deveria ser rejeitado")
	}
}
