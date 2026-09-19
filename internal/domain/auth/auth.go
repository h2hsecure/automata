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

// TenantInfo holds tenant details returned during login.
type TenantInfo struct {
	ID   string `json:"id"`
	Slug string `json:"slug"`
	Name string `json:"name"`
}

// AuthService defines the interface for authentication operations.
type AuthService interface {
	Login(email, password string) (token string, userID string, tenants []TenantInfo, err error)
	VerifyToken(token string) (userID string, tenantID string, err error)
	VerifyTokenUserOnly(token string) (userID string, err error)
	CreateAPIKey(userID, tenantID, name string, plaintextKey string, expiresAt *time.Time) (*APIKey, error)
	ValidateAPIKey(key string) (*APIKey, error)
	HashPassword(password string) (string, error)
	ComparePassword(hash, password string) error
}
