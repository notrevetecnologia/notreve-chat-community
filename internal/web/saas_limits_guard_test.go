package web

import (
	"strings"
	"testing"
)

func TestValidBcryptPasswordSize(t *testing.T) {
	valid := strings.Repeat("a", 72)
	invalid := strings.Repeat("a", 73)
	blank := "   "
	if !validBcryptPasswordSize(nil) {
		t.Fatal("senha ausente em atualização deveria ser aceita")
	}
	if !validBcryptPasswordSize(&blank) {
		t.Fatal("senha vazia em atualização deveria ser aceita")
	}
	if !validBcryptPasswordSize(&valid) {
		t.Fatal("senha com 72 bytes deveria ser aceita")
	}
	if validBcryptPasswordSize(&invalid) {
		t.Fatal("senha acima de 72 bytes deveria ser rejeitada")
	}
}
