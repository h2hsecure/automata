package banner

import (
	"testing"
	"time"

	bdomain "github.com/h2hsecure/automata/internal/domain/banner"
)

// fakeRepo is an in-memory implementation of banner.Repository used for
// usecase tests.
type fakeRepo struct {
	banners map[string]*bdomain.Banner
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{banners: make(map[string]*bdomain.Banner)}
}

func (f *fakeRepo) CreatePlacement(*bdomain.Placement) error { return nil }
func (f *fakeRepo) GetPlacement(string) (*bdomain.Placement, error) {
	return nil, nil
}
func (f *fakeRepo) ListPlacements(string, bool) ([]*bdomain.Placement, error) {
	return nil, nil
}
func (f *fakeRepo) UpdatePlacement(*bdomain.Placement) error { return nil }
func (f *fakeRepo) DeletePlacement(string) error             { return nil }

func (f *fakeRepo) CreateBanner(b *bdomain.Banner) error {
	b.ID = "banner-1"
	f.banners[b.ID] = b
	return nil
}

func (f *fakeRepo) GetBanner(id string) (*bdomain.Banner, error) {
	b, ok := f.banners[id]
	if !ok {
		return nil, nil
	}
	return b, nil
}

func (f *fakeRepo) ListBanners(string, bdomain.ListOptions) ([]*bdomain.Banner, int64, error) {
	return nil, 0, nil
}

func (f *fakeRepo) ListActiveBannersForPlacement(string, string, time.Time) ([]*bdomain.Banner, error) {
	return nil, nil
}

func (f *fakeRepo) UpdateBanner(b *bdomain.Banner) error {
	f.banners[b.ID] = b
	return nil
}

func (f *fakeRepo) DeleteBanner(string) error { return nil }
func (f *fakeRepo) RecordImpression(string, string) error {
	return nil
}
func (f *fakeRepo) RecordClick(string, string) error { return nil }

func (f *fakeRepo) CreateCampaign(*bdomain.Campaign) error { return nil }
func (f *fakeRepo) GetCampaign(string) (*bdomain.Campaign, error) {
	return nil, nil
}
func (f *fakeRepo) ListCampaigns(string, bool) ([]*bdomain.Campaign, error) {
	return nil, nil
}
func (f *fakeRepo) UpdateCampaign(*bdomain.Campaign) error { return nil }
func (f *fakeRepo) DeleteCampaign(string) error            { return nil }

func TestCreateBannerStoresPositionAndButtons(t *testing.T) {
	repo := newFakeRepo()
	u := NewBannerUsecase(repo)

	created, err := u.CreateBanner("ctx-1", &bdomain.CreateBannerRequest{
		Name:     "Cookie banner",
		Type:     bdomain.BannerTypeHTML,
		Content:  "We use cookies",
		Position: "fullpage",
		Buttons: []bdomain.Button{
			{ID: "b1", Label: "Accept", Action: "accept", Dismisses: true},
			{ID: "b2", Label: "Cookie Policy", Action: "view-policy", LinkURL: "/cookie-policy"},
		},
	})
	if err != nil {
		t.Fatalf("CreateBanner: %v", err)
	}
	if created.Position != "fullpage" {
		t.Errorf("Position = %q, want fullpage", created.Position)
	}
	if len(created.Buttons) != 2 {
		t.Fatalf("len(Buttons) = %d, want 2", len(created.Buttons))
	}
	stored := repo.banners[created.ID]
	if stored.Position != "fullpage" {
		t.Errorf("stored Position = %q, want fullpage", stored.Position)
	}
	if len(stored.Buttons) != 2 || !stored.Buttons[0].Dismisses {
		t.Errorf("stored Buttons = %+v", stored.Buttons)
	}
}

func TestCreateBannerNilButtonsBecomesEmptySlice(t *testing.T) {
	repo := newFakeRepo()
	u := NewBannerUsecase(repo)

	created, err := u.CreateBanner("ctx-1", &bdomain.CreateBannerRequest{
		Name: "Legacy banner",
		Type: bdomain.BannerTypeHTML,
	})
	if err != nil {
		t.Fatalf("CreateBanner: %v", err)
	}
	if created.Buttons == nil {
		t.Error("Buttons is nil, want non-nil empty slice")
	}
	if len(created.Buttons) != 0 {
		t.Errorf("len(Buttons) = %d, want 0", len(created.Buttons))
	}
}

func TestUpdateBannerAppliesPositionAndButtons(t *testing.T) {
	repo := newFakeRepo()
	u := NewBannerUsecase(repo)

	created, err := u.CreateBanner("ctx-1", &bdomain.CreateBannerRequest{
		Name: "Legacy banner",
		Type: bdomain.BannerTypeHTML,
	})
	if err != nil {
		t.Fatalf("CreateBanner: %v", err)
	}

	updated, err := u.UpdateBanner("ctx-1", created.ID, &bdomain.UpdateBannerRequest{
		Position: "top",
		Buttons:  []bdomain.Button{{ID: "b1", Label: "OK", Action: "accept", Dismisses: true}},
	})
	if err != nil {
		t.Fatalf("UpdateBanner: %v", err)
	}
	if updated.Position != "top" {
		t.Errorf("Position = %q, want top", updated.Position)
	}
	if len(updated.Buttons) != 1 || updated.Buttons[0].ID != "b1" {
		t.Errorf("Buttons = %+v", updated.Buttons)
	}
}

func TestUpdateBannerWithoutPositionKeepsExisting(t *testing.T) {
	repo := newFakeRepo()
	u := NewBannerUsecase(repo)

	created, err := u.CreateBanner("ctx-1", &bdomain.CreateBannerRequest{
		Name:     "Cookie banner",
		Type:     bdomain.BannerTypeHTML,
		Position: "bottom",
		Buttons:  []bdomain.Button{{ID: "b1", Label: "OK", Action: "accept", Dismisses: true}},
	})
	if err != nil {
		t.Fatalf("CreateBanner: %v", err)
	}

	updated, err := u.UpdateBanner("ctx-1", created.ID, &bdomain.UpdateBannerRequest{
		Name: "Renamed",
	})
	if err != nil {
		t.Fatalf("UpdateBanner: %v", err)
	}
	if updated.Position != "bottom" {
		t.Errorf("Position = %q, want bottom (preserved)", updated.Position)
	}
	if len(updated.Buttons) != 1 {
		t.Errorf("len(Buttons) = %d, want 1 (preserved)", len(updated.Buttons))
	}
}