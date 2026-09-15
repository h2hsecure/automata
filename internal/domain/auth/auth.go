package auth

import "time"

// APIKey represents a tenant API key.
type APIKey struct {
	ID        string
	TenantID  string
	UserID    string
	KeyHash   string
	Name      string
	ExpiresAt *time.Time
	CreatedAt time.Time
}

// AuthService defines the interface for authentication operations.
type AuthService interface {
	Login(email, password, tenantSlug string) (token string, userID string, err error)
	VerifyToken(token string) (userID string, tenantID string, err error)
	CreateAPIKey(userID, tenantID, name string, plaintextKey string, expiresAt *time.Time) (*APIKey, error)
	ValidateAPIKey(key string) (*APIKey, error)
	HashPassword(password string) (string, error)
	ComparePassword(hash, password string) error
}
