package handler

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/hekemen/automata/internal/domain/staticfile"
	"github.com/hekemen/automata/internal/infrastructure/storage"
)

// fakeStaticFileRepo is an in-memory staticfile.Repository for handler tests.
type fakeStaticFileRepo struct {
	files    []*staticfile.StaticFile
	missing  bool // if true, Delete reports not found
}

func (r *fakeStaticFileRepo) Create(f *staticfile.StaticFile) error {
	r.files = append(r.files, f)
	return nil
}

func (r *fakeStaticFileRepo) GetByName(contextID, name string) (*staticfile.StaticFile, error) {
	for _, f := range r.files {
		if f.ContextID == contextID && f.Name == name {
			return f, nil
		}
	}
	return nil, os.ErrNotExist
}

func (r *fakeStaticFileRepo) List(contextID string) ([]*staticfile.StaticFile, error) {
	var out []*staticfile.StaticFile
	for _, f := range r.files {
		if f.ContextID == contextID {
			out = append(out, f)
		}
	}
	return out, nil
}

func (r *fakeStaticFileRepo) Delete(contextID, name string) error {
	if r.missing {
		return os.ErrNotExist
	}
	for i, f := range r.files {
		if f.ContextID == contextID && f.Name == name {
			r.files = append(r.files[:i], r.files[i+1:]...)
			return nil
		}
	}
	return os.ErrNotExist
}

func newTestStaticStore(t *testing.T) *storage.FileStore {
	t.Helper()
	s, err := storage.New(t.TempDir())
	if err != nil {
		t.Fatalf("storage.New: %v", err)
	}
	return s
}

func multipartBody(t *testing.T, filename, nameField, content string) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, err := w.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	if _, err := fw.Write([]byte(content)); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if nameField != "" {
		if err := w.WriteField("name", nameField); err != nil {
			t.Fatalf("WriteField name: %v", err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	return &buf, w.FormDataContentType()
}

func TestStaticFileUpload_StoresAndCreatesRecord(t *testing.T) {
	store := newTestStaticStore(t)
	repo := &fakeStaticFileRepo{}
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	body, ct := multipartBody(t, "logo.png", "", "PNGDATA")
	c.Request = httptest.NewRequest("POST", "/api/context/static-files", body)
	c.Request.Header.Set("Content-Type", ct)
	c.Set("context_id", "ctx-1")

	h := NewStaticFileHandler(repo, store)
	h.Upload(c)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	if len(repo.files) != 1 {
		t.Fatalf("expected 1 record, got %d", len(repo.files))
	}
	rec := repo.files[0]
	if rec.ContextID != "ctx-1" || rec.Name != "logo.png" {
		t.Fatalf("unexpected record: %+v", rec)
	}
	if rec.Token == "" {
		t.Fatal("expected non-empty token")
	}
	if _, err := store.Open(rec.Token); err != nil {
		t.Fatalf("stored file missing: %v", err)
	}
}

func TestStaticFileUpload_UsesProvidedName(t *testing.T) {
	store := newTestStaticStore(t)
	repo := &fakeStaticFileRepo{}
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	body, ct := multipartBody(t, "original.jpg", "hero-image.jpg", "JPGDATA")
	c.Request = httptest.NewRequest("POST", "/api/context/static-files", body)
	c.Request.Header.Set("Content-Type", ct)
	c.Set("context_id", "ctx-1")

	h := NewStaticFileHandler(repo, store)
	h.Upload(c)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	if len(repo.files) != 1 || repo.files[0].Name != "hero-image.jpg" {
		t.Fatalf("expected name hero-image.jpg, got %+v", repo.files)
	}
}

func TestStaticFileUpload_NilStore_Unsupported(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	body, ct := multipartBody(t, "a.txt", "", "x")
	c.Request = httptest.NewRequest("POST", "/api/context/static-files", body)
	c.Request.Header.Set("Content-Type", ct)
	c.Set("context_id", "ctx-1")

	h := NewStaticFileHandler(&fakeStaticFileRepo{}, nil)
	h.Upload(c)

	if w.Code != http.StatusNotImplemented {
		t.Fatalf("expected 501, got %d", w.Code)
	}
}

func TestStaticFileUpload_MissingFilePart_BadRequest(t *testing.T) {
	store := newTestStaticStore(t)
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/context/static-files", bytes.NewBufferString(""))
	c.Set("context_id", "ctx-1")

	h := NewStaticFileHandler(&fakeStaticFileRepo{}, store)
	h.Upload(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestStaticFileUpload_InvalidName_BadRequest(t *testing.T) {
	store := newTestStaticStore(t)
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	// name containing a slash / traversal
	body, ct := multipartBody(t, "a.txt", "../evil.txt", "x")
	c.Request = httptest.NewRequest("POST", "/api/context/static-files", body)
	c.Request.Header.Set("Content-Type", ct)
	c.Set("context_id", "ctx-1")

	h := NewStaticFileHandler(&fakeStaticFileRepo{}, store)
	h.Upload(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestStaticFileList_ReturnsFiles(t *testing.T) {
	store := newTestStaticStore(t)
	repo := &fakeStaticFileRepo{files: []*staticfile.StaticFile{
		{ContextID: "ctx-1", Name: "a.png", Token: "ctx-1/aaaa.png"},
		{ContextID: "ctx-2", Name: "other.png", Token: "ctx-2/bbbb.png"},
	}}
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/context/static-files", nil)
	c.Set("context_id", "ctx-1")

	h := NewStaticFileHandler(repo, store)
	h.List(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "a.png") {
		t.Fatalf("expected a.png in response, got %s", w.Body.String())
	}
	if strings.Contains(w.Body.String(), "other.png") {
		t.Fatalf("response leaked other context: %s", w.Body.String())
	}
}

func TestStaticFileDelete_RemovesRecordAndFile(t *testing.T) {
	store := newTestStaticStore(t)
	repo := &fakeStaticFileRepo{}
	// Pre-create a real file in the store and a matching record.
	token, _ := store.StoreMeta("ctx-1", ".png", "a.png", "image/png", []byte("img"))
	repo.files = append(repo.files, &staticfile.StaticFile{
		ContextID: "ctx-1", Name: "a.png", Token: token, Mime: "image/png", Size: 3,
	})

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("DELETE", "/api/context/static-files/a.png", nil)
	c.Set("context_id", "ctx-1")
	c.Params = gin.Params{{Key: "name", Value: "a.png"}}

	h := NewStaticFileHandler(repo, store)
	h.Delete(c)

	// A bare 204 isn't captured by the direct-call test harness (no body to
	// flush the status), so assert the meaningful side effects and that the
	// success path was taken (no "not found" error body).
	if strings.Contains(w.Body.String(), "not found") {
		t.Fatalf("unexpected not-found body: %s", w.Body.String())
	}
	if len(repo.files) != 0 {
		t.Fatalf("expected record removed, got %d", len(repo.files))
	}
	if _, err := store.Open(token); err == nil {
		t.Fatal("expected file removed from store")
	}
}

func TestStaticFileDelete_NotFound(t *testing.T) {
	store := newTestStaticStore(t)
	repo := &fakeStaticFileRepo{missing: true}
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("DELETE", "/api/context/static-files/nope.png", nil)
	c.Set("context_id", "ctx-1")
	c.Params = gin.Params{{Key: "name", Value: "nope.png"}}

	h := NewStaticFileHandler(repo, store)
	h.Delete(c)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}