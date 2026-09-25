package cookie

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"time"
)

// SessionData is the JSON payload stored in the encrypted cookie.
type SessionData struct {
	UserID      string `json:"user_id"`
	Email       string `json:"email"`
	IsAdmin     bool   `json:"is_admin"`
	ContextID   string `json:"context_id,omitempty"`
	ContextName string `json:"context_name,omitempty"`
	Token       string `json:"token"`
}

// Manager handles encryption and decryption of session cookies.
type Manager struct {
	secret string
}

// New creates a new cookie Manager with the given encryption secret.
func New(secret string) *Manager {
	if secret == "" {
		secret = "change-me-in-production"
	}
	return &Manager{secret: secret}
}

// Encrypt encrypts session data and returns a base64-encoded ciphertext.
func (m *Manager) Encrypt(data SessionData) (string, error) {
	plaintext, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("marshal session: %w", err)
	}

	block, err := aes.NewCipher([]byte(m.secret))
	if err != nil {
		return "", fmt.Errorf("create cipher: %w", err)
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create GCM: %w", err)
	}

	nonce := make([]byte, aesGCM.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}

	ciphertext := aesGCM.Seal(nonce, nonce, plaintext, nil)
	return base64.URLEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts a base64-encoded cookie value and returns the session data.
func (m *Manager) Decrypt(cookieValue string) (SessionData, error) {
	var data SessionData

	ciphertext, err := base64.URLEncoding.DecodeString(cookieValue)
	if err != nil {
		return data, fmt.Errorf("decode cookie: %w", err)
	}

	block, err := aes.NewCipher([]byte(m.secret))
	if err != nil {
		return data, fmt.Errorf("create cipher: %w", err)
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return data, fmt.Errorf("create GCM: %w", err)
	}

	nonceSize := aesGCM.NonceSize()
	if len(ciphertext) < nonceSize {
		return data, fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]

	plaintext, err := aesGCM.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return data, fmt.Errorf("decrypt cookie: %w", err)
	}

	if err := json.Unmarshal(plaintext, &data); err != nil {
		return data, fmt.Errorf("unmarshal session: %w", err)
	}

	return data, nil
}

// CookieName is the name used for the session cookie.
const CookieName = "automata_session"

// CookieMaxAge is the cookie expiration in seconds (24 hours).
const CookieMaxAge = 86400

// RefreshCookieName is the name used for the refresh token cookie.
const RefreshCookieName = "automata_refresh"

// RefreshCookieMaxAge is the cookie expiration in seconds (7 days).
const RefreshCookieMaxAge = 604800

// RefreshPayload is the JSON payload stored in the encrypted refresh cookie.
type RefreshPayload struct {
	Token     string `json:"token"`
	CreatedAt string `json:"created_at"`
}

// EncryptRefreshToken encrypts a refresh token JWT and returns a base64-encoded ciphertext.
func (m *Manager) EncryptRefreshToken(jwtToken string) (string, error) {
	payload := RefreshPayload{
		Token:     jwtToken,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	plaintext, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal refresh payload: %w", err)
	}

	block, err := aes.NewCipher([]byte(m.secret))
	if err != nil {
		return "", fmt.Errorf("create cipher: %w", err)
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create GCM: %w", err)
	}

	nonce := make([]byte, aesGCM.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}

	ciphertext := aesGCM.Seal(nonce, nonce, plaintext, nil)
	return base64.URLEncoding.EncodeToString(ciphertext), nil
}

// DecryptRefreshToken decrypts a base64-encoded refresh cookie value and returns the JWT token.
func (m *Manager) DecryptRefreshToken(cookieValue string) (string, error) {
	ciphertext, err := base64.URLEncoding.DecodeString(cookieValue)
	if err != nil {
		return "", fmt.Errorf("decode refresh cookie: %w", err)
	}

	block, err := aes.NewCipher([]byte(m.secret))
	if err != nil {
		return "", fmt.Errorf("create cipher: %w", err)
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create GCM: %w", err)
	}

	nonceSize := aesGCM.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]

	plaintext, err := aesGCM.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt refresh cookie: %w", err)
	}

	var payload RefreshPayload
	if err := json.Unmarshal(plaintext, &payload); err != nil {
		return "", fmt.Errorf("unmarshal refresh payload: %w", err)
	}

	return payload.Token, nil
}
