package api

import (
	"github.com/gin-gonic/gin"
	"github.com/h2hsecure/automata/internal/adapter/api/handler"
	"github.com/h2hsecure/automata/internal/adapter/api/middleware"
	"github.com/h2hsecure/automata/internal/domain/auth"
	"github.com/h2hsecure/automata/internal/domain/context"
	"github.com/h2hsecure/automata/internal/domain/form"
	"github.com/h2hsecure/automata/internal/domain/staticfile"
	"github.com/h2hsecure/automata/internal/infrastructure/storage"
)

// RegisterStaticFileRoutes registers the authenticated, context-resolved
// admin CRUD routes for context static files:
//
//	GET    <prefix>/context/static-files
//	POST   <prefix>/context/static-files
//	DELETE <prefix>/context/static-files/:name
//
// It is separate from NewServer so callers without file storage (e.g. cicd
// harnesses) can build the API server unchanged. A nil store makes uploads
// respond 501; list and delete still work.
func RegisterStaticFileRoutes(
	engine *gin.Engine,
	contextRepo context.Repository,
	authService auth.AuthService,
	repo staticfile.Repository,
	store *storage.FileStore,
	prefix string,
) {
	if prefix == "" {
		prefix = "/api"
	}

	h := handler.NewStaticFileHandler(repo, store)
	g := engine.Group(prefix).Use(ContextResolver(contextRepo))
	g.GET("/context/static-files", middleware.AuthMiddleware(authService), h.List)
	g.POST("/context/static-files", middleware.AuthMiddleware(authService), h.Upload)
	g.DELETE("/context/static-files/:name", middleware.AuthMiddleware(authService), h.Delete)
}

// RegisterSubmissionFileRoutes registers the authenticated, context-resolved
// admin route for downloading files attached to form submissions:
//
//	GET <prefix>/context/forms/:id/submissions/:sid/files/:idx
//
// Separate from NewServer for the same reason as RegisterStaticFileRoutes.
// A nil store makes downloads respond 404.
func RegisterSubmissionFileRoutes(
	engine *gin.Engine,
	contextRepo context.Repository,
	authService auth.AuthService,
	formRepo form.Repository,
	store *storage.FileStore,
	prefix string,
) {
	if prefix == "" {
		prefix = "/api"
	}

	h := handler.NewFormHandler(formRepo).WithFileStore(store)
	g := engine.Group(prefix).Use(ContextResolver(contextRepo))
	g.GET("/context/forms/:id/submissions/:sid/files/:idx", middleware.AuthMiddleware(authService), h.DownloadSubmissionFile)
}