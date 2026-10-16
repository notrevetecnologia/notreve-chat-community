package web

import (
	"encoding/json"
	"testing"

	"github.com/notrevetecnologia/notreve-chat-community/internal/store"
)

func TestReactCampaignGroupSerializesEmptySlicesAsArrays(t *testing.T) {
	view := reactCampaignGroupFromStore(store.GrupoCanal{ID: 7, Name: "Comercial"})
	raw, err := json.Marshal(view)
	if err != nil {
		t.Fatalf("marshal falhou: %v", err)
	}
	var decoded map[string]json.RawMessage
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal falhou: %v", err)
	}
	if got := string(decoded["channel_ids"]); got != "[]" {
		t.Fatalf("channel_ids=%s, esperado []", got)
	}
	if got := string(decoded["channel_names"]); got != "[]" {
		t.Fatalf("channel_names=%s, esperado []", got)
	}
}

func TestReactCampaignGroupPreservesChannels(t *testing.T) {
	view := reactCampaignGroupFromStore(store.GrupoCanal{ID: 3, Name: "Suporte", ChannelIDs: []int{1, 2}, ChannelNames: []string{"A", "B"}})
	raw, err := json.Marshal(view)
	if err != nil {
		t.Fatalf("marshal falhou: %v", err)
	}
	var decoded struct {
		ChannelIDs   []int    `json:"channel_ids"`
		ChannelNames []string `json:"channel_names"`
	}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal falhou: %v", err)
	}
	if len(decoded.ChannelIDs) != 2 || decoded.ChannelIDs[0] != 1 || decoded.ChannelIDs[1] != 2 {
		t.Fatalf("channel_ids inesperado: %+v", decoded.ChannelIDs)
	}
	if len(decoded.ChannelNames) != 2 || decoded.ChannelNames[0] != "A" || decoded.ChannelNames[1] != "B" {
		t.Fatalf("channel_names inesperado: %+v", decoded.ChannelNames)
	}
}

func TestReactCampaignGroupsEnvelope(t *testing.T) {
	data := map[string]interface{}{
		"items":    []reactCampaignGroup{reactCampaignGroupFromStore(store.GrupoCanal{ID: 1, Name: "A"})},
		"channels": []reactCampaignChannel{{ID: 3, Name: "WA", Type: "whatsapp", Active: true}},
	}
	meta := reactPageMeta(1, 25, 0, nil)
	raw, err := json.Marshal(reactEnvelope{Data: data, Meta: meta})
	if err != nil {
		t.Fatalf("marshal falhou: %v", err)
	}
	var decoded struct {
		Data struct {
			Items    []map[string]json.RawMessage `json:"items"`
			Channels []reactCampaignChannel       `json:"channels"`
		} `json:"data"`
		Meta map[string]interface{} `json:"meta"`
	}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal falhou: %v", err)
	}
	if len(decoded.Data.Items) != 1 {
		t.Fatalf("items=%d, esperado 1", len(decoded.Data.Items))
	}
	if got := string(decoded.Data.Items[0]["channel_ids"]); got != "[]" {
		t.Fatalf("items[0].channel_ids=%s, esperado []", got)
	}
	if len(decoded.Data.Channels) != 1 || decoded.Data.Channels[0].Name != "WA" {
		t.Fatalf("channels inesperado: %+v", decoded.Data.Channels)
	}
	if total, ok := decoded.Meta["total"].(float64); !ok || total != 1 {
		t.Fatalf("meta.total=%v, esperado 1", decoded.Meta["total"])
	}
}
