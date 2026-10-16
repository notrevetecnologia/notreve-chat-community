package store

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"io"
	"strings"

	"github.com/notrevetecnologia/notreve-chat-community/internal/securesecret"
)

var (
	ErrLGPDEncryptionUnavailable = errors.New("chave de criptografia LGPD indisponível")
	ErrLGPDDecryptionFailed      = errors.New("falha ao decifrar dado sensível")
)

type lgpdCipher struct {
	gcm cipher.AEAD
}

func newLGPDSensitiveCipher() (*lgpdCipher, error) {
	key, err := securesecret.DeriveKey("lgpd-sensitive-data")
	if err != nil {
		return nil, ErrLGPDEncryptionUnavailable
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, ErrLGPDEncryptionUnavailable
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, ErrLGPDEncryptionUnavailable
	}
	return &lgpdCipher{gcm: gcm}, nil
}

func (s *Store) lgpdCipher() (*lgpdCipher, error) {
	return newLGPDSensitiveCipher()
}

func (s *Store) LGPDEncryptionEnabled() bool {
	_, err := s.lgpdCipher()
	return err == nil
}

func (s *Store) EncryptLGPDSensitive(plaintext string) (ciphertext, nonce []byte, err error) {
	crypt, err := s.lgpdCipher()
	if err != nil {
		return nil, nil, err
	}
	nonce = make([]byte, crypt.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, ErrLGPDEncryptionUnavailable
	}
	return crypt.gcm.Seal(nil, nonce, []byte(plaintext), []byte("lgpd-sensitive-data")), nonce, nil
}

func (s *Store) DecryptLGPDSensitive(ciphertext, nonce []byte) (string, error) {
	if len(ciphertext) == 0 || len(nonce) == 0 {
		return "", ErrLGPDDecryptionFailed
	}
	crypt, err := s.lgpdCipher()
	if err != nil {
		return "", ErrLGPDDecryptionFailed
	}
	if len(nonce) != crypt.gcm.NonceSize() {
		return "", ErrLGPDDecryptionFailed
	}
	plaintext, err := crypt.gcm.Open(nil, nonce, ciphertext, []byte("lgpd-sensitive-data"))
	if err != nil {
		return "", ErrLGPDDecryptionFailed
	}
	return string(plaintext), nil
}

func MaskLGPDSensitiveDocument(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= 4 {
		return strings.Repeat("*", len(runes))
	}
	return strings.Repeat("*", len(runes)-4) + string(runes[len(runes)-4:])
}
