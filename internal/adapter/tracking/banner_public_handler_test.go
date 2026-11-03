package tracking

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	bdomain "github.com/h2hsecure/automata/internal/domain/banner"
	pagedomain "github.com/h2hsecure/automata/internal/domain/page"
)

// --- fake banner repo (implements bdomain.Repository) ---

type fakeBannerRepoPublic struct {
	banners     []*bdomain.Banner
	decisions   []*bdomain.Decision
	impressions int
}

func (r *fakeBannerRepoPublic) CreatePlacement(*bdomain.Placement) error { return nil }
func (r *fakeBannerRepoPublic) GetPlacement(string) (*bdomain.Placement, error) {
	return nil, nil
}
func (r *fakeBannerRepoPublic) ListPlacements(string, bool) ([]*bdomain.Placement, error) {
	return nil, nil
}
func (r *fakeBannerRepoPublic) UpdatePlacement(*bdomain.Placement) error { return nil }
func (r *fakeBannerRepoPublic) DeletePlacement(string) error             { return nil }

func (r *fakeBannerRepoPublic) CreateBanner(b *bdomain.Banner) error {
	r.banners = append(r.banners, b)
	return nil
}
func (r *fakeBannerRepoPublic) GetBanner(id string) (*bdomain.Banner, error) {
	for _, b := range r.banners {
		if b.ID == id {
			return b, nil
		}
	}
	return nil, nil
}
func (r *fakeBannerRepoPublic) ListBanners(string, bdomain.ListOptions) ([]*bdomain.Banner, int64, error) {
	return nil, 0, nil
}
func (r *fakeBannerRepoPublic) ListActiveBannersForPlacement(contextID, _ string, _ time.Time) ([]*bdomain.Banner, error) {
	var out []*bdomain.Banner
	for _, b := range r.banners {
		if b.ContextID == contextID && b.IsActive {
			out = append(out, b)
		}
	}
	return out, nil
}
func (r *fakeBannerRepoPublic) UpdateBanner(*bdomain.Banner) error { return nil }
func (r *fakeBannerRepoPublic) DeleteBanner(string) error          { return nil }
func (r *fakeBannerRepoPublic) RecordImpression(string, string, string) error {
	r.impressions++
	return nil
}
func (r *fakeBannerRepoPublic) RecordClick(string, string, string) error { return nil }
func (r *fakeBannerRepoPublic) RecordDecision(d *bdomain.Decision) error {
	if d.ID == "" {
		d.ID = "d1"
	}
	r.decisions = append(r.decisions, d)
	return nil
}
func (r *fakeBannerRepoPublic) ListDecisions(string, int, int) ([]*bdomain.Decision, int64, error) {
	return nil, 0, nil
}
func (r *fakeBannerRepoPublic) DecisionCounts(string) (map[string]int64, error) {
	return nil, nil
}
func (r *fakeBannerRepoPublic) CreateCampaign(*bdomain.Campaign) error { return nil }
func (r *fakeBannerRepoPublic) GetCampaign(string) (*bdomain.Campaign, error) {
	return nil, nil
}
func (r *fakeBannerRepoPublic) ListCampaigns(string, bool) ([]*bdomain.Campaign, error) {
	return nil, nil
}
func (r *fakeBannerRepoPublic) UpdateCampaign(*bdomain.Campaign) error { return nil }
func (r *fakeBannerRepoPublic) DeleteCampaign(string) error            { return nil }

// --- fake page repo (implements pagedomain.Repository) ---

type fakePageRepoPublic struct {
	pages []*pagedomain.Page
}

func (r *fakePageRepoPublic) Create(p *pagedomain.Page) error {
	r.pages = append(r.pages, p)
	return nil
}
func (r *fakePageRepoPublic) GetByID(id string) (*pagedomain.Page, error) {
	for _, p := range r.pages {
		if p.ID == id {
			return p, nil
		}
	}
	return nil, nil
}
func (r *fakePageRepoPublic) GetBySlug(contextID, slug string) (*pagedomain.Page, error) {
	for _, p := range r.pages {
		if p.ContextID == contextID && p.Slug == slug {
			return p, nil
		}
	}
	return nil, nil
}
func (r *fakePageRepoPublic) List(contextID string) ([]*pagedomain.Page, error) {
	return nil, nil
}
func (r *fakePageRepoPublic) Update(*pagedomain.Page) error { return nil }
func (r *fakePageRepoPublic) Delete(string) error           { return nil }

// Position/banner-merge methods (unused by banner tests; keep the fake
// implementing pagedomain.Repository until this file is deleted in Task 7).
func (r *fakePageRepoPublic) ListActive(contextID string) ([]*pagedomain.Page, error) {
	var out []*pagedomain.Page
	for _, p := range r.pages {
		if p.ContextID == contextID && p.IsActive {
			out = append(out, p)
		}
	}
	return out, nil
}
func (r *fakePageRepoPublic) RecordImpression(contextID, pageID, visitorID string) error {
	return nil
}
func (r *fakePageRepoPublic) RecordDecision(*pagedomain.Decision) error { return nil }
func (r *fakePageRepoPublic) ListDecisions(pageID string, offset, limit int) ([]*pagedomain.Decision, int64, error) {
	return nil, 0, nil
}
func (r *fakePageRepoPublic) DecisionCounts(pageID string) (map[string]int64, error) {
	return map[string]int64{}, nil
}

// --- tests ---

func TestBannerPublicHandler_Config_ReturnsBannersWithPage(t *testing.T) {
	bannerRepo := &fakeBannerRepoPublic{banners: []*bdomain.Banner{{
		ID: "b1", ContextID: "ctx-1", Name: "Cookie banner",
		Position: "fullpage", Content: "We use cookies", IsActive: true,
		Buttons: []bdomain.Button{{ID: "b1", Label: "Accept", Action: "accept", Dismisses: true},
			{ID: "b2", Label: "Cookie Policy", Action: "view-policy", LinkURL: "/cookie-policy"}},
	}}}
	pageRepo := &fakePageRepoPublic{pages: []*pagedomain.Page{{
		ID: "p1", ContextID: "ctx-1", Slug: "cookie-policy", Title: "Cookie Policy",
		Content: "<p>We use cookies.</p>", IsActive: true,
	}}}

	h := NewBannerPublicHandler(bannerRepo, pageRepo)
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/banner/config/ctx-1", nil)
	c.Params = gin.Params{{Key: "contextID", Value: "ctx-1"}}

	h.Config(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Banners []struct {
			ID       string           `json:"id"`
			Position string           `json:"position"`
			Content  string           `json:"content"`
			Buttons  []bdomain.Button `json:"buttons"`
			Page     *string          `json:"page"`
		} `json:"banners"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Banners) != 1 {
		t.Fatalf("expected 1 banner, got %d", len(resp.Banners))
	}
	if resp.Banners[0].Position != "fullpage" {
		t.Errorf("expected fullpage, got %q", resp.Banners[0].Position)
	}
	if resp.Banners[0].Page == nil || *resp.Banners[0].Page != "<p>We use cookies.</p>" {
		t.Errorf("expected inlined page content, got %v", resp.Banners[0].Page)
	}
}

func TestBannerPublicHandler_Config_PageNilWhenNoPage(t *testing.T) {
	bannerRepo := &fakeBannerRepoPublic{banners: []*bdomain.Banner{{
		ID: "b1", ContextID: "ctx-1", Name: "B", Position: "top",
		Content: "c", IsActive: true,
		Buttons: []bdomain.Button{{ID: "b1", Label: "OK", Action: "ok", Dismisses: true}},
	}}}
	h := NewBannerPublicHandler(bannerRepo, &fakePageRepoPublic{})
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/banner/config/ctx-1", nil)
	c.Params = gin.Params{{Key: "contextID", Value: "ctx-1"}}

	h.Config(c)

	var resp struct {
		Banners []struct {
			Page *string `json:"page"`
		} `json:"banners"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Banners[0].Page != nil {
		t.Errorf("expected page nil, got %v", resp.Banners[0].Page)
	}
}

func TestBannerPublicHandler_Config_RecordsImpression(t *testing.T) {
	bannerRepo := &fakeBannerRepoPublic{banners: []*bdomain.Banner{{
		ID: "b1", ContextID: "ctx-1", Name: "B", Content: "c", IsActive: true,
	}}}
	h := NewBannerPublicHandler(bannerRepo, &fakePageRepoPublic{})
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/banner/config/ctx-1?visitor_id=v1", nil)
	c.Params = gin.Params{{Key: "contextID", Value: "ctx-1"}}
	c.Request.URL.RawQuery = "visitor_id=v1"

	h.Config(c)

	if bannerRepo.impressions != 1 {
		t.Errorf("expected 1 impression, got %d", bannerRepo.impressions)
	}
}

func TestBannerPublicHandler_Decision_Valid(t *testing.T) {
	bannerRepo := &fakeBannerRepoPublic{banners: []*bdomain.Banner{{
		ID: "b1", ContextID: "ctx-1", Name: "B", IsActive: true,
	}}}
	h := NewBannerPublicHandler(bannerRepo, &fakePageRepoPublic{})
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	body := `{"banner_id":"b1","action":"accept","visitor_id":"v1"}`
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/banner/decision/ctx-1", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "text/plain")
	c.Params = gin.Params{{Key: "contextID", Value: "ctx-1"}}
	c.Request.RemoteAddr = "1.2.3.4:1234"

	h.Decision(c)
	c.Writer.WriteHeaderNow()

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
	}
	if len(bannerRepo.decisions) != 1 {
		t.Fatalf("expected 1 decision, got %d", len(bannerRepo.decisions))
	}
	d := bannerRepo.decisions[0]
	if d.Action != "accept" || d.BannerID != "b1" || d.VisitorID != "v1" {
		t.Errorf("unexpected decision: %+v", d)
	}
	if d.IPHash == "" {
		t.Error("expected non-empty IP hash")
	}
}

func TestBannerPublicHandler_Decision_OtherContext_404(t *testing.T) {
	bannerRepo := &fakeBannerRepoPublic{banners: []*bdomain.Banner{{
		ID: "b1", ContextID: "ctx-1", Name: "B", IsActive: true,
	}}}
	h := NewBannerPublicHandler(bannerRepo, &fakePageRepoPublic{})
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	body := `{"banner_id":"b1","action":"accept"}`
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/banner/decision/ctx-2", strings.NewReader(body))
	c.Params = gin.Params{{Key: "contextID", Value: "ctx-2"}}

	h.Decision(c)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestBannerPublicHandler_Decision_MissingAction_400(t *testing.T) {
	bannerRepo := &fakeBannerRepoPublic{banners: []*bdomain.Banner{{
		ID: "b1", ContextID: "ctx-1", Name: "B", IsActive: true,
	}}}
	h := NewBannerPublicHandler(bannerRepo, &fakePageRepoPublic{})
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	body := `{"banner_id":"b1"}`
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/banner/decision/ctx-1", strings.NewReader(body))
	c.Params = gin.Params{{Key: "contextID", Value: "ctx-1"}}

	h.Decision(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestRegisterBannerPublicRoutes_RegistersRoutes(t *testing.T) {
	h := NewBannerPublicHandler(&fakeBannerRepoPublic{}, &fakePageRepoPublic{})
	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterBannerPublicRoutes(r, h)

	for _, method := range []string{http.MethodGet, http.MethodHead} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(method, "/banner/config/ctx-1", nil)
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 for %s config, got %d", method, w.Code)
		}
	}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/banner/decision/ctx-1", strings.NewReader(`{}`))
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for decision, got %d", w.Code)
	}
}

func TestBannerPublicHandler_ServeSnippet(t *testing.T) {
	h := NewBannerPublicHandler(&fakeBannerRepoPublic{}, &fakePageRepoPublic{})
	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterBannerPublicRoutes(r, h)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/banner/snippet/ctx-1.js", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/javascript; charset=utf-8" {
		t.Errorf("expected application/javascript content type, got %q", ct)
	}
	if !strings.Contains(w.Body.String(), `/banner/config/ctx-1`) {
		t.Errorf("snippet missing config URL for ctx-1")
	}
}

func TestBannerPublicHandler_ServeSnippet_EmptyContext_400(t *testing.T) {
	h := NewBannerPublicHandler(&fakeBannerRepoPublic{}, &fakePageRepoPublic{})
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/banner/snippet/.js", nil)
	c.Params = gin.Params{{Key: "contextID", Value: ""}}

	h.ServeSnippet(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}
