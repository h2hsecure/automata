package auth

import (
	stdcontext "context"
	"fmt"
	"time"

	"github.com/h2hsecure/automata/internal/domain/context"
)

// ErrDuplicateKey is returned when an API key name already exists in a context.
var ErrDuplicateKey = fmt.Errorf("api key name already exists for this context")

// APIKey represents a context API key.
type APIKey struct {
	ID        string
	ContextID string
	UserID    string
	KeyHash   string
	Name      string
	ExpiresAt *time.Time
	CreatedAt time.Time
	// ContextName/ContextSlug are display-only fields populated by
	// ListAllWithContexts for the admin listing (WebUI Website column).
	ContextName string
	ContextSlug string
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
	// InsertRefreshToken registers a refresh token jti minted at login so
	// RefreshToken can enforce rotation and detect reuse. No-op for services
	// built without server-side rotation.
	InsertRefreshToken(ctx stdcontext.Context, jti, userID string, expiresAt time.Time) error
	RefreshToken(refreshToken string) (accessToken string, newRefreshToken string, err error)
	VerifyToken(token string) (userID string, contextID string, err error)
	VerifyTokenUserOnly(token string) (userID string, err error)
	CreateAPIKey(userID, contextID, name string, plaintextKey string, expiresAt *time.Time) (*APIKey, error)
	ValidateAPIKey(key string) (*APIKey, error)
	HashPassword(password string) (string, error)
	ComparePassword(hash, password string) error
	GetCurrentUser(contextID string) (*context.User, error)
	GetUserByID(userID string) (*context.User, error)
	ChangePassword(userID, currentPassword, newPassword string) error
	LogoutAllSessions(userID string) error
}
