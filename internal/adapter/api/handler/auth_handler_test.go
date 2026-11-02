package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/h2hsecure/automata/internal/domain/auth"
	"github.com/h2hsecure/automata/internal/domain/context"
)

// fakeAuthService implements auth.AuthService for handler tests.
type fakeAuthService struct{}

func (f *fakeAuthService) Login(email, password string) (string, string, []auth.ContextInfo, error) {
	return "token", "user-1", nil, nil
}
func (f *fakeAuthService) RefreshToken(refreshToken string) (string, string, error) {
	return "token", "refresh", nil
}
func (f *fakeAuthService) VerifyToken(token string) (string, string, error) {
	return "user-1", "ctx-1", nil
}
func (f *fakeAuthService) VerifyTokenUserOnly(token string) (string, error) {
	return "user-1", nil
}
func (f *fakeAuthService) CreateAPIKey(userID, contextID, name string, plaintextKey string, expiresAt *time.Time) (*auth.APIKey, error) {
	return &auth.APIKey{
		ID:        "key-1",
		UserID:    userID,
		ContextID: contextID,
		KeyHash:   "hash",
		Name:      name,
		ExpiresAt: expiresAt,
	}, nil
}
func (f *fakeAuthService) ValidateAPIKey(key string) (*auth.APIKey, error) {
	return nil, errors.New("not found")
}
func (f *fakeAuthService) HashPassword(password string) (string, error) {
	return "hash", nil
}
func (f *fakeAuthService) ComparePassword(hash, password string) error { return nil }
func (f *fakeAuthService) GetCurrentUser(contextID string) (*context.User, error) {
	return &context.User{ID: "user-1"}, nil
}
func (f *fakeAuthService) GetUserByID(userID string) (*context.User, error) {
	return &context.User{ID: userID}, nil
}
func (f *fakeAuthService) ChangePassword(userID, currentPassword, newPassword string) error {
	return nil
}
func (f *fakeAuthService) LogoutAllSessions(userID string) error { return nil }

// fakeApiKeyRepo implements auth_repo.ApiKeyRepository for handler tests.
type fakeApiKeyRepo struct {
	createErr error
	keys      map[string]*auth.APIKey
	deleted   map[string]int
}

func (f *fakeApiKeyRepo) Create(key *auth.APIKey) error { return f.createErr }
func (f *fakeApiKeyRepo) ListByContext(contextID string) ([]*auth.APIKey, error) {
	return nil, nil
}
func (f *fakeApiKeyRepo) ListAll() ([]*auth.APIKey, error) { return nil, nil }
func (f *fakeApiKeyRepo) Delete(id string) error {
	if f.deleted == nil {
		f.deleted = map[string]int{}
	}
	f.deleted[id]++
	return nil
}
func (f *fakeApiKeyRepo) GetByHash(hash string) (*auth.APIKey, error) {
	return nil, errors.New("not found")
}
func (f *fakeApiKeyRepo) GetByPrefix(prefix, contextID string) (*auth.APIKey, error) {
	return nil, errors.New("not found")
}
func (f *fakeApiKeyRepo) GetByID(id, contextID string) (*auth.APIKey, error) {
	if key, ok := f.keys[id]; ok && key.ContextID == contextID {
		return key, nil
	}
	return nil, errors.New("not found")
}

// newCreateAPIKeyTestRouter builds a router that emulates the auth middleware
// by pre-setting the user_id / context_id gin context values before calling
// the real handler.
func newCreateAPIKeyTestRouter(repo *fakeApiKeyRepo) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := NewAuthHandler(&fakeAuthService{}, nil, nil, repo, nil)
	r := gin.New()
	r.POST("/admin/api-keys", func(c *gin.Context) {
		c.Set("user_id", "user-1")
		c.Set("context_id", "ctx-1")
		h.CreateAPIKey(c)
	})
	return r
}

func TestCreateAPIKeyDuplicateNameReturnsConflict(t *testing.T) {
	r := newCreateAPIKeyTestRouter(&fakeApiKeyRepo{createErr: auth.ErrDuplicateKey})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/admin/api-keys", bytes.NewBufferString(`{"name":"Production API Key"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d (body: %s)", w.Code, w.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON body: %v", err)
	}
	errMsg, _ := resp["error"].(string)
	if errMsg == "" {
		t.Fatalf("expected error message in response, got %v", resp)
	}
}

func TestCreateAPIKeyOtherRepoErrorReturns500(t *testing.T) {
	r := newCreateAPIKeyTestRouter(&fakeApiKeyRepo{createErr: errors.New("db down")})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/admin/api-keys", bytes.NewBufferString(`{"name":"Production API Key"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d (body: %s)", w.Code, w.Body.String())
	}
}

// newRevokeAPIKeyTestRouter emulates the auth middleware by pre-setting the
// user_id / context_id gin context values, mirroring newCreateAPIKeyTestRouter.
func newRevokeAPIKeyTestRouter(repo *fakeApiKeyRepo) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := NewAuthHandler(&fakeAuthService{}, nil, nil, repo, nil)
	r := gin.New()
	r.DELETE("/admin/api-keys/:id", func(c *gin.Context) {
		c.Set("user_id", "user-1")
		c.Set("context_id", "ctx-1")
		h.RevokeAPIKey(c)
	})
	return r
}

// The webui passes the key ID (UUID) returned by ListAPIKeys, so revocation
// must resolve the key by ID — not by plaintext prefix (a UUID is never a
// prefix of the stored bcrypt hash).
func TestRevokeAPIKeyExistingKeySucceeds(t *testing.T) {
	keyID := "11111111-1111-1111-1111-111111111111"
	repo := &fakeApiKeyRepo{
		keys: map[string]*auth.APIKey{
			keyID: {ID: keyID, ContextID: "ctx-1", Name: "Production API Key"},
		},
	}
	r := newRevokeAPIKeyTestRouter(repo)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/admin/api-keys/"+keyID, nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", w.Code, w.Body.String())
	}
	if repo.deleted[keyID] != 1 {
		t.Fatalf("expected Delete to be called once for %s, got %d", keyID, repo.deleted[keyID])
	}
}

func TestRevokeAPIKeyUnknownKeyReturnsNotFound(t *testing.T) {
	keyID := "22222222-2222-2222-2222-222222222222"
	r := newRevokeAPIKeyTestRouter(&fakeApiKeyRepo{})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/admin/api-keys/"+keyID, nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d (body: %s)", w.Code, w.Body.String())
	}
}

func TestRevokeAPIKeyOtherContextReturnsNotFound(t *testing.T) {
	keyID := "33333333-3333-3333-3333-333333333333"
	repo := &fakeApiKeyRepo{
		keys: map[string]*auth.APIKey{
			keyID: {ID: keyID, ContextID: "other-ctx", Name: "Other Context Key"},
		},
	}
	r := newRevokeAPIKeyTestRouter(repo)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/admin/api-keys/"+keyID, nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d (body: %s)", w.Code, w.Body.String())
	}
	if repo.deleted[keyID] != 0 {
		t.Fatalf("expected Delete not to be called for a foreign-context key, got %d", repo.deleted[keyID])
	}
}
