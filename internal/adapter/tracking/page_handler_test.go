package tracking

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/h2hsecure/automata/internal/domain/page"
)

type fakePageRepoForTrack struct {
	pages []*page.Page
}

func (r *fakePageRepoForTrack) Create(p *page.Page) error {
	r.pages = append(r.pages, p)
	return nil
}

func (r *fakePageRepoForTrack) GetByID(id string) (*page.Page, error) {
	for _, p := range r.pages {
		if p.ID == id {
			return p, nil
		}
	}
	return nil, errors.New("page not found")
}

func (r *fakePageRepoForTrack) GetBySlug(contextID, slug string) (*page.Page, error) {
	for _, p := range r.pages {
		if p.ContextID == contextID && p.Slug == slug {
			return p, nil
		}
	}
	return nil, errors.New("page not found")
}

func (r *fakePageRepoForTrack) List(contextID string) ([]*page.Page, error) {
	var out []*page.Page
	for _, p := range r.pages {
		if p.ContextID == contextID {
			out = append(out, p)
		}
	}
	return out, nil
}

func (r *fakePageRepoForTrack) Update(p *page.Page) error { return nil }

func (r *fakePageRepoForTrack) Delete(id string) error { return nil }

func (r *fakePageRepoForTrack) ListActive(contextID string) ([]*page.Page, error) {
	var out []*page.Page
	for _, p := range r.pages {
		if p.ContextID == contextID && p.IsActive {
			out = append(out, p)
		}
	}
	return out, nil
}

func (r *fakePageRepoForTrack) RecordImpression(contextID, pageID, visitorID string) error {
	return nil
}

func (r *fakePageRepoForTrack) RecordDecision(*page.Decision) error { return nil }

func (r *fakePageRepoForTrack) ListDecisions(pageID string, offset, limit int) ([]*page.Decision, int64, error) {
	return nil, 0, nil
}

func (r *fakePageRepoForTrack) DecisionCounts(pageID string) (map[string]int64, error) {
	return map[string]int64{}, nil
}

func TestPageServeHandler_ServesActivePage(t *testing.T) {
	repo := &fakePageRepoForTrack{pages: []*page.Page{{
		ID: "p1", ContextID: "ctx-1", Slug: "cookie-policy", Title: "Cookie Policy",
		Content: "<p>We use cookies.</p>", IsActive: true,
	}}}

	h := NewPageServeHandler(repo)
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/page/ctx-1/cookie-policy", nil)
	c.Params = gin.Params{{Key: "contextID", Value: "ctx-1"}, {Key: "slug", Value: "cookie-policy"}}

	h.Serve(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if got := w.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
		t.Fatalf("expected text/html content type, got %q", got)
	}
	body := w.Body.String()
	for _, want := range []string{"Cookie Policy", "We use cookies.", "<!DOCTYPE html>"} {
		if !strings.Contains(body, want) {
			t.Errorf("expected body to contain %q, got %q", want, body)
		}
	}
}

func TestPageServeHandler_Missing_404(t *testing.T) {
	h := NewPageServeHandler(&fakePageRepoForTrack{})
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/page/ctx-1/nope", nil)
	c.Params = gin.Params{{Key: "contextID", Value: "ctx-1"}, {Key: "slug", Value: "nope"}}

	h.Serve(c)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestPageServeHandler_Inactive_404(t *testing.T) {
	repo := &fakePageRepoForTrack{pages: []*page.Page{{
		ID: "p1", ContextID: "ctx-1", Slug: "old", Title: "Old",
		Content: "<p>hidden</p>", IsActive: false,
	}}}

	h := NewPageServeHandler(repo)
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/page/ctx-1/old", nil)
	c.Params = gin.Params{{Key: "contextID", Value: "ctx-1"}, {Key: "slug", Value: "old"}}

	h.Serve(c)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for inactive page, got %d", w.Code)
	}
}

func TestRegisterPageRoutes_RegistersGETAndHEAD(t *testing.T) {
	h := NewPageServeHandler(&fakePageRepoForTrack{})
	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterPageRoutes(r, h)

	for _, method := range []string{http.MethodGet, http.MethodHead} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(method, "/page/ctx-1/nope", nil)
		r.ServeHTTP(w, req)
		if w.Code != http.StatusNotFound {
			t.Fatalf("expected 404 (route matched) for %s, got %d", method, w.Code)
		}
	}
}
