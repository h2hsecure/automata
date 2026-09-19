package auth

import (
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	domainauth "github.com/hekemen/automata/internal/domain/auth"
	"github.com/hekemen/automata/internal/domain/tenant"
	"github.com/hekemen/automata/internal/infrastructure/config"
	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/bcrypt"
)

type service struct {
	userRepo tenant.UserRepository
}

// NewService creates a new auth service.
func NewService(userRepo tenant.UserRepository) domainauth.AuthService {
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

// Login authenticates a user by email and password, returns a JWT token and tenant list.
func (s *service) Login(email, password string) (string, string, []domainauth.TenantInfo, error) {
	secretKey := config.Get("auth.secret_key")
	if secretKey == "" {
		secretKey = "automata-dev-secret-key-change-in-production"
	}

	if s.userRepo == nil {
		return "", "", nil, fmt.Errorf("user repository not initialized")
	}

	users, err := s.userRepo.GetByEmailGlobal(email)
	if err != nil {
		return "", "", nil, fmt.Errorf("invalid email or password")
	}

	// Verify password against the first matching user
	if err := s.ComparePassword(users[0].PasswordHash, password); err != nil {
		return "", "", nil, fmt.Errorf("invalid email or password")
	}

	// Build tenant list for all matching users
	tenants := make([]domainauth.TenantInfo, 0, len(users))
	for _, u := range users {
		tenants = append(tenants, domainauth.TenantInfo{
			ID:   u.TenantID,
			Slug: "", // Will be filled by handler from tenantRepo
			Name: "", // Will be filled by handler from tenantRepo
		})
	}

	// Generate JWT without tenant_slug (tenant comes from header)
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id":    users[0].ID,
		"user_email": email,
		"exp":        time.Now().Add(24 * time.Hour).Unix(),
	})

	tokenString, err := token.SignedString([]byte(secretKey))
	if err != nil {
		log.Error().Err(err).Str("secret_key_prefix", string([]byte(secretKey)[:min(10, len(secretKey))])).Msg("token generation failed")
		return "", "", nil, fmt.Errorf("generate token: %w", err)
	}

	return tokenString, users[0].ID, tenants, nil
}

// VerifyToken verifies a JWT token and returns the user ID and tenant ID.
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

	tenantSlug, ok := claims["tenant_slug"].(string)
	if !ok {
		return "", "", fmt.Errorf("missing tenant_slug in token")
	}

	return userID, tenantSlug, nil
}

// VerifyTokenUserOnly verifies a JWT token and returns only the user ID.
// It does not require a tenant claim, making it suitable for the new global login flow.
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
func (s *service) CreateAPIKey(userID, tenantID, name, plaintextKey string, expiresAt *time.Time) (*domainauth.APIKey, error) {
	keyHash, err := bcrypt.GenerateFromPassword([]byte(plaintextKey), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash api key: %w", err)
	}

	return &domainauth.APIKey{
		UserID:    userID,
		TenantID:  tenantID,
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
