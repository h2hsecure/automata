package handler

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/h2hsecure/automata/internal/domain/contact"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// fakeContactRepo is an in-memory contact.Repository for handler tests.
// It records the filters passed to List so tests can assert on query parsing.
type fakeContactRepo struct {
	lastListFilters contact.FilterOptions
	contacts        []*contact.Contact
	listErr         error
}

func (r *fakeContactRepo) Create(c *contact.Contact) error { return nil }

func (r *fakeContactRepo) GetByID(id string) (*contact.Contact, error) {
	return nil, nil
}

func (r *fakeContactRepo) List(offset, limit int, filters contact.FilterOptions) ([]*contact.Contact, error) {
	r.lastListFilters = filters
	if r.listErr != nil {
		return nil, r.listErr
	}
	return r.contacts, nil
}

func (r *fakeContactRepo) Update(c *contact.Contact) error { return nil }

func (r *fakeContactRepo) Delete(id string) error { return nil }

func (r *fakeContactRepo) FindByEmail(contextID, email string) (*contact.Contact, error) {
	return nil, nil
}

func (r *fakeContactRepo) Merge(keepID, mergeIntoID string) error { return nil }

func (r *fakeContactRepo) CreateActivity(a *contact.Activity) error { return nil }

func (r *fakeContactRepo) GetActivity(contactID, contextID string, offset, limit int, activityType *string) ([]contact.Activity, int64, error) {
	return nil, 0, nil
}

func (r *fakeContactRepo) GetActivityCount(contactID string) (int64, error) {
	return 0, nil
}

func (r *fakeContactRepo) CountByContext(contextID string) (int64, error) {
	return 0, nil
}

func (r *fakeContactRepo) CountDistinctContactsByActivity(contextID, activityType string, since *time.Time) (int, error) {
	return 0, nil
}

func (r *fakeContactRepo) FindBySourceID(contextID, source, sourceID string) (*contact.Contact, error) {
	return nil, nil
}

func (r *fakeContactRepo) Count(contextID string, filters contact.FilterOptions) (int64, error) {
	return int64(len(r.contacts)), nil
}

func newContactListRouter(repo *fakeContactRepo) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/context/contacts", func(c *gin.Context) {
		c.Set("context_id", "ctx-1")
		(&ContactHandler{repo: repo}).List(c)
	})
	return r
}

func TestContactList_SinceParamParsesRFC3339(t *testing.T) {
	repo := &fakeContactRepo{}
	router := newContactListRouter(repo)

	req := httptest.NewRequest(http.MethodGet, "/context/contacts?since=2026-07-30T00:00:00Z", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", w.Code, w.Body.String())
	}
	if repo.lastListFilters.Since == nil {
		t.Fatal("Filters.Since = nil, want it set from the since query param")
	}
	want := time.Date(2026, 7, 30, 0, 0, 0, 0, time.UTC)
	if !repo.lastListFilters.Since.Equal(want) {
		t.Errorf("Filters.Since = %v, want %v", repo.lastListFilters.Since, want)
	}
}

func TestContactList_InvalidSinceReturns400(t *testing.T) {
	repo := &fakeContactRepo{}
	router := newContactListRouter(repo)

	req := httptest.NewRequest(http.MethodGet, "/context/contacts?since=not-a-date", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body: %s)", w.Code, w.Body.String())
	}
}

// TestContactList_RepoErrorReturnsGenericMessage asserts that a repository
// failure surfaces a generic client message (no internal detail) while the
// real error is written to the server log.
func TestContactList_RepoErrorReturnsGenericMessage(t *testing.T) {
	repo := &fakeContactRepo{listErr: errors.New("boom: relation \"contacts\" does not exist (SQLSTATE 42P01)")}
	router := newContactListRouter(repo)

	var buf bytes.Buffer
	orig := log.Logger
	log.Logger = zerolog.New(&buf)
	defer func() { log.Logger = orig }()

	req := httptest.NewRequest(http.MethodGet, "/context/contacts", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500 (body: %s)", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "failed to list contacts") {
		t.Errorf("body = %s, want generic message", w.Body.String())
	}
	if strings.Contains(w.Body.String(), "boom") || strings.Contains(w.Body.String(), "42P01") {
		t.Errorf("raw repository error leaked to client: %s", w.Body.String())
	}
	if !strings.Contains(buf.String(), "boom") {
		t.Errorf("real error not present in server log: %s", buf.String())
	}
}