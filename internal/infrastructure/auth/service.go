package auth

import (
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	domainauth "github.com/hekemen/automata/internal/domain/auth"
	"github.com/hekemen/automata/internal/domain/context"
	"github.com/hekemen/automata/internal/infrastructure/config"
	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/bcrypt"
)

type service struct {
	userRepo context.UserRepository
}

// NewService creates a new auth service.
func NewService(userRepo context.UserRepository) domainauth.AuthService {
	return &service{userRepo: userRepo}
}

// HashPassword hashes a password using bcrypt.
func (s *service) HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(bytes), nil
}

// ComparePassword compares a password against a bcrypt hash.
func (s *service) ComparePassword(hash, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}

// Login authenticates a user by email and password, returns a JWT token and context list.
func (s *service) Login(email, password string) (string, string, []domainauth.ContextInfo, error) {
	secretKey := config.Get("auth.secret_key")
	if secretKey == "" {
		secretKey = "automata-dev-secret-key-change-in-production"
	}

	if s.userRepo == nil {
		return "", "", nil, fmt.Errorf("user repository not initialized")
	}

	// Query users table first (platform-level), fall back to context_users
	var u *context.User
	u, err := s.userRepo.GetByUsername(email)
	if err != nil {
		// Fall back to context_users for backward compatibility
		users, fallbackErr := s.userRepo.GetByEmailGlobal(email)
		if fallbackErr != nil || len(users) == 0 {
			return "", "", nil, fmt.Errorf("invalid email or password")
		}
		u = users[0]
	}

	if u.PasswordHash == "" {
		return "", "", nil, fmt.Errorf("invalid email or password")
	}

	if err := s.ComparePassword(u.PasswordHash, password); err != nil {
		return "", "", nil, fmt.Errorf("invalid email or password")
	}

	// Get all context memberships for this user
	memberships, err := s.userRepo.ListByUser(u.ID)
	if err != nil || len(memberships) == 0 {
		// Fallback: if no memberships found, use context_users list
		users, fallbackErr := s.userRepo.GetByEmailGlobal(email)
		if fallbackErr == nil && len(users) > 0 {
			memberships = make([]context.UserContext, 0, len(users))
			for _, ctxUser := range users {
				memberships = append(memberships, context.UserContext{
					ContextID: ctxUser.ContextID,
					Role:      "owner",
				})
			}
		} else {
			memberships = make([]context.UserContext, 0)
		}
	}

	// Build context list with roles
	contexts := make([]domainauth.ContextInfo, 0, len(memberships))
	for _, mc := range memberships {
		contexts = append(contexts, domainauth.ContextInfo{
			ID:   mc.ContextID,
			Slug: "", // Will be filled by handler from contextRepo
			Name: "", // Will be filled by handler from contextRepo
			Role: mc.Role,
		})
	}

	// Generate JWT with is_admin claim
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id":    u.ID,
		"user_email": email,
		"is_admin":   u.IsAdmin,
		"exp":        time.Now().Add(24 * time.Hour).Unix(),
	})

	tokenString, err := token.SignedString([]byte(secretKey))
	if err != nil {
		log.Error().Err(err).Msg("token generation failed")
		return "", "", nil, fmt.Errorf("generate token: %w", err)
	}

	return tokenString, u.ID, contexts, nil
}

// VerifyToken verifies a JWT token and returns the user ID and context ID.
func (s *service) VerifyToken(tokenStr string) (string, string, error) {
	secretKey := config.Get("auth.secret_key")
	if secretKey == "" {
		secretKey = "automata-dev-secret-key-change-in-production"
	}

	tokenStr = strings.TrimPrefix(tokenStr, "Bearer ")
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(secretKey), nil
	})

	if err != nil || !token.Valid {
		return "", "", fmt.Errorf("invalid token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", "", fmt.Errorf("invalid token claims")
	}

	userID, ok := claims["user_id"].(string)
	if !ok {
		return "", "", fmt.Errorf("missing user_id in token")
	}

	contextSlug, ok := claims["context_slug"].(string)
	if !ok {
		return "", "", fmt.Errorf("missing context_slug in token")
	}

	return userID, contextSlug, nil
}

// VerifyTokenUserOnly verifies a JWT token and returns only the user ID.
// It does not require a context claim, making it suitable for the new global login flow.
func (s *service) VerifyTokenUserOnly(tokenStr string) (string, error) {
	secretKey := config.Get("auth.secret_key")
	if secretKey == "" {
		secretKey = "automata-dev-secret-key-change-in-production"
	}

	tokenStr = strings.TrimPrefix(tokenStr, "Bearer ")
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(secretKey), nil
	})

	if err != nil || !token.Valid {
		return "", fmt.Errorf("invalid token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", fmt.Errorf("invalid token claims")
	}

	userID, ok := claims["user_id"].(string)
	if !ok {
		return "", fmt.Errorf("missing user_id in token")
	}

	return userID, nil
}

// CreateAPIKey creates a new API key with bcrypt-hashed value.
func (s *service) CreateAPIKey(userID, contextID, name, plaintextKey string, expiresAt *time.Time) (*domainauth.APIKey, error) {
	keyHash, err := bcrypt.GenerateFromPassword([]byte(plaintextKey), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash api key: %w", err)
	}

	return &domainauth.APIKey{
		UserID:    userID,
		ContextID:  contextID,
		KeyHash:   string(keyHash),
		Name:      name,
		ExpiresAt: expiresAt,
	}, nil
}

// ValidateAPIKey compares a plaintext API key against stored hashes.
func (s *service) ValidateAPIKey(key string) (*domainauth.APIKey, error) {
	// This is a stub - actual implementation will query the database
	return nil, fmt.Errorf("validate api key requires database repository")
}

// GetCurrentUser returns the platform user for the given context.
func (s *service) GetCurrentUser(contextID string) (*context.User, error) {
	// This is a stub - actual implementation will query the database
	return nil, fmt.Errorf("get current user requires database repository")
}
