package page

import (
	"errors"
	"testing"

	"github.com/h2hsecure/automata/internal/domain/page"
)

// fakeRepo is an in-memory page.Repository for usecase tests.
type fakeRepo struct {
	pages       map[string]*page.Page
	createErr   error
	updateErr   error
	deleteErr   error
	createCount int
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{pages: map[string]*page.Page{}}
}

func (r *fakeRepo) Create(p *page.Page) error {
	r.createCount++
	if r.createErr != nil {
		return r.createErr
	}
	r.pages[p.ID] = p
	return nil
}

func (r *fakeRepo) GetByID(id string) (*page.Page, error) {
	p, ok := r.pages[id]
	if !ok {
		return nil, errors.New("page not found")
	}
	return p, nil
}

func (r *fakeRepo) GetBySlug(contextID, slug string) (*page.Page, error) {
	for _, p := range r.pages {
		if p.ContextID == contextID && p.Slug == slug {
			return p, nil
		}
	}
	return nil, errors.New("page not found")
}

func (r *fakeRepo) List(contextID string) ([]*page.Page, error) {
	var out []*page.Page
	for _, p := range r.pages {
		if p.ContextID == contextID {
			out = append(out, p)
		}
	}
	return out, nil
}

func (r *fakeRepo) Update(p *page.Page) error {
	if r.updateErr != nil {
		return r.updateErr
	}
	if _, ok := r.pages[p.ID]; !ok {
		return errors.New("page not found")
	}
	r.pages[p.ID] = p
	return nil
}

func (r *fakeRepo) Delete(id string) error {
	if r.deleteErr != nil {
		return r.deleteErr
	}
	if _, ok := r.pages[id]; !ok {
		return errors.New("page not found")
	}
	delete(r.pages, id)
	return nil
}

func (r *fakeRepo) ListActive(contextID string) ([]*page.Page, error) {
	var out []*page.Page
	for _, p := range r.pages {
		if p.ContextID == contextID && p.IsActive {
			out = append(out, p)
		}
	}
	return out, nil
}

func (r *fakeRepo) RecordImpression(contextID, pageID, visitorID string) error {
	return nil
}

func (r *fakeRepo) RecordDecision(d *page.Decision) error {
	return nil
}

func (r *fakeRepo) ListDecisions(pageID string, offset, limit int) ([]*page.Decision, int64, error) {
	return nil, 0, nil
}

func (r *fakeRepo) DecisionCounts(pageID string) (map[string]int64, error) {
	return map[string]int64{}, nil
}

func TestCreatePage_PositionAndButtons_Stored(t *testing.T) {
	repo := newFakeRepo()

	p, err := CreatePage(repo, "ctx-1", CreatePageInput{
		Slug:     "banner",
		Title:    "Cookie Banner",
		Position: page.PositionBottom,
		Buttons: []page.Button{
			{Label: "Accept", Href: "#", Action: "dismiss_accept", Variant: "primary"},
		},
	})
	if err != nil {
		t.Fatalf("CreatePage: %v", err)
	}
	if p.Position != page.PositionBottom {
		t.Errorf("expected position bottom, got %q", p.Position)
	}
	if len(p.Buttons) != 1 || p.Buttons[0].Label != "Accept" || p.Buttons[0].Action != "dismiss_accept" {
		t.Errorf("unexpected buttons: %+v", p.Buttons)
	}
}

func TestCreatePage_InvalidPosition_Fails(t *testing.T) {
	repo := newFakeRepo()

	_, err := CreatePage(repo, "ctx-1", CreatePageInput{Slug: "x", Title: "y", Position: "fullpage"})
	if err == nil {
		t.Fatal("expected error for invalid position")
	}
	if repo.createCount != 0 {
		t.Fatalf("expected repo.Create not called, got %d", repo.createCount)
	}
}

func TestUpdatePage_PositionAndButtons(t *testing.T) {
	repo := newFakeRepo()
	created, _ := CreatePage(repo, "ctx-1", CreatePageInput{Slug: "plain", Title: "Plain"})

	updated, err := UpdatePage(repo, "ctx-1", created.ID, CreatePageInput{
		Slug:     "plain",
		Title:    "Plain",
		Position: page.PositionFull,
		Buttons: []page.Button{
			{Label: "OK", Href: "/privacy", Action: "link"},
		},
	})
	if err != nil {
		t.Fatalf("UpdatePage: %v", err)
	}
	if updated.Position != page.PositionFull {
		t.Errorf("expected position full, got %q", updated.Position)
	}
	if len(updated.Buttons) != 1 || updated.Buttons[0].Action != "link" {
		t.Errorf("unexpected buttons: %+v", updated.Buttons)
	}
}

