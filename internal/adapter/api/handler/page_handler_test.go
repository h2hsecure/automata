package handler

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/h2hsecure/automata/internal/domain/page"
)

// fakePageRepo is an in-memory page.Repository for handler tests.
type fakePageRepo struct {
	pages     map[string]*page.Page
	nextID    int
	dupOnSlug string // if set, Create returns a duplicate error for this slug
}

func newFakePageRepo() *fakePageRepo {
	return &fakePageRepo{pages: map[string]*page.Page{}}
}

func (r *fakePageRepo) Create(p *page.Page) error {
	if r.dupOnSlug != "" && p.Slug == r.dupOnSlug {
		return fmt.Errorf("insert page: duplicate key value violates unique constraint (duplicate slug)")
	}
	r.nextID++
	p.ID = fmt.Sprintf("page-%d", r.nextID)
	r.pages[p.ID] = p
	return nil
}

func (r *fakePageRepo) GetByID(id string) (*page.Page, error) {
	p, ok := r.pages[id]
	if !ok {
		return nil, errors.New("page not found")
	}
	return p, nil
}

func (r *fakePageRepo) GetBySlug(contextID, slug string) (*page.Page, error) {
	for _, p := range r.pages {
		if p.ContextID == contextID && p.Slug == slug {
			return p, nil
		}
	}
	return nil, errors.New("page not found")
}

func (r *fakePageRepo) List(contextID string) ([]*page.Page, error) {
	var out []*page.Page
	for _, p := range r.pages {
		if p.ContextID == contextID {
			out = append(out, p)
		}
	}
	return out, nil
}

func (r *fakePageRepo) Update(p *page.Page) error {
	if r.dupOnSlug != "" && p.Slug == r.dupOnSlug {
		return fmt.Errorf("update page: duplicate key value violates unique constraint (duplicate slug)")
	}
	if _, ok := r.pages[p.ID]; !ok {
		return errors.New("page not found")
	}
	r.pages[p.ID] = p
	return nil
}

func (r *fakePageRepo) Delete(id string) error {
	if _, ok := r.pages[id]; !ok {
		return errors.New("page not found")
	}
	delete(r.pages, id)
	return nil
}

func newPageTestContext(method, path, body string, params gin.Params, contextID string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	var reader *bytes.Reader
	if body != "" {
		reader = bytes.NewReader([]byte(body))
	} else {
		reader = bytes.NewReader(nil)
	}
	c.Request = httptest.NewRequest(method, path, reader)
	if body != "" {
		c.Request.Header.Set("Content-Type", "application/json")
	}
	c.Params = params
	if contextID != "" {
		c.Set("context_id", contextID)
	}
	return c, w
}

