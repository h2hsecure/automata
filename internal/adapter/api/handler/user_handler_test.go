package handler

import (
	stdcontext "context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/h2hsecure/automata/internal/domain/context"
)

// recordingUserRepo captures created/updated users.
type recordingUserRepo struct {
	created *context.User
	existing *context.User
}

func (r *recordingUserRepo) Create(u *context.User) error {
	r.created = u
	return nil
}
func (r *recordingUserRepo) GetByID(id string) (*context.User, error) {
	if r.existing == nil || r.existing.ID != id {
		return nil, errors.New("user not found")
	}
	return r.existing, nil
}
func (r *recordingUserRepo) GetByEmail(contextID, email string) (*context.User, error) {
	return nil, errors.New("not found")
}
func (r *recordingUserRepo) GetByEmailGlobal(email string) ([]*context.User, error) {
	return nil, nil
}
func (r *recordingUserRepo) GetByUsername(email string) (*context.User, error) {
	return nil, errors.New("not found")
}
func (r *recordingUserRepo) ExistsAnyUser() (bool, error) { return false, nil }
func (r *recordingUserRepo) ListAll() ([]*context.User, error) { return nil, nil }
func (r *recordingUserRepo) Update(u *context.User) error {
	r.existing = u
	return nil
}
func (r *recordingUserRepo) Delete(id string) error            { return nil }
func (r *recordingUserRepo) UpdateProfile(userID string, displayName, avatarURL *string) error {
	return nil
}
func (r *recordingUserRepo) ChangePassword(userID, passwordHash string) error { return nil }
func (r *recordingUserRepo) GetPasswordChangedAt(userID string) (time.Time, error) {
	return time.Time{}, nil
}
func (r *recordingUserRepo) UpdatePasswordChangedAt(userID string) error { return nil }
func (r *recordingUserRepo) GetByUserIDContext(ctx stdcontext.Context, userID, contextID string) (*context.UserContext, error) {
	return nil, errors.New("not found")
}
func (r *recordingUserRepo) ListByUser(userID string) ([]context.UserContext, error) {
	return nil, nil
}
func (r *recordingUserRepo) CreateMembership(uc *context.UserContext) error { return nil }
func (r *recordingUserRepo) UpdateMembership(uc *context.UserContext) error { return nil }
func (r *recordingUserRepo) DeleteMembership(userID, contextID string) error {
	return nil
}
func (r *recordingUserRepo) ListMembers(contextID string) ([]context.UserContext, error) {
	return nil, nil
}

func newUserHandlerEngine(repo *recordingUserRepo) *gin.Engine {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	h := NewUserHandler(repo, nil)
	engine.POST("/admin/users", h.Create)
	engine.PUT("/admin/users/:id", h.Update)
	return engine
}

// TestUserHandler_CreateIgnoresIsAdminFromBody is the regression test for
// the privilege-escalation finding: the is_admin field in the request body
// must never reach the database.
func TestUserHandler_CreateIgnoresIsAdminFromBody(t *testing.T) {
	repo := &recordingUserRepo{}
	engine := newUserHandlerEngine(repo)

	body := `{"email":"evil@example.com","password":"secret123","is_admin":true}`
	req := httptest.NewRequest(http.MethodPost, "/admin/users", strings.NewReader(body))
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d (body %s)", w.Code, w.Body.String())
	}
	if repo.created == nil {
		t.Fatal("no user was created")
	}
	if repo.created.IsAdmin {
		t.Fatalf("is_admin leaked from request body into created user")
	}
}

// TestUserHandler_UpdateIgnoresIsAdminFromBody: a non-admin field in the
// update body must not flip the stored admin flag.
func TestUserHandler_UpdateIgnoresIsAdminFromBody(t *testing.T) {
	id := uuid.New().String()
	repo := &recordingUserRepo{existing: &context.User{ID: id, Email: "a@example.com", IsAdmin: false}}
	engine := newUserHandlerEngine(repo)

	body := `{"is_admin":true}`
	req := httptest.NewRequest(http.MethodPut, "/admin/users/"+id, strings.NewReader(body))
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body %s)", w.Code, w.Body.String())
	}
	if repo.existing.IsAdmin {
		t.Fatalf("is_admin flipped to true via update body")
	}
}