func TestCreatePage_ValidatesAndStores(t *testing.T) {
	repo := newFakeRepo()

	p, err := CreatePage(repo, "ctx-1", CreatePageInput{Slug: "cookie-policy", Title: "Cookie Policy", Content: "<p>hi</p>"})
	if err != nil {
		t.Fatalf("CreatePage: %v", err)
	}
	if p.ID == "" {
		t.Fatal("expected generated ID")
	}
	if !p.IsActive {
		t.Error("expected IsActive to default to true")
	}
	if repo.createCount != 1 {
		t.Fatalf("expected repo.Create called once, got %d", repo.createCount)
	}
	if _, ok := repo.pages[p.ID]; !ok {
		t.Fatal("page not stored in repo")
	}
}

func TestCreatePage_EmptySlug_Fails(t *testing.T) {
	repo := newFakeRepo()

	_, err := CreatePage(repo, "ctx-1", CreatePageInput{Slug: "", Title: "No Slug"})
	if err == nil {
		t.Fatal("expected error for empty slug")
	}
	if repo.createCount != 0 {
		t.Fatalf("expected repo.Create not called, got %d", repo.createCount)
	}
}

func TestCreatePage_EmptyTitle_Fails(t *testing.T) {
	repo := newFakeRepo()

	_, err := CreatePage(repo, "ctx-1", CreatePageInput{Slug: "cookie", Title: ""})
	if err == nil {
		t.Fatal("expected error for empty title")
	}
}

func TestCreatePage_EmptyContext_Fails(t *testing.T) {
	repo := newFakeRepo()

	_, err := CreatePage(repo, "", CreatePageInput{Slug: "x", Title: "y"})
	if err == nil {
		t.Fatal("expected error for empty contextID")
	}
}

func TestUpdatePage_ForeignContext_Fails(t *testing.T) {
	repo := newFakeRepo()
	created, _ := CreatePage(repo, "ctx-1", CreatePageInput{Slug: "cookie", Title: "Cookie"})

	_, err := UpdatePage(repo, "ctx-2", created.ID, CreatePageInput{Slug: "cookie", Title: "New"})
	if err == nil {
		t.Fatal("expected error for foreign context")
	}
}

func TestUpdatePage_OwnContext_Updates(t *testing.T) {
	repo := newFakeRepo()
	created, _ := CreatePage(repo, "ctx-1", CreatePageInput{Slug: "cookie", Title: "Cookie"})
	inactive := false

	updated, err := UpdatePage(repo, "ctx-1", created.ID, CreatePageInput{Slug: "cookie-v2", Title: "Cookie 2", IsActive: &inactive})
	if err != nil {
		t.Fatalf("UpdatePage: %v", err)
	}
	if updated.Slug != "cookie-v2" || updated.Title != "Cookie 2" {
		t.Fatalf("unexpected updated page: %+v", updated)
	}
	if updated.IsActive {
		t.Error("expected IsActive to be set to false")
	}
}

func TestDeletePage_OwnContext_Deletes(t *testing.T) {
	repo := newFakeRepo()
	created, _ := CreatePage(repo, "ctx-1", CreatePageInput{Slug: "cookie", Title: "Cookie"})

	if err := DeletePage(repo, "ctx-1", created.ID); err != nil {
		t.Fatalf("DeletePage: %v", err)
	}
	if _, ok := repo.pages[created.ID]; ok {
		t.Fatal("page should be deleted")
	}
}

func TestDeletePage_ForeignContext_Fails(t *testing.T) {
	repo := newFakeRepo()
	created, _ := CreatePage(repo, "ctx-1", CreatePageInput{Slug: "cookie", Title: "Cookie"})

	if err := DeletePage(repo, "ctx-2", created.ID); err == nil {
		t.Fatal("expected error for foreign context")
	}
	if _, ok := repo.pages[created.ID]; !ok {
		t.Fatal("foreign-context page must not be deleted")
	}
}

func TestListPages_FiltersByContext(t *testing.T) {
	repo := newFakeRepo()
	_, _ = CreatePage(repo, "ctx-1", CreatePageInput{Slug: "a", Title: "A"})
	_, _ = CreatePage(repo, "ctx-1", CreatePageInput{Slug: "b", Title: "B"})
	_, _ = CreatePage(repo, "ctx-2", CreatePageInput{Slug: "c", Title: "C"})

	pages, err := ListPages(repo, "ctx-1")
	if err != nil {
		t.Fatalf("ListPages: %v", err)
	}
	if len(pages) != 2 {
		t.Fatalf("expected 2 pages for ctx-1, got %d", len(pages))
	}
}

func TestGetPageBySlug(t *testing.T) {
	repo := newFakeRepo()
	_, _ = CreatePage(repo, "ctx-1", CreatePageInput{Slug: "cookie", Title: "Cookie"})

	p, err := GetPageBySlug(repo, "ctx-1", "cookie")
	if err != nil {
		t.Fatalf("GetPageBySlug: %v", err)
	}
	if p.Slug != "cookie" {
		t.Fatalf("unexpected slug %q", p.Slug)
	}

	if _, err := GetPageBySlug(repo, "ctx-1", "missing"); err == nil {
		t.Fatal("expected error for missing slug")
	}
}
