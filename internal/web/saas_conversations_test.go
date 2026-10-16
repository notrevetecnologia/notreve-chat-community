package web

import (
	"bytes"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

func TestSaaSCanonicalConversationPaths(t *testing.T) {
	if id, ok := saasReactConversationID("/api/react/conversations/42"); !ok || id != 42 {
		t.Fatalf("conversation id = %d, ok=%v", id, ok)
	}
	if id, ok := saasReactConversationStatusID("/api/react/conversations/42/status"); !ok || id != 42 {
		t.Fatalf("status id = %d, ok=%v", id, ok)
	}
	if id, ok := saasReactMessageMediaID("/api/react/messages/7/media"); !ok || id != 7 {
		t.Fatalf("media id = %d, ok=%v", id, ok)
	}
	conversationID, messageID, ok := saasReactConversationMessageIDs("/api/react/conversations/42/messages/7")
	if !ok || conversationID != 42 || messageID != 7 {
		t.Fatalf("message path = (%d,%d), ok=%v", conversationID, messageID, ok)
	}
}

func TestSaaSCanonicalConversationPathsRejectLegacyAndMalformed(t *testing.T) {
	if _, ok := saasReactConversationID("/api/conversas/42"); ok {
		t.Fatal("rota legada foi aceita como detalhe canônico")
	}
	if _, ok := saasReactConversationStatusID("/api/conversas/42/status"); ok {
		t.Fatal("rota legada foi aceita como status canônico")
	}
	if _, ok := saasReactMessageMediaID("/api/mensagens/7/media"); ok {
		t.Fatal("rota legada foi aceita como mídia canônica")
	}
	if _, _, ok := saasReactConversationMessageIDs("/api/conversas/42/editar/7"); ok {
		t.Fatal("rota legada foi aceita como edição canônica")
	}
	for _, path := range []string{
		"/api/react/conversations/0",
		"/api/react/conversations/x",
		"/api/react/conversations/42/extra",
		"/api/react/messages/0/media",
		"/api/react/messages/x/media",
		"/api/react/conversations/42/messages/0",
		"/api/react/conversations/0/messages/7",
	} {
		if _, ok := saasReactConversationID(path); ok {
			t.Fatalf("rota malformada aceita como conversa: %s", path)
		}
		if _, ok := saasReactMessageMediaID(path); ok {
			t.Fatalf("rota malformada aceita como mídia: %s", path)
		}
		if _, _, ok := saasReactConversationMessageIDs(path); ok {
			t.Fatalf("rota malformada aceita como mensagem: %s", path)
		}
	}
}

func TestMessageMediaContentTypeFallsBackToAudio(t *testing.T) {
	if got := messageMediaContentType("audio", ""); got != "audio/ogg" {
		t.Fatalf("content-type sem mime = %q, esperado audio/ogg", got)
	}
	if got := messageMediaContentType("audio", "application/octet-stream"); got != "audio/ogg" {
		t.Fatalf("content-type genérico = %q, esperado audio/ogg", got)
	}
	if got := messageMediaContentType("audio", "audio/ogg; codecs=opus"); got != "audio/ogg; codecs=opus" {
		t.Fatalf("content-type declarado perdido: %q", got)
	}
}

func TestWriteMediaBytesServesAudioBytes(t *testing.T) {
	payload := []byte{0x4f, 0x67, 0x67, 0x53, 0x00, 0x02}
	raw, err := base64.StdEncoding.DecodeString(base64.StdEncoding.EncodeToString(payload))
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	writeMediaBytes(recorder, "audio", "", raw)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, esperado 200", recorder.Code)
	}
	if got := recorder.Header().Get("Content-Type"); got != "audio/ogg" {
		t.Fatalf("content-type = %q, esperado audio/ogg", got)
	}
	if got := recorder.Header().Get("Content-Length"); got != strconv.Itoa(len(payload)) {
		t.Fatalf("content-length = %q, esperado %d", got, len(payload))
	}
	if !bytes.Equal(recorder.Body.Bytes(), payload) {
		t.Fatalf("bytes servidos = %v, esperado %v", recorder.Body.Bytes(), payload)
	}
}
