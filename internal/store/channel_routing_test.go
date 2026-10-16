package store

import "testing"

func TestConversationIdentityKeyNormalizesPhoneAndKeepsWebchatVisitor(t *testing.T) {
	if got := conversationIdentityKey("0000000000000@s.whatsapp.net"); got != "0000000000000" {
		t.Fatalf("JID não normalizado: %q", got)
	}
	if got := conversationIdentityKey("+00 (00) 00000-0000"); got != "0000000000000" {
		t.Fatalf("telefone formatado não normalizado: %q", got)
	}
	if got := conversationIdentityKey("web:visitor-abc"); got != "web:visitor-abc" {
		t.Fatalf("identidade de visitante alterada: %q", got)
	}
}
