package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/h2hsecure/automata/internal/domain/context"
)

// setupAccessEngine simulates: fakeAuth (user_id) → RequireContextAccess → handler.
// Routes registered per-case with the path/params/header under test.
func setupAccessEngine(repo *stubUserRepo, method, path string, mw gin.HandlerFunc, handler gin.HandlerFunc) *gin.Engine {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	fakeAuth := func(c *gin.Context) {
		c.Set("user_id", "u1")
		c.Next()
	}
	chain := []gin.HandlerFunc{fakeAuth, mw, handler}
	switch method {
	case http.MethodGet:
		engine.GET(path, chain...)
	default:
		engine.POST(path, chain...)
	}
	return engine
}

func doGet(engine *gin.Engine, path string, headers map[string]string) int {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	return w.Code
}

var adminRepo = &stubUserRepo{user: &context.User{ID: "u1", IsAdmin: true}}
var memberRepo = &stubUserRepo{
	user:        &context.User{ID: "u1", IsAdmin: false},
	memberships: map[string]string{"u1": "ctx-a"},
}
var strangerRepo = &stubUserRepo{
	user:        &context.User{ID: "u1", IsAdmin: false},
	memberships: map[string]string{},
}

func TestContextAccess_ContextIDParam_StrangerGets403(t *testing.T) {
	engine := setupAccessEngine(strangerRepo, http.MethodGet, "/admin/configs/:contextId",
		RequireContextAccess(strangerRepo, "contextId"),
		func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })

	if code := doGet(engine, "/admin/configs/ctx-b", nil); code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", code)
	}
}

func TestContextAccess_ContextIDParam_MemberPasses(t *testing.T) {
	engine := setupAccessEngine(memberRepo, http.MethodGet, "/admin/configs/:contextId",
		RequireContextAccess(memberRepo, "contextId"),
		func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })

	if code := doGet(engine, "/admin/configs/ctx-a", nil); code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
}

func TestContextAccess_IDParam_StrangerGets403(t *testing.T) {
	engine := setupAccessEngine(strangerRepo, http.MethodGet, "/admin/contexts/:id/settings",
		RequireContextAccess(strangerRepo, "id"),
		func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })

	if code := doGet(engine, "/admin/contexts/ctx-b/settings", nil); code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", code)
	}
}

func TestContextAccess_IDParam_MemberPasses(t *testing.T) {
	engine := setupAccessEngine(memberRepo, http.MethodGet, "/admin/contexts/:id/settings",
		RequireContextAccess(memberRepo, "id"),
		func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })

	if code := doGet(engine, "/admin/contexts/ctx-a/settings", nil); code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
}

func TestContextAccess_HeaderFallback_StrangerGets403(t *testing.T) {
	engine := setupAccessEngine(strangerRepo, http.MethodGet, "/admin/api-keys",
		RequireContextAccess(strangerRepo),
		func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })

	if code := doGet(engine, "/admin/api-keys", map[string]string{"X-Context-ID": "ctx-b"}); code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", code)
	}
}

func TestContextAccess_HeaderFallback_MemberPasses(t *testing.T) {
	engine := setupAccessEngine(memberRepo, http.MethodGet, "/admin/api-keys",
		RequireContextAccess(memberRepo),
		func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })

	if code := doGet(engine, "/admin/api-keys", map[string]string{"X-Context-ID": "ctx-a"}); code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
}

func TestContextAccess_NoContextResolves_Passes(t *testing.T) {
	engine := setupAccessEngine(strangerRepo, http.MethodGet, "/admin/api-keys",
		RequireContextAccess(strangerRepo),
		func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })

	if code := doGet(engine, "/admin/api-keys", nil); code != http.StatusOK {
		t.Fatalf("expected 200 (no context to validate), got %d", code)
	}
}

func TestContextAccess_PlatformAdminPassesAnyContext(t *testing.T) {
	engine := setupAccessEngine(adminRepo, http.MethodGet, "/admin/configs/:contextId",
		RequireContextAccess(adminRepo, "contextId"),
		func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })

	if code := doGet(engine, "/admin/configs/ctx-anywhere", nil); code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
}