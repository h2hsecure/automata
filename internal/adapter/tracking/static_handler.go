package tracking

import (
	"errors"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/hekemen/automata/internal/domain/staticfile"
	"github.com/hekemen/automata/internal/infrastructure/storage"
)

// StaticHandler serves public, unauthenticated static files linked to a
// context at GET /static/:contextID/:name.
type StaticHandler struct {
	repo  staticfile.Repository
	store *storage.FileStore
}

// NewStaticHandler creates a StaticHandler. store may be nil, in which case
// serving responds with 404.
func NewStaticHandler(repo staticfile.Repository, store *storage.FileStore) *StaticHandler {
	return &StaticHandler{repo: repo, store: store}
}

// RegisterStaticRoutes registers the public static file endpoint on the
// tracking engine: GET /static/:contextID/:name.
func RegisterStaticRoutes(r *gin.Engine, h *StaticHandler) {
	r.GET("/static/:contextID/:name", h.Serve)
}

func (h *StaticHandler) Serve(c *gin.Context) {
	if h.store == nil || h.repo == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "static files are not available"})
		return
	}

	contextID := c.Param("contextID")
	name := c.Param("name")
	if contextID == "" || name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "contextID and name required"})
		return
	}

	rec, err := h.repo.GetByName(contextID, name)
	if err != nil || isStaticNotFound(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "static file not found"})
		return
	}

	path, err := h.store.Path(rec.Token)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "static file not found"})
		return
	}

	// Prefer the recorded MIME type; c.File fills in a type by extension if
	// this is left empty.
	if rec.Mime != "" {
		c.Header("Content-Type", rec.Mime)
	}
	c.File(path)
}

func isStaticNotFound(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, os.ErrNotExist) {
		return true
	}
	return strings.Contains(err.Error(), "not found")
}