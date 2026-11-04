package middleware

import (
	stdcontext "context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/h2hsecure/automata/internal/domain/context"
)

// stubUserRepo implements context.UserRepository with a single user.
type stubUserRepo struct {
	user        *context.User
	memberships map[string]string // userID -> contextID
}

func (s *stubUserRepo) Create(u *context.User) error { return nil }
func (s *stubUserRepo) GetByID(id string) (*context.User, error) {
	if s.user == nil || s.user.ID != id {
		return nil, errors.New("user not found")
	}
	return s.user, nil
}
func (s *stubUserRepo) GetByEmail(contextID, email string) (*context.User, error) {
	return nil, errors.New("not found")
}
func (s *stubUserRepo) GetByEmailGlobal(email string) ([]*context.User, error) {
	return nil, nil
}
func (s *stubUserRepo) GetByUsername(email string) (*context.User, error) {
	return nil, errors.New("not found")
}
func (s *stubUserRepo) ExistsAnyUser() (bool, error)      { return false, nil }
func (s *stubUserRepo) ListAll() ([]*context.User, error) { return nil, nil }
func (s *stubUserRepo) Update(u *context.User) error      { return nil }
func (s *stubUserRepo) Delete(id string) error            { return nil }
func (s *stubUserRepo) UpdateProfile(userID string, displayName, avatarURL *string) error {
	return nil
}
func (s *stubUserRepo) ChangePassword(userID, passwordHash string) error { return nil }
func (s *stubUserRepo) GetPasswordChangedAt(userID string) (time.Time, error) {
	return time.Time{}, nil
}
func (s *stubUserRepo) UpdatePasswordChangedAt(userID string) error { return nil }
func (s *stubUserRepo) GetByUserIDContext(ctx stdcontext.Context, userID, contextID string) (*context.UserContext, error) {
	if s.memberships[userID] == contextID {
		return &context.UserContext{UserID: userID, ContextID: contextID, Role: "owner"}, nil
	}
	return nil, errors.New("not found")
}
func (s *stubUserRepo) ListByUser(userID string) ([]context.UserContext, error) {
	return nil, nil
}
func (s *stubUserRepo) CreateMembership(uc *context.UserContext) error { return nil }
func (s *stubUserRepo) UpdateMembership(uc *context.UserContext) error { return nil }
func (s *stubUserRepo) DeleteMembership(userID, contextID string) error {
	return nil
}
func (s *stubUserRepo) ListMembers(contextID string) ([]context.UserContext, error) {
	return nil, nil
}

var _ context.UserRepository = (*stubUserRepo)(nil)

// setupAdminEngine builds a gin engine where /admin/thing requires admin,
// with a fake auth middleware that sets user_id first (as AuthMiddleware does).
func setupAdminEngine(user *context.User, handler gin.HandlerFunc) *gin.Engine {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	fakeAuth := func(c *gin.Context) {
		c.Set("user_id", "u1")
		c.Next()
	}
	engine.GET("/admin/thing", fakeAuth, RequireAdmin(&stubUserRepo{user: user}), handler)
	return engine
}

func TestRequireAdmin_NonAdminGets403(t *testing.T) {
	engine := setupAdminEngine(&context.User{ID: "u1", IsAdmin: false}, func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	req := httptest.NewRequest(http.MethodGet, "/admin/thing", nil)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d (body %s)", w.Code, w.Body.String())
	}
}

func TestRequireAdmin_AdminPasses(t *testing.T) {
	engine := setupAdminEngine(&context.User{ID: "u1", IsAdmin: true}, func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	req := httptest.NewRequest(http.MethodGet, "/admin/thing", nil)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body %s)", w.Code, w.Body.String())
	}
}

func TestRequireAdmin_MissingUserGets403(t *testing.T) {
	// No user row: GetByID errors → treat as non-admin.
	engine := setupAdminEngine(nil, func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	req := httptest.NewRequest(http.MethodGet, "/admin/thing", nil)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}