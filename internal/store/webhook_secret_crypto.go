package store

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"io"

	"github.com/notrevetecnologia/notreve-chat-community/internal/securesecret"
)

var (
	ErrWebhookSecretStorageUnavailable = errors.New("webhook signing secret storage unavailable")
	ErrWebhookSigningSecretUnavailable = errors.New("webhook signing secret unavailable")
)

type webhookSecretCipher struct {
	gcm cipher.AEAD
}

func newWebhookSecretCipher() (*webhookSecretCipher, error) {
	key, err := securesecret.DeriveKey("webhook-signing-secrets")
	if err != nil {
		return nil, ErrWebhookSecretStorageUnavailable
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, ErrWebhookSecretStorageUnavailable
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, ErrWebhookSecretStorageUnavailable
	}
	return &webhookSecretCipher{gcm: gcm}, nil
}

func (s *Store) webhookSecretCipher() (*webhookSecretCipher, error) {
	return newWebhookSecretCipher()
}

func (s *Store) encryptWebhookSigningSecret(secret string) ([]byte, []byte, error) {
	crypt, err := s.webhookSecretCipher()
	if err != nil {
		return nil, nil, err
	}
	nonce := make([]byte, crypt.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, ErrWebhookSecretStorageUnavailable
	}
	return crypt.gcm.Seal(nil, nonce, []byte(secret), []byte("webhook-signing-secrets")), nonce, nil
}

func (s *Store) decryptWebhookSigningSecret(ciphertext, nonce []byte) (string, error) {
	if len(ciphertext) == 0 || len(nonce) == 0 {
		return "", ErrWebhookSigningSecretUnavailable
	}
	crypt, err := s.webhookSecretCipher()
	if err != nil {
		return "", err
	}
	if len(nonce) != crypt.gcm.NonceSize() {
		return "", ErrWebhookSigningSecretUnavailable
	}
	plaintext, err := crypt.gcm.Open(nil, nonce, ciphertext, []byte("webhook-signing-secrets"))
	if err != nil {
		return "", ErrWebhookSigningSecretUnavailable
	}
	return string(plaintext), nil
}
