package store

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestResolveActiveChatflowRespectsAtivoAndLink(t *testing.T) {
	pool := newSaaSIsolationPool(t)
	defer pool.Close()

	tenantA := createIsolationTenant(t, pool, "cf-a")
	tenantB := createIsolationTenant(t, pool, "cf-b")
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM tenants WHERE id=$1 OR id=$2`, tenantA, tenantB)
	})

	s := New(pool)
	ctxA := WithTenantID(context.Background(), tenantA)
	ctxB := WithTenantID(context.Background(), tenantB)

	channelID, err := s.CreateChannel(ctxA, Channel{Type: "webchat", Name: "Canal CF", Active: true, WidgetConfig: "{}", PublicID: "cf-channel"})
	if err != nil {
		t.Fatalf("criando canal: %v", err)
	}

	flowID, err := s.CreateChatflow(ctxA, Chatflow{Nome: "Fluxo A", Ativo: false})
	if err != nil {
		t.Fatalf("criando chat flow: %v", err)
	}
	if _, err := s.CreateChatflowStep(ctxA, ChatflowStep{ChatflowID: flowID, Ord: 1, Tipo: "start", Config: `{"next":2}`}); err != nil {
		t.Fatalf("criando passo do chat flow: %v", err)
	}

	if err := s.SetChannelChatflow(ctxA, channelID, &flowID); err != nil {
		t.Fatalf("vinculando chat flow: %v", err)
	}
	channel, err := s.GetChannelByID(ctxA, channelID)
	if err != nil {
		t.Fatalf("lendo canal: %v", err)
	}
	if channel.ChatflowID == nil || *channel.ChatflowID != flowID || channel.Mode != "chatflow" {
		t.Fatalf("chatflow_id/mode não persistiu: %+v (%q)", channel.ChatflowID, channel.Mode)
	}

	flow, steps, err := s.ResolveActiveChatflow(ctxA, channel.ChatflowID)
	if err != nil {
		t.Fatalf("resolvendo flow inativo: %v", err)
	}
	if flow != nil || steps != nil {
		t.Fatalf("flow inativo deveria cair na IA livre: %+v", flow)
	}

	if err := s.ActivateChatflow(ctxA, flowID, true); err != nil {
		t.Fatalf("ativando chat flow: %v", err)
	}
	flow, steps, err = s.ResolveActiveChatflow(ctxA, channel.ChatflowID)
	if err != nil {
		t.Fatalf("resolvendo flow ativo: %v", err)
	}
	if flow == nil || flow.ID != flowID || len(steps) != 1 {
		t.Fatalf("flow ativo não foi resolvido: %+v com %d passos", flow, len(steps))
	}

	if err := s.SetChannelChatflow(ctxA, channelID, nil); err != nil {
		t.Fatalf("limpando vínculo: %v", err)
	}
	channel, err = s.GetChannelByID(ctxA, channelID)
	if err != nil {
		t.Fatalf("relendo canal: %v", err)
	}
	if channel.ChatflowID != nil || channel.Mode != "ai" {
		t.Fatalf("vínculo deveria ter sido limpo: %+v (%q)", channel.ChatflowID, channel.Mode)
	}
	flow, _, err = s.ResolveActiveChatflow(ctxA, channel.ChatflowID)
	if err != nil || flow != nil {
		t.Fatalf("sem vínculo deveria cair na IA livre: %+v (%v)", flow, err)
	}

	otherFlow, err := s.CreateChatflow(ctxB, Chatflow{Nome: "Fluxo B", Ativo: true})
	if err != nil {
		t.Fatalf("criando chat flow de outro tenant: %v", err)
	}
	if err := s.SetChannelChatflow(ctxA, channelID, &otherFlow); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("vincular flow de outro tenant deveria falhar com ErrNoRows, obteve %v", err)
	}
}