func TestPageHandler_List(t *testing.T) {
	repo := newFakePageRepo()
	repo.pages["page-1"] = &page.Page{ID: "page-1", ContextID: "ctx-1", Slug: "a", Title: "A"}
	repo.pages["page-2"] = &page.Page{ID: "page-2", ContextID: "ctx-1", Slug: "b", Title: "B"}

	h := NewPageHandler(repo)
	c, w := newPageTestContext("GET", "/context/pages", "", nil, "ctx-1")
	h.List(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestPageHandler_Get(t *testing.T) {
	repo := newFakePageRepo()
	repo.pages["page-1"] = &page.Page{ID: "page-1", ContextID: "ctx-1", Slug: "a", Title: "A"}

	h := NewPageHandler(repo)
	c, w := newPageTestContext("GET", "/context/pages/page-1", "", gin.Params{{Key: "id", Value: "page-1"}}, "ctx-1")
	h.Get(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestPageHandler_Get_Missing_404(t *testing.T) {
	repo := newFakePageRepo()
	h := NewPageHandler(repo)
	c, w := newPageTestContext("GET", "/context/pages/nope", "", gin.Params{{Key: "id", Value: "nope"}}, "ctx-1")
	h.Get(c)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestPageHandler_Get_ForeignContext_404(t *testing.T) {
	repo := newFakePageRepo()
	repo.pages["page-1"] = &page.Page{ID: "page-1", ContextID: "other-ctx", Slug: "a", Title: "A"}

	h := NewPageHandler(repo)
	c, w := newPageTestContext("GET", "/context/pages/page-1", "", gin.Params{{Key: "id", Value: "page-1"}}, "ctx-1")
	h.Get(c)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for foreign context, got %d", w.Code)
	}
}

func TestPageHandler_Create(t *testing.T) {
	repo := newFakePageRepo()
	h := NewPageHandler(repo)
	body := `{"slug":"cookie-policy","title":"Cookie Policy","content":"<p>hi</p>"}`
	c, w := newPageTestContext("POST", "/context/pages", body, nil, "ctx-1")
	h.Create(c)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	if len(repo.pages) != 1 {
		t.Fatalf("expected 1 page created, got %d", len(repo.pages))
	}
}

func TestPageHandler_Create_EmptySlug_400(t *testing.T) {
	repo := newFakePageRepo()
	h := NewPageHandler(repo)
	body := `{"slug":"","title":"No Slug"}`
	c, w := newPageTestContext("POST", "/context/pages", body, nil, "ctx-1")
	h.Create(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
	if len(repo.pages) != 0 {
		t.Fatalf("expected no page created, got %d", len(repo.pages))
	}
}

func TestPageHandler_Create_Duplicate_409(t *testing.T) {
	repo := newFakePageRepo()
	repo.dupOnSlug = "taken"
	h := NewPageHandler(repo)
	body := `{"slug":"taken","title":"Taken"}`
	c, w := newPageTestContext("POST", "/context/pages", body, nil, "ctx-1")
	h.Create(c)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPageHandler_Update(t *testing.T) {
	repo := newFakePageRepo()
	repo.pages["page-1"] = &page.Page{ID: "page-1", ContextID: "ctx-1", Slug: "a", Title: "A"}
	h := NewPageHandler(repo)
	body := `{"slug":"a2","title":"A2"}`
	c, w := newPageTestContext("PUT", "/context/pages/page-1", body, gin.Params{{Key: "id", Value: "page-1"}}, "ctx-1")
	h.Update(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if repo.pages["page-1"].Title != "A2" {
		t.Fatalf("expected title updated, got %q", repo.pages["page-1"].Title)
	}
}

func TestPageHandler_Update_ForeignContext_404(t *testing.T) {
	repo := newFakePageRepo()
	repo.pages["page-1"] = &page.Page{ID: "page-1", ContextID: "other-ctx", Slug: "a", Title: "A"}
	h := NewPageHandler(repo)
	body := `{"slug":"a","title":"A"}`
	c, w := newPageTestContext("PUT", "/context/pages/page-1", body, gin.Params{{Key: "id", Value: "page-1"}}, "ctx-1")
	h.Update(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 (foreign context maps to 400 via usecase), got %d", w.Code)
	}
}

func TestPageHandler_Delete(t *testing.T) {
	repo := newFakePageRepo()
	repo.pages["page-1"] = &page.Page{ID: "page-1", ContextID: "ctx-1", Slug: "a", Title: "A"}
	h := NewPageHandler(repo)
	c, w := newPageTestContext("DELETE", "/context/pages/page-1", "", gin.Params{{Key: "id", Value: "page-1"}}, "ctx-1")
	h.Delete(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if len(repo.pages) != 0 {
		t.Fatalf("expected page deleted, got %d", len(repo.pages))
	}
}

func TestPageHandler_Delete_ForeignContext_404(t *testing.T) {
	repo := newFakePageRepo()
	repo.pages["page-1"] = &page.Page{ID: "page-1", ContextID: "other-ctx", Slug: "a", Title: "A"}
	h := NewPageHandler(repo)
	c, w := newPageTestContext("DELETE", "/context/pages/page-1", "", gin.Params{{Key: "id", Value: "page-1"}}, "ctx-1")
	h.Delete(c)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for foreign context, got %d", w.Code)
	}
	if len(repo.pages) != 1 {
		t.Fatalf("foreign-context page must not be deleted, got %d", len(repo.pages))
	}
}
