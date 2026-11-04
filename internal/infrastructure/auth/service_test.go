package auth

import (
	"bytes"
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

const testJWTSecret = "test-secret-key-for-jwt-signing-32bytes"
const testCookieSecret = "test-cookie-secret-for-testing-32bytes!!"

func TestMain(m *testing.M) {
	// Initialize secrets for all auth tests
	InitForTesting(testJWTSecret, testCookieSecret)
	m.Run()
	Reset()
}

func TestRefreshToken_HappyPath(t *testing.T) {
	s := &service{}

	// Create a valid refresh token
	secretKey := testJWTSecret
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
	secretKey := testJWTSecret
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
	secretKey := testJWTSecret
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
	secretKey := testJWTSecret
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

	secretKey := testJWTSecret
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
	secretKey := testJWTSecret

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

// stubRefreshStore is an in-memory RefreshTokenStore for service tests.
type stubRefreshStore struct {
	mu       sync.Mutex
	active   map[string]bool
	revoked  []string
	inserted []string
}

func newStubRefreshStore() *stubRefreshStore {
	return &stubRefreshStore{active: map[string]bool{}}
}

func (s *stubRefreshStore) Insert(_ context.Context, jti, _ string, _ time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.active[jti] = true
	s.inserted = append(s.inserted, jti)
	return nil
}

func (s *stubRefreshStore) RevokeIfActive(_ context.Context, jti string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active[jti] {
		delete(s.active, jti)
		s.revoked = append(s.revoked, jti)
		return "ok", nil
	}
	for _, r := range s.revoked {
		if r == jti {
			return "already_revoked", nil
		}
	}
	return "missing", nil
}

func (s *stubRefreshStore) Prune(_ context.Context, _ time.Time) (int64, error) {
	return 0, nil
}

func (s *stubRefreshStore) activeCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.active)
}

func (s *stubRefreshStore) isActive(jti string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.active[jti]
}

func makeRefreshToken(t *testing.T, userID, jti string) string {
	t.Helper()
	claims := jwt.MapClaims{
		"user_id": userID,
		"type":    "refresh",
		"exp":     time.Now().Add(7 * 24 * time.Hour).Unix(),
		"iat":     time.Now().Unix(),
	}
	if jti != "" {
		claims["jti"] = jti
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(testJWTSecret))
	if err != nil {
		t.Fatalf("failed to create refresh token: %v", err)
	}
	return token
}

func TestRefreshToken_Rotation(t *testing.T) {
	store := newStubRefreshStore()
	s := &service{refreshStore: store}

	// Simulate login: handler mints the refresh JWT and registers its jti.
	token1 := makeRefreshToken(t, "test-user-123", "jti-1")
	if err := store.Insert(context.Background(), "jti-1", "test-user-123", time.Now().Add(7*24*time.Hour)); err != nil {
		t.Fatalf("insert: %v", err)
	}

	// First refresh succeeds and rotates.
	_, token2, err := s.RefreshToken(token1)
	if err != nil {
		t.Fatalf("first refresh failed: %v", err)
	}
	if store.isActive("jti-1") {
		t.Error("old jti should be revoked after rotation")
	}
	if store.activeCount() != 1 {
		t.Errorf("expected exactly 1 active refresh token after rotation, got %d", store.activeCount())
	}

	// The newly issued refresh token works.
	_, _, err = s.RefreshToken(token2)
	if err != nil {
		t.Fatalf("second refresh (with rotated token) failed: %v", err)
	}

	// Replaying the original token is rejected (reuse detection).
	_, _, err = s.RefreshToken(token1)
	if err != ErrRefreshTokenInvalid {
		t.Fatalf("expected ErrRefreshTokenInvalid for replayed token, got: %v", err)
	}
}

func TestRefreshToken_ReuseRejected(t *testing.T) {
	store := newStubRefreshStore()
	s := &service{refreshStore: store}

	token := makeRefreshToken(t, "test-user-123", "jti-reuse")
	store.Insert(context.Background(), "jti-reuse", "test-user-123", time.Now().Add(7*24*time.Hour))

	// Capture warn logs to assert the theft signal is logged.
	var buf bytes.Buffer
	origLogger := log.Logger
	log.Logger = zerolog.New(&buf).Level(zerolog.WarnLevel)
	defer func() { log.Logger = origLogger }()

	if _, _, err := s.RefreshToken(token); err != nil {
		t.Fatalf("first refresh failed: %v", err)
	}
	if _, _, err := s.RefreshToken(token); err != ErrRefreshTokenInvalid {
		t.Fatalf("expected ErrRefreshTokenInvalid on reuse, got: %v", err)
	}
	if !strings.Contains(buf.String(), "refresh token reuse detected") {
		t.Errorf("expected reuse warning in logs, got: %s", buf.String())
	}
}

func TestRefreshToken_MissingJtiRejectedWithStore(t *testing.T) {
	store := newStubRefreshStore()
	s := &service{refreshStore: store}

	// A refresh token without a jti cannot be tracked server-side.
	token := makeRefreshToken(t, "test-user-123", "")
	_, _, err := s.RefreshToken(token)
	if err != ErrRefreshTokenInvalid {
		t.Fatalf("expected ErrRefreshTokenInvalid for jti-less token with store, got: %v", err)
	}
}

func TestRefreshToken_UnknownJtiRejected(t *testing.T) {
	store := newStubRefreshStore()
	s := &service{refreshStore: store}

	// jti was never registered (e.g. token minted before rotation existed).
	token := makeRefreshToken(t, "test-user-123", "jti-unknown")
	_, _, err := s.RefreshToken(token)
	if err != ErrRefreshTokenInvalid {
		t.Fatalf("expected ErrRefreshTokenInvalid for unknown jti, got: %v", err)
	}
}
