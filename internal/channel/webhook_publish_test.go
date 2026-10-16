package channel

import (
	"strconv"
	"testing"
)

func TestReceivedMessageKeyUsesWAMsgID(t *testing.T) {
	if got := receivedMessageKey("wa-msg-123"); got != "wa-msg-123" {
		t.Fatalf("esperava chave estável do WAMsgID, obtive %q", got)
	}
}

func TestReceivedMessageKeyWithoutWAMsgIDIsNumeric(t *testing.T) {
	key := receivedMessageKey("")
	if key == "" {
		t.Fatal("chave não pode ser vazia")
	}
	if _, err := strconv.ParseInt(key, 10, 64); err != nil {
		t.Fatalf("sem WAMsgID a chave deve ser numérica (timestamp), obtive %q", key)
	}
}
