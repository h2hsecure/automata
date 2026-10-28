package auth

import (
	"context"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	domainctx "github.com/h2hsecure/automata/internal/domain/context"
)

// stubUserRepo is a minimal UserRepository stub for login tests.
type stubUserRepo struct {
	user *domainctx.User
}

func (r *stubUserRepo) GetByUsername(email string) (*domainctx.User, error) {
	return r.user, nil
}
func (r *stubUserRepo) GetByEmailGlobal(email string) ([]*domainctx.User, error) {
	return nil, nil
}
func (r *stubUserRepo) ListByUser(userID string) ([]domainctx.UserContext, error) {
	return nil, nil
}

func (r *stubUserRepo) Create(u *domainctx.User) error                                    { return nil }
func (r *stubUserRepo) GetByID(id string) (*domainctx.User, error)                         { return r.user, nil }
func (r *stubUserRepo) GetByEmail(contextID, email string) (*domainctx.User, error)       { return nil, nil }
func (r *stubUserRepo) ExistsAnyUser() (bool, error)                                       { return true, nil }
func (r *stubUserRepo) ListAll() ([]*domainctx.User, error)                                { return nil, nil }
func (r *stubUserRepo) Update(u *domainctx.User) error                                     { return nil }
func (r *stubUserRepo) Delete(id string) error                                             { return nil }
func (r *stubUserRepo) UpdateProfile(userID string, displayName, avatarURL *string) error { return nil }
func (r *stubUserRepo) ChangePassword(userID string, passwordHash string) error           { return nil }
func (r *stubUserRepo) GetPasswordChangedAt(userID string) (time.Time, error)             { return time.Time{}, nil }
func (r *stubUserRepo) UpdatePasswordChangedAt(userID string) error                        { return nil }
func (r *stubUserRepo) GetByUserIDContext(ctx context.Context, userID, contextID string) (*domainctx.UserContext, error) {
	return nil, nil
}
func (r *stubUserRepo) CreateMembership(uc *domainctx.UserContext) error        { return nil }
func (r *stubUserRepo) UpdateMembership(uc *domainctx.UserContext) error        { return nil }
func (r *stubUserRepo) DeleteMembership(userID, contextID string) error         { return nil }
func (r *stubUserRepo) ListMembers(contextID string) ([]domainctx.UserContext, error) { return nil, nil }

// loginJTI performs a login against the stub repo and returns the access token.
func loginJTI(t *testing.T, password string) string {
	t.Helper()

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	s := &service{
		userRepo: &stubUserRepo{user: &domainctx.User{
			ID:           "test-user-123",
			Email:        "test@example.com",
			PasswordHash: string(hash),
		}},
	}
	token, _, _, err := s.Login("test@example.com", password)
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}
	return token
}

// jtiOf parses a token and returns the jti claim as a string.
func jtiOf(t *testing.T, tokenStr string) string {
	t.Helper()

	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		return []byte(testJWTSecret), nil
	})
	if err != nil {
		t.Fatalf("failed to parse token: %v", err)
	}
	claims := token.Claims.(jwt.MapClaims)
	jti, ok := claims["jti"].(string)
	if !ok || jti == "" {
		t.Fatalf("expected jti claim to be a non-empty string, got %v", claims["jti"])
	}
	return jti
}

// assertValidUUID validates that s is a parseable UUID.
func assertValidUUID(t *testing.T, s string) {
	t.Helper()
	if _, err := uuid.Parse(s); err != nil {
		t.Errorf("expected jti to be a valid UUID, got %q: %v", s, err)
	}
}

func TestLogin_AccessTokenHasJTI(t *testing.T) {
	token := loginJTI(t, "Passw0rd!x")
	assertValidUUID(t, jtiOf(t, token))
}

func TestLogin_ConsecutiveLoginsHaveDifferentJTI(t *testing.T) {
	first := jtiOf(t, loginJTI(t, "Passw0rd!x"))
	second := jtiOf(t, loginJTI(t, "Passw0rd!x"))
	if first == second {
		t.Errorf("expected different jti per login, got %q twice", first)
	}
}

func TestRefreshToken_IssuedTokensHaveJTI(t *testing.T) {
	s := &service{}

	refreshClaims := jwt.MapClaims{
		"user_id": "test-user-123",
		"type":    "refresh",
		"exp":     time.Now().Add(7 * 24 * time.Hour).Unix(),
		"iat":     time.Now().Unix(),
	}
	refresh := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
	refreshToken, err := refresh.SignedString([]byte(testJWTSecret))
	if err != nil {
		t.Fatalf("failed to create refresh token: %v", err)
	}

	accessToken, newRefreshToken, err := s.RefreshToken(refreshToken)
	if err != nil {
		t.Fatalf("RefreshToken returned error: %v", err)
	}

	assertValidUUID(t, jtiOf(t, accessToken))
	assertValidUUID(t, jtiOf(t, newRefreshToken))
}