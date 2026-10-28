package tracking

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/h2hsecure/automata/internal/domain/staticfile"
	"github.com/h2hsecure/automata/internal/infrastructure/storage"
)

type fakeStaticRepoForTrack struct {
	files []*staticfile.StaticFile
}

func (r *fakeStaticRepoForTrack) Create(f *staticfile.StaticFile) error {
	r.files = append(r.files, f)
	return nil
}

func (r *fakeStaticRepoForTrack) GetByName(contextID, name string) (*staticfile.StaticFile, error) {
	for _, f := range r.files {
		if f.ContextID == contextID && f.Name == name {
			return f, nil
		}
	}
	return nil, os.ErrNotExist
}

func (r *fakeStaticRepoForTrack) List(contextID string) ([]*staticfile.StaticFile, error) {
	return r.files, nil
}

func (r *fakeStaticRepoForTrack) Delete(contextID, name string) error {
	return os.ErrNotExist
}

func newTrackStaticStore(t *testing.T) *storage.FileStore {
	t.Helper()
	s, err := storage.New(t.TempDir())
	if err != nil {
		t.Fatalf("storage.New: %v", err)
	}
	return s
}

func TestStaticServe_ReturnsStoredContent(t *testing.T) {
	store := newTrackStaticStore(t)
	token, err := store.StoreMeta("ctx-1", ".png", "logo.png", "image/png", []byte("PNGDATA"))
	if err != nil {
		t.Fatalf("StoreMeta: %v", err)
	}
	repo := &fakeStaticRepoForTrack{files: []*staticfile.StaticFile{
		{ContextID: "ctx-1", Name: "logo.png", Token: token, Mime: "image/png", Size: 7},
	}}

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/static/ctx-1/logo.png", nil)
	c.Params = gin.Params{{Key: "contextID", Value: "ctx-1"}, {Key: "name", Value: "logo.png"}}

	h := NewStaticHandler(repo, store)
	h.Serve(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if got := w.Body.String(); got != "PNGDATA" {
		t.Fatalf("expected PNGDATA, got %q", got)
	}
	if ct := w.Header().Get("Content-Type"); ct != "image/png" {
		t.Fatalf("expected image/png, got %q", ct)
	}
}

func TestStaticServe_NotFound(t *testing.T) {
	store := newTrackStaticStore(t)
	repo := &fakeStaticRepoForTrack{}
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/static/ctx-1/nope.png", nil)
	c.Params = gin.Params{{Key: "contextID", Value: "ctx-1"}, {Key: "name", Value: "nope.png"}}

	h := NewStaticHandler(repo, store)
	h.Serve(c)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestStaticServe_NilStore_NotFound(t *testing.T) {
	repo := &fakeStaticRepoForTrack{files: []*staticfile.StaticFile{
		{ContextID: "ctx-1", Name: "a.png", Token: "ctx-1/aaaa.png"},
	}}
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/static/ctx-1/a.png", nil)
	c.Params = gin.Params{{Key: "contextID", Value: "ctx-1"}, {Key: "name", Value: "a.png"}}

	h := NewStaticHandler(repo, nil)
	h.Serve(c)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestRegisterStaticRoutes_RegistersRoute(t *testing.T) {
	store := newTrackStaticStore(t)
	token, _ := store.StoreMeta("ctx-1", ".txt", "hi.txt", "text/plain", []byte("hi"))
	repo := &fakeStaticRepoForTrack{files: []*staticfile.StaticFile{
		{ContextID: "ctx-1", Name: "hi.txt", Token: token, Mime: "text/plain", Size: 2},
	}}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterStaticRoutes(r, NewStaticHandler(repo, store))

	req := httptest.NewRequest("GET", "/static/ctx-1/hi.txt", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if got := w.Body.String(); got != "hi" {
		t.Fatalf("expected hi, got %q", got)
	}
}