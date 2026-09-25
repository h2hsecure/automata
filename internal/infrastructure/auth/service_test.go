package auth

import (
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/hekemen/automata/internal/infrastructure/config"
)

func TestMain(m *testing.M) {
	// Set up a known secret key for testing
	config.Set("auth.secret_key", "test-secret-key-for-jwt-signing-32bytes")
	m.Run()
	config.Reset()
}

func TestRefreshToken_HappyPath(t *testing.T) {
	s := &service{}

	// Create a valid refresh token
	secretKey := config.Get("auth.secret_key")
	refreshClaims := jwt.MapClaims{
		"user_id":    "test-user-123",
		"user_email": "test@example.com",
		"is_admin":   true,
		"type":       "refresh",
		"exp":        time.Now().Add(7 * 24 * time.Hour).Unix(),
		"iat":        time.Now().Unix(),
	}
	refresh := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
	refreshToken, err := refresh.SignedString([]byte(secretKey))
	if err != nil {
		t.Fatalf("failed to create refresh token: %v", err)
	}

	accessToken, newRefreshToken, err := s.RefreshToken(refreshToken)
	if err != nil {
		t.Fatalf("RefreshToken returned error: %v", err)
	}
	if accessToken == "" {
		t.Fatal("expected non-empty access token")
	}
	if newRefreshToken == "" {
		t.Fatal("expected non-empty new refresh token")
	}
	if accessToken == refreshToken {
		t.Fatal("access token should be different from refresh token")
	}

	// Verify access token claims
	atClaims, err := jwt.Parse(accessToken, func(t *jwt.Token) (interface{}, error) {
		return []byte(secretKey), nil
	})
	if err != nil {
		t.Fatalf("failed to parse access token: %v", err)
	}
	mc := atClaims.Claims.(jwt.MapClaims)
	if mc["user_id"] != "test-user-123" {
		t.Errorf("expected user_id to be test-user-123, got %v", mc["user_id"])
	}
	if mc["is_admin"] != true {
		t.Error("expected is_admin to be true")
	}

	// Verify new refresh token has type=refresh
	nrtClaims, err := jwt.Parse(newRefreshToken, func(t *jwt.Token) (interface{}, error) {
		return []byte(secretKey), nil
	})
	if err != nil {
		t.Fatalf("failed to parse new refresh token: %v", err)
	}
	nrtmc := nrtClaims.Claims.(jwt.MapClaims)
	if nrtmc["type"] != "refresh" {
		t.Errorf("expected type to be refresh, got %v", nrtmc["type"])
	}
}

func TestRefreshToken_Expired(t *testing.T) {
	s := &service{}

	// Create an expired refresh token
	secretKey := config.Get("auth.secret_key")
	refreshClaims := jwt.MapClaims{
		"user_id":    "test-user-123",
		"type":       "refresh",
		"exp":        time.Now().Add(-1 * time.Hour).Unix(),
		"iat":        time.Now().Add(-8 * 24 * time.Hour).Unix(),
	}
	refresh := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
	refreshToken, err := refresh.SignedString([]byte(secretKey))
	if err != nil {
		t.Fatalf("failed to create refresh token: %v", err)
	}

	_, _, err = s.RefreshToken(refreshToken)
	if err != ErrRefreshTokenExpired {
		t.Fatalf("expected ErrRefreshTokenExpired, got: %v", err)
	}
}

func TestRefreshToken_InvalidType(t *testing.T) {
	s := &service{}

	// Create an access token (type=access, not type=refresh)
	secretKey := config.Get("auth.secret_key")
	accessClaims := jwt.MapClaims{
		"user_id":    "test-user-123",
		"type":       "access",
		"exp":        time.Now().Add(24 * time.Hour).Unix(),
		"iat":        time.Now().Unix(),
	}
	access := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
	accessToken, err := access.SignedString([]byte(secretKey))
	if err != nil {
		t.Fatalf("failed to create access token: %v", err)
	}

	_, _, err = s.RefreshToken(accessToken)
	if err != ErrRefreshTokenInvalid {
		t.Fatalf("expected ErrRefreshTokenInvalid, got: %v", err)
	}
}

func TestRefreshToken_InvalidSignature(t *testing.T) {
	s := &service{}

	// Create a refresh token signed with wrong key
	wrongKey := "wrong-secret-key-different-from-config"
	refreshClaims := jwt.MapClaims{
		"user_id":    "test-user-123",
		"type":       "refresh",
		"exp":        time.Now().Add(7 * 24 * time.Hour).Unix(),
		"iat":        time.Now().Unix(),
	}
	refresh := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
	refreshToken, err := refresh.SignedString([]byte(wrongKey))
	if err != nil {
		t.Fatalf("failed to create refresh token: %v", err)
	}

	_, _, err = s.RefreshToken(refreshToken)
	if err != ErrRefreshTokenInvalid {
		t.Fatalf("expected ErrRefreshTokenInvalid, got: %v", err)
	}
}

func TestRefreshToken_BearerPrefix(t *testing.T) {
	s := &service{}

	// Create a valid refresh token
	secretKey := config.Get("auth.secret_key")
	refreshClaims := jwt.MapClaims{
		"user_id":    "test-user-123",
		"type":       "refresh",
		"exp":        time.Now().Add(7 * 24 * time.Hour).Unix(),
		"iat":        time.Now().Unix(),
	}
	refresh := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
	refreshToken, err := refresh.SignedString([]byte(secretKey))
	if err != nil {
		t.Fatalf("failed to create refresh token: %v", err)
	}

	// Test with Bearer prefix
	tokenWithPrefix := "Bearer " + refreshToken
	accessToken, newRefreshToken, err := s.RefreshToken(tokenWithPrefix)
	if err != nil {
		t.Fatalf("RefreshToken with Bearer prefix returned error: %v", err)
	}
	if accessToken == "" || newRefreshToken == "" {
		t.Fatal("expected non-empty tokens")
	}
}

func TestRefreshToken_MissingUserID(t *testing.T) {
	s := &service{}

	secretKey := config.Get("auth.secret_key")
	refreshClaims := jwt.MapClaims{
		"type":  "refresh",
		"exp":   time.Now().Add(7 * 24 * time.Hour).Unix(),
		"iat":   time.Now().Unix(),
	}
	refresh := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
	refreshToken, err := refresh.SignedString([]byte(secretKey))
	if err != nil {
		t.Fatalf("failed to create refresh token: %v", err)
	}

	_, _, err = s.RefreshToken(refreshToken)
	if err != ErrRefreshTokenInvalid {
		t.Fatalf("expected ErrRefreshTokenInvalid for missing user_id, got: %v", err)
	}
}

func TestRefreshToken_TrimsBearerPrefix(t *testing.T) {
	s := &service{}
	secretKey := config.Get("auth.secret_key")

	refreshClaims := jwt.MapClaims{
		"user_id":    "test-user",
		"type":       "refresh",
		"exp":        time.Now().Add(7 * 24 * time.Hour).Unix(),
		"iat":        time.Now().Unix(),
	}
	refresh := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
	refreshToken, _ := refresh.SignedString([]byte(secretKey))
	bearerToken := "Bearer " + refreshToken

	accessToken, _, err := s.RefreshToken(bearerToken)
	if err != nil {
		t.Fatalf("expected successful refresh with Bearer prefix, got: %v", err)
	}
	if !strings.HasPrefix(accessToken, "eyJ") {
		t.Fatal("expected access token to start with eyJ")
	}
}
