package mcp

import (
	"context"
	_ "embed"
	"net/http"
	"strings"
	"sync"

	"github.com/hekemen/automata/internal/domain/auth"
	"github.com/hekemen/automata/internal/domain/contact"
	"github.com/hekemen/automata/internal/domain/form"
	"github.com/hekemen/automata/internal/domain/tracking"
	"github.com/hekemen/automata/internal/infrastructure/webhook"
	bannerusecase "github.com/hekemen/automata/internal/usecase/banner"
	emailusecase "github.com/hekemen/automata/internal/usecase/email"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Deps carries the application-layer dependencies for every MCP tool.
// Tools call these directly (no REST proxying) and are scoped to the
// authenticated key's context.
type Deps struct {
	AuthService  auth.AuthService
	ContactRepo  contact.Repository
	FormRepo     form.Repository
	BannerUsecase *bannerusecase.BannerUsecase
	TrackingRepo tracking.Repository
	EmailUsecase *emailusecase.EmailUsecase
	WebhookSvc   *webhook.Service
}

type apiKeyCtxKey struct{}

// SkillMD is the MCP client skill guide, served at GET /api/mcp-skill.md.
//
//go:embed skill.md
var SkillMD string

// NewHandler returns an http.Handler serving stateless streamable MCP at the
// mounted path. Every request must carry a valid Bearer API key.
func NewHandler(deps Deps) http.Handler {
	inner := mcp.NewStreamableHTTPHandler(
		func(r *http.Request) *mcp.Server {
			v := r.Context().Value(apiKeyCtxKey{})
			apiKey, ok := v.(*auth.APIKey)
			if !ok {
				return nil // defensive: middleware always sets it; nil → 400
			}
			return getOrCreateServer(deps, apiKey.ContextID)
		},
		&mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true},
	)
	return authMiddleware{deps: deps, inner: inner}
}

type authMiddleware struct {
	deps  Deps
	inner http.Handler
}

func (m authMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	key := bearerToken(r)
	if key == "" {
		http.Error(w, `{"error":"missing api key"}`, http.StatusUnauthorized)
		return
	}
	apiKey, err := m.deps.AuthService.ValidateAPIKey(key)
	if err != nil {
		http.Error(w, `{"error":"invalid api key"}`, http.StatusUnauthorized)
		return
	}
	r = r.WithContext(context.WithValue(r.Context(), apiKeyCtxKey{}, apiKey))
	m.inner.ServeHTTP(w, r)
}

// bearerToken extracts the Bearer token from the Authorization header
// ("" when missing or malformed).
func bearerToken(r *http.Request) string {
	header := r.Header.Get("Authorization")
	return strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
}

// servers caches one MCP server per context (one process, few contexts,
// unbounded by design).
var servers sync.Map // contextID -> *mcp.Server

func getOrCreateServer(deps Deps, contextID string) *mcp.Server {
	if v, ok := servers.Load(contextID); ok {
		return v.(*mcp.Server)
	}
	s := mcp.NewServer(&mcp.Implementation{Name: "automata", Version: "v1.0.0"}, nil)
	registerAllTools(s, deps, contextID)
	servers.Store(contextID, s)
	return s
}

func registerAllTools(s *mcp.Server, d Deps, contextID string) {
	// per-domain registrations added in later tasks
}