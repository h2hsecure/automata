package tracking

import (
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/hekemen/automata/internal/domain/form"
	"github.com/hekemen/automata/internal/infrastructure/storage"
)

// UploadHandler serves public, unauthenticated form file uploads. Files are
// stored in the shared FileStore and referenced at submission time by the
// returned token.
type UploadHandler struct {
	store    *storage.FileStore
	formRepo form.Repository
}

// NewUploadHandler creates an UploadHandler. store may be nil, in which case
// uploads respond with 501.
func NewUploadHandler(store *storage.FileStore, formRepo form.Repository) *UploadHandler {
	return &UploadHandler{store: store, formRepo: formRepo}
}

// RegisterUploadRoutes registers the public form upload endpoint on the
// tracking engine: POST /form/upload/:contextID/:slug?field=<slug>.
func RegisterUploadRoutes(r *gin.Engine, h *UploadHandler) {
	r.POST("/form/upload/:contextID/:slug", h.Upload)
}

// Upload handles a multipart upload. The field's slug is passed as the
// "field" query parameter; the field's validation map (accept, max_size)
// governs what is accepted.
func (h *UploadHandler) Upload(c *gin.Context) {
	if h.store == nil {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "file uploads are not configured"})
		return
	}

	contextID := c.Param("contextID")
	slug := c.Param("slug")
	if contextID == "" || slug == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "contextID and slug required"})
		return
	}

	f, err := h.formRepo.GetBySlug(contextID, slug)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "form not found"})
		return
	}

	accept, maxSize := fieldConstraints(f, c.Query("field"))

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file part required"})
		return
	}
	defer file.Close()

	if maxSize > 0 && header.Size > maxSize {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "file too large"})
		return
	}

	mime := header.Header.Get("Content-Type")
	if !storage.Accepts(header.Filename, mime, accept) {
		c.JSON(http.StatusUnsupportedMediaType, gin.H{"error": "file type not allowed"})
		return
	}

	data, err := io.ReadAll(file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "read file failed"})
		return
	}

	token, err := h.store.StoreMeta(contextID, storage.ExtOf(header.Filename), header.Filename, mime, data)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "store file failed"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"token": token,
		"name":  header.Filename,
		"size":  header.Size,
		"type":  mime,
	})
}

// fieldConstraints extracts accept patterns and max size from the form
// field's validation map. Unknown or missing fields have no constraints.
func fieldConstraints(f *form.Form, fieldSlug string) (accept []string, maxSize int64) {
	if fieldSlug == "" || f == nil {
		return nil, 0
	}
	for i := range f.Fields {
		field := &f.Fields[i]
		if field.Slug != fieldSlug || field.Validation == nil {
			continue
		}
		if list, ok := field.Validation["accept"].([]interface{}); ok {
			for _, v := range list {
				if s, ok := v.(string); ok {
					accept = append(accept, s)
				}
			}
		}
		switch m := field.Validation["max_size"].(type) {
		case float64:
			maxSize = int64(m)
		case int:
			maxSize = int64(m)
		case int64:
			maxSize = m
		}
		return accept, maxSize
	}
	return nil, 0
}