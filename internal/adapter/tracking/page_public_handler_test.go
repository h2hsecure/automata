package tracking

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	pagedomain "github.com/h2hsecure/automata/internal/domain/page"
)

// --- fake page repo (implements pagedomain.Repository) ---

type fakePagePublicRepo struct {
	pages       map[string]*pagedomain.Page
	decisions   []*pagedomain.Decision
	impressions int
}

func newFakePagePublicRepo() *fakePagePublicRepo {
	return &fakePagePublicRepo{pages: map[string]*pagedomain.Page{}}
}

func (r *fakePagePublicRepo) Create(p *pagedomain.Page) error {
	r.pages[p.ID] = p
	return nil
}
func (r *fakePagePublicRepo) GetByID(id string) (*pagedomain.Page, error) {
	if p, ok := r.pages[id]; ok {
		return p, nil
	}
	return nil, nil
}
func (r *fakePagePublicRepo) GetBySlug(contextID, slug string) (*pagedomain.Page, error) {
	for _, p := range r.pages {
		if p.ContextID == contextID && p.Slug == slug {
			return p, nil
		}
	}
	return nil, nil
}
func (r *fakePagePublicRepo) List(contextID string) ([]*pagedomain.Page, error) {
	var out []*pagedomain.Page
	for _, p := range r.pages {
		if p.ContextID == contextID {
			out = append(out, p)
		}
	}
	return out, nil
}
func (r *fakePagePublicRepo) Update(p *pagedomain.Page) error {
	r.pages[p.ID] = p
	return nil
}
func (r *fakePagePublicRepo) Delete(id string) error {
	delete(r.pages, id)
	return nil
}
func (r *fakePagePublicRepo) ListActive(contextID string) ([]*pagedomain.Page, error) {
	var out []*pagedomain.Page
	for _, p := range r.pages {
		if p.ContextID == contextID && p.IsActive {
			out = append(out, p)
		}
	}
	return out, nil
}
func (r *fakePagePublicRepo) RecordImpression(contextID, pageID, visitorID string) error {
	r.impressions++
	return nil
}
func (r *fakePagePublicRepo) RecordDecision(d *pagedomain.Decision) error {
	r.decisions = append(r.decisions, d)
	return nil
}
func (r *fakePagePublicRepo) ListDecisions(pageID string, offset, limit int) ([]*pagedomain.Decision, int64, error) {
	var out []*pagedomain.Decision
	for _, d := range r.decisions {
		if d.PageID == pageID {
			out = append(out, d)
		}
	}
	return out, int64(len(out)), nil
}
func (r *fakePagePublicRepo) DecisionCounts(pageID string) (map[string]int64, error) {
	counts := map[string]int64{}
	for _, d := range r.decisions {
		if d.PageID == pageID {
			counts[d.Action]++
		}
	}
	return counts, nil
}

// --- helpers ---

func pagePublicTestContext(method, path, body string, query map[string]string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	url := path
	if len(query) > 0 {
		parts := make([]string, 0, len(query))
		for k, v := range query {
			parts = append(parts, k+"="+v)
		}
		url += "?" + strings.Join(parts, "&")
	}
	req := httptest.NewRequest(method, url, strings.NewReader(body))
	c.Request = req
	return c, w
}

// --- tests ---

func TestPagePublicHandler_Config(t *testing.T) {
	repo := newFakePagePublicRepo()
	repo.pages["banner-1"] = &pagedomain.Page{
		ID: "banner-1", ContextID: "ctx-1", Slug: "cookie", Title: "Cookie",
		Content: "<p>cookies</p>", IsActive: true, Position: "bottom",
		Buttons: []pagedomain.Button{{Label: "Accept", Action: "dismiss_accept"}},
	}
	repo.pages["inactive-1"] = &pagedomain.Page{
		ID: "inactive-1", ContextID: "ctx-1", Slug: "off", Title: "Off",
		Content: "x", IsActive: false, Position: "top",
	}
	h := NewPagePublicHandler(repo)
	c, w := pagePublicTestContext("GET", "/page/config/ctx-1", "", map[string]string{"visitor_id": "v1"})
	c.Params = gin.Params{{Key: "contextID", Value: "ctx-1"}}
	h.Config(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Pages []map[string]interface{} `json:"pages"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Pages) != 1 {
		t.Fatalf("expected 1 active page, got %d", len(resp.Pages))
	}
	if resp.Pages[0]["id"] != "banner-1" || resp.Pages[0]["position"] != "bottom" {
		t.Fatalf("unexpected page payload: %v", resp.Pages[0])
	}
	if repo.impressions != 1 {
		t.Fatalf("expected 1 impression recorded, got %d", repo.impressions)
	}
}

func TestPagePublicHandler_Config_InlinesPageContent(t *testing.T) {
	repo := newFakePagePublicRepo()
	repo.pages["banner-1"] = &pagedomain.Page{
		ID: "banner-1", ContextID: "ctx-1", Slug: "cookie", Title: "Cookie",
		Content: "<p>cookies</p>", IsActive: true, Position: "bottom",
		Buttons: []pagedomain.Button{{Label: "Decline", Href: "/privacy", Action: "dismiss_decline"}},
	}
	repo.pages["privacy-1"] = &pagedomain.Page{
		ID: "privacy-1", ContextID: "ctx-1", Slug: "privacy", Title: "Privacy",
		Content: "<p>privacy policy</p>", IsActive: true,
	}
	h := NewPagePublicHandler(repo)
	c, w := pagePublicTestContext("GET", "/page/config/ctx-1", "", nil)
	c.Params = gin.Params{{Key: "contextID", Value: "ctx-1"}}
	h.Config(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "privacy policy") {
		t.Errorf("expected inlined privacy page content in response")
	}
}

func TestPagePublicHandler_Decision(t *testing.T) {
	repo := newFakePagePublicRepo()
	repo.pages["banner-1"] = &pagedomain.Page{
		ID: "banner-1", ContextID: "ctx-1", Slug: "cookie", Title: "Cookie",
		IsActive: true, Position: "bottom",
	}
	h := NewPagePublicHandler(repo)
	c, w := pagePublicTestContext("POST", "/page/decision/ctx-1",
		`{"page_id":"banner-1","decision":"dismiss_accept","visitor_id":"v1"}`, nil)
	c.Params = gin.Params{{Key: "contextID", Value: "ctx-1"}}
	h.Decision(c)
	c.Writer.WriteHeaderNow()

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s (decisions=%d)", w.Code, w.Body.String(), len(repo.decisions))
	}
	if len(repo.decisions) != 1 || repo.decisions[0].Action != "dismiss_accept" {
		t.Fatalf("decision not recorded correctly: %+v", repo.decisions)
	}
}

func TestPagePublicHandler_Decision_MissingFields_400(t *testing.T) {
	repo := newFakePagePublicRepo()
	h := NewPagePublicHandler(repo)
	c, w := pagePublicTestContext("POST", "/page/decision/ctx-1",
		`{"decision":"dismiss_accept"}`, nil)
	c.Params = gin.Params{{Key: "contextID", Value: "ctx-1"}}
	h.Decision(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestPagePublicHandler_Decision_ForeignContext_404(t *testing.T) {
	repo := newFakePagePublicRepo()
	repo.pages["banner-1"] = &pagedomain.Page{
		ID: "banner-1", ContextID: "other-ctx", Slug: "cookie", Title: "Cookie",
		IsActive: true, Position: "bottom",
	}
	h := NewPagePublicHandler(repo)
	c, w := pagePublicTestContext("POST", "/page/decision/ctx-1",
		`{"page_id":"banner-1","decision":"dismiss_accept"}`, nil)
	c.Params = gin.Params{{Key: "contextID", Value: "ctx-1"}}
	h.Decision(c)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for foreign context, got %d", w.Code)
	}
}

func TestPagePublicHandler_ServeSnippet(t *testing.T) {
	h := NewPagePublicHandler(newFakePagePublicRepo())
	r := gin.New()
	RegisterPagePublicRoutes(r, h)

	req := httptest.NewRequest("GET", "/page/snippet/ctx-42.js", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/javascript") {
		t.Fatalf("expected javascript content type, got %q", ct)
	}
	body := w.Body.String()
	if !strings.Contains(body, `/page/config/ctx-42`) || strings.Contains(body, "/banner/") {
		t.Errorf("snippet missing page endpoints or referencing banner endpoints")
	}
}
