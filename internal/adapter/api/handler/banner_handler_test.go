package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	bdomain "github.com/h2hsecure/automata/internal/domain/banner"
)

type fakeBannerRepoAPI struct {
	banners   []*bdomain.Banner
	decisions []*bdomain.Decision
	counts    map[string]int64
}

func (r *fakeBannerRepoAPI) CreatePlacement(*bdomain.Placement) error { return nil }
func (r *fakeBannerRepoAPI) GetPlacement(string) (*bdomain.Placement, error) {
	return nil, nil
}
func (r *fakeBannerRepoAPI) ListPlacements(string, bool) ([]*bdomain.Placement, error) {
	return nil, nil
}
func (r *fakeBannerRepoAPI) UpdatePlacement(*bdomain.Placement) error { return nil }
func (r *fakeBannerRepoAPI) DeletePlacement(string) error             { return nil }
func (r *fakeBannerRepoAPI) CreateBanner(*bdomain.Banner) error       { return nil }
func (r *fakeBannerRepoAPI) GetBanner(id string) (*bdomain.Banner, error) {
	for _, b := range r.banners {
		if b.ID == id {
			return b, nil
		}
	}
	return nil, nil
}
func (r *fakeBannerRepoAPI) ListBanners(string, bdomain.ListOptions) ([]*bdomain.Banner, int64, error) {
	return nil, 0, nil
}
func (r *fakeBannerRepoAPI) ListActiveBannersForPlacement(string, string, time.Time) ([]*bdomain.Banner, error) {
	return nil, nil
}
func (r *fakeBannerRepoAPI) UpdateBanner(*bdomain.Banner) error { return nil }
func (r *fakeBannerRepoAPI) DeleteBanner(string) error          { return nil }
func (r *fakeBannerRepoAPI) RecordImpression(string, string) error {
	return nil
}
func (r *fakeBannerRepoAPI) RecordClick(string, string) error { return nil }
func (r *fakeBannerRepoAPI) RecordDecision(*bdomain.Decision) error {
	return nil
}
func (r *fakeBannerRepoAPI) ListDecisions(string, int, int) ([]*bdomain.Decision, int64, error) {
	return r.decisions, int64(len(r.decisions)), nil
}
func (r *fakeBannerRepoAPI) DecisionCounts(string) (map[string]int64, error) {
	if r.counts == nil {
		return map[string]int64{}, nil
	}
	return r.counts, nil
}
func (r *fakeBannerRepoAPI) CreateCampaign(*bdomain.Campaign) error { return nil }
func (r *fakeBannerRepoAPI) GetCampaign(string) (*bdomain.Campaign, error) {
	return nil, nil
}
func (r *fakeBannerRepoAPI) ListCampaigns(string, bool) ([]*bdomain.Campaign, error) {
	return nil, nil
}
func (r *fakeBannerRepoAPI) UpdateCampaign(*bdomain.Campaign) error { return nil }
func (r *fakeBannerRepoAPI) DeleteCampaign(string) error            { return nil }

func newBannerHandlerTestContext(repo bdomain.Repository, contextID, bannerID string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/context/banners/"+bannerID+"/decisions", nil)
	c.Params = gin.Params{{Key: "id", Value: bannerID}}
	c.Set("context_id", contextID)
	return c, w
}

func TestBannerHandler_ListDecisions_ReturnsDecisionsAndCounts(t *testing.T) {
	repo := &fakeBannerRepoAPI{
		banners:   []*bdomain.Banner{{ID: "b1", ContextID: "ctx-1"}},
		decisions: []*bdomain.Decision{{ID: "d1", ContextID: "ctx-1", BannerID: "b1", VisitorID: "v1", Action: "accept"}},
		counts:    map[string]int64{"accept": 1},
	}
	h := NewBannerHandler(repo)
	c, w := newBannerHandlerTestContext(repo, "ctx-1", "b1")

	h.ListDecisions(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp struct {
		Decisions []*bdomain.Decision `json:"decisions"`
		Counts    map[string]int64    `json:"counts"`
		Total     int64               `json:"total"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Decisions) != 1 || resp.Decisions[0].Action != "accept" {
		t.Errorf("unexpected decisions: %+v", resp.Decisions)
	}
	if resp.Counts["accept"] != 1 {
		t.Errorf("expected accept count 1, got %d", resp.Counts["accept"])
	}
	if resp.Total != 1 {
		t.Errorf("expected total 1, got %d", resp.Total)
	}
}

func TestBannerHandler_ListDecisions_OtherContext_404(t *testing.T) {
	repo := &fakeBannerRepoAPI{banners: []*bdomain.Banner{{ID: "b1", ContextID: "ctx-1"}}}
	h := NewBannerHandler(repo)
	c, w := newBannerHandlerTestContext(repo, "ctx-2", "b1")

	h.ListDecisions(c)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestBannerHandler_ListDecisions_MissingContext_400(t *testing.T) {
	repo := &fakeBannerRepoAPI{banners: []*bdomain.Banner{{ID: "b1", ContextID: "ctx-1"}}}
	h := NewBannerHandler(repo)
	c, w := newBannerHandlerTestContext(repo, "", "b1")

	h.ListDecisions(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}
