package securesecret

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

const prefix = "enc:v1:"

var ErrKeyUnavailable = errors.New("AGENTE_IA_SECRETS_KEY não configurada")
var ErrPlaintextSecret = errors.New("segredo não cifrado recusado")

func masterKey() ([]byte, error) {
	raw := strings.TrimSpace(os.Getenv("AGENTE_IA_SECRETS_KEY"))
	if raw == "" {
		return nil, ErrKeyUnavailable
	}
	key, err := base64.StdEncoding.DecodeString(raw)
	if err != nil || len(key) != 32 {
		return nil, errors.New("AGENTE_IA_SECRETS_KEY deve ser base64 de 32 bytes")
	}
	return key, nil
}

func ValidateKey() error {
	_, err := masterKey()
	return err
}

func DeriveKey(scope string) ([]byte, error) {
	key, err := masterKey()
	if err != nil {
		return nil, err
	}
	scope = strings.TrimSpace(scope)
	if scope == "" {
		return nil, errors.New("scope de derivação obrigatório")
	}
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte("notreve-agente-ia/v1/" + scope))
	return mac.Sum(nil), nil
}

func IsEncrypted(value string) bool {
	return strings.HasPrefix(strings.TrimSpace(value), prefix)
}

func LookupHash(value string) []byte {
	sum := sha256.Sum256([]byte(strings.TrimSpace(value)))
	return sum[:]
}

func Encrypt(scope, value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	if IsEncrypted(value) {
		return value, nil
	}
	key, err := masterKey()
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := gcm.Seal(nil, nonce, []byte(value), []byte(scope))
	payload := append(nonce, sealed...)
	return prefix + base64.RawURLEncoding.EncodeToString(payload), nil
}

func Decrypt(scope, value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	if !IsEncrypted(value) {
		return "", ErrPlaintextSecret
	}
	key, err := masterKey()
	if err != nil {
		return "", err
	}
	payload, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(value, prefix))
	if err != nil {
		return "", fmt.Errorf("segredo cifrado inválido: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(payload) < gcm.NonceSize()+gcm.Overhead() {
		return "", errors.New("segredo cifrado truncado")
	}
	plain, err := gcm.Open(nil, payload[:gcm.NonceSize()], payload[gcm.NonceSize():], []byte(scope))
	if err != nil {
		return "", errors.New("segredo cifrado não pôde ser autenticado")
	}
	return string(plain), nil
}
