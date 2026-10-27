package handler

import (
	"errors"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/hekemen/automata/internal/domain/staticfile"
	"github.com/hekemen/automata/internal/infrastructure/storage"
)

// StaticFileHandler serves admin CRUD for files linked to a context. Uploads
// store the byte stream in the shared FileStore and record a URL name in the
// database.
type StaticFileHandler struct {
	repo  staticfile.Repository
	store *storage.FileStore
}

// NewStaticFileHandler creates a StaticFileHandler. store may be nil, in which
// case uploads respond with 501.
func NewStaticFileHandler(repo staticfile.Repository, store *storage.FileStore) *StaticFileHandler {
	return &StaticFileHandler{repo: repo, store: store}
}

// staticNameRe permits a single URL-safe path segment: alphanumerics, dot,
// dash, underscore. Rejects traversal and empty names.
var staticNameRe = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

func (h *StaticFileHandler) List(c *gin.Context) {
	contextID := c.GetString("context_id")
	if contextID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "context not found"})
		return
	}

	files, err := h.repo.List(contextID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if files == nil {
		files = []*staticfile.StaticFile{}
	}

	c.JSON(http.StatusOK, gin.H{"files": files})
}

func (h *StaticFileHandler) Upload(c *gin.Context) {
	if h.store == nil {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "file storage is not configured"})
		return
	}

	contextID := c.GetString("context_id")
	if contextID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "context not found"})
		return
	}

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file part required"})
		return
	}
	defer file.Close()

	name := c.PostForm("name")
	if name == "" {
		name = header.Filename
	}
	if !staticNameRe.MatchString(name) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid file name"})
		return
	}

	data, err := io.ReadAll(file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "read file failed"})
		return
	}

	token, err := h.store.StoreMeta(contextID, storage.ExtOf(name), name, header.Header.Get("Content-Type"), data)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "store file failed"})
		return
	}

	rec := &staticfile.StaticFile{
		ContextID: contextID,
		Name:      name,
		Token:     token,
		Mime:      header.Header.Get("Content-Type"),
		Size:      header.Size,
	}
	if err := h.repo.Create(rec); err != nil {
		// Roll back the stored file if the record can't be persisted.
		_ = h.store.Remove(token)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "record static file failed"})
		return
	}

	c.JSON(http.StatusCreated, rec)
}

func (h *StaticFileHandler) Delete(c *gin.Context) {
	contextID := c.GetString("context_id")
	if contextID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "context not found"})
		return
	}

	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name required"})
		return
	}

	rec, err := h.repo.GetByName(contextID, name)
	if err != nil {
		if isNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "static file not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err := h.repo.Delete(contextID, name); err != nil {
		if isNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "static file not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Best-effort removal of the stored file; a missing file is not an error.
	if h.store != nil {
		_ = h.store.Remove(rec.Token)
	}

	c.Status(http.StatusNoContent)
}

func isNotFound(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, os.ErrNotExist) {
		return true
	}
	return strings.Contains(err.Error(), "not found")
}