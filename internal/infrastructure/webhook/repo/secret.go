package repo

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"

	"github.com/h2hsecure/automata/internal/infrastructure/auth"
)

// The webhook signing secret is stored encrypted at rest (AES-GCM) in the
// signing_secret_enc column. The format is base64(nonce || ciphertext) where
// the nonce is the AES-GCM nonce prepended to the ciphertext. Plaintext
// secrets only ever exist in memory at signing time.

// encryptSecret encrypts a webhook signing secret for storage.
func encryptSecret(plaintext string) (string, error) {
	key := auth.GetSecretKey()
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return "", fmt.Errorf("new aes cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("new gcm: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// decryptSecret decrypts a stored webhook signing secret.
func decryptSecret(stored string) (string, error) {
	if stored == "" {
		return "", nil
	}
	raw, err := base64.StdEncoding.DecodeString(stored)
	if err != nil {
		return "", fmt.Errorf("decode secret: %w", err)
	}
	key := auth.GetSecretKey()
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return "", fmt.Errorf("new aes cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("new gcm: %w", err)
	}
	nonceSize := gcm.NonceSize()
	if len(raw) < nonceSize {
		return "", fmt.Errorf("secret ciphertext too short")
	}
	plaintext, err := gcm.Open(nil, raw[:nonceSize], raw[nonceSize:], nil)
	if err != nil {
		return "", fmt.Errorf("decrypt secret: %w", err)
	}
	return string(plaintext), nil
}