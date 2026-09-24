package auth

import (
	"time"

	"github.com/hekemen/automata/internal/domain/context"
)

// APIKey represents a context API key.
type APIKey struct {
	ID        string
	ContextID string
	UserID    string
	KeyHash   string
	Name      string
	ExpiresAt *time.Time
	CreatedAt time.Time
}

// ContextInfo holds context details returned during login.
type ContextInfo struct {
	ID   string `json:"id"`
	Slug string `json:"slug"`
	Name string `json:"name"`
	Role string `json:"role"` // owner, admin, member
}

// AuthService defines the interface for authentication operations.
type AuthService interface {
	Login(email, password string) (token string, userID string, contexts []ContextInfo, err error)
	VerifyToken(token string) (userID string, contextID string, err error)
	VerifyTokenUserOnly(token string) (userID string, err error)
	CreateAPIKey(userID, contextID, name string, plaintextKey string, expiresAt *time.Time) (*APIKey, error)
	ValidateAPIKey(key string) (*APIKey, error)
	HashPassword(password string) (string, error)
	ComparePassword(hash, password string) error
	GetCurrentUser(contextID string) (*context.User, error)
}
