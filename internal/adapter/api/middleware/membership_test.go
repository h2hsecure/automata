package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/h2hsecure/automata/internal/domain/context"
)

// setupMembershipEngine builds an engine that simulates the production chain:
// fakeAuth (sets user_id, optionally auth_via_api_key) → fakeResolver (sets
// context_id) → RequireContextMembership → handler (200).
func setupMembershipEngine(repo *stubUserRepo, contextID string, authViaAPIKey bool, handler gin.HandlerFunc) *gin.Engine {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	fakeAuth := func(c *gin.Context) {
		c.Set("user_id", "u1")
		if authViaAPIKey {
			c.Set("auth_via_api_key", true)
		}
		c.Next()
	}
	fakeResolver := func(c *gin.Context) {
		c.Set("context_id", contextID)
		c.Next()
	}
	engine.GET("/context/contacts", fakeAuth, fakeResolver, RequireContextMembership(repo), handler)
	return engine
}

func doRequest(engine *gin.Engine) int {
	req := httptest.NewRequest(http.MethodGet, "/context/contacts", nil)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	return w.Code
}

// TestMembership_UserOfContextAGets403OnContextB is the regression test for
// the tenant-isolation finding: a user of context A requesting context B must
// be rejected, not served context B's data.
func TestMembership_UserOfContextAGets403OnContextB(t *testing.T) {
	repo := &stubUserRepo{
		user:        &context.User{ID: "u1", IsAdmin: false},
		memberships: map[string]string{"u1": "ctx-a"},
	}
	engine := setupMembershipEngine(repo, "ctx-b", false, func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	if code := doRequest(engine); code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", code)
	}
}

// TestMembership_MemberPasses: a user with a membership row for the resolved
// context is allowed.
func TestMembership_MemberPasses(t *testing.T) {
	repo := &stubUserRepo{
		user:        &context.User{ID: "u1", IsAdmin: false},
		memberships: map[string]string{"u1": "ctx-a"},
	}
	engine := setupMembershipEngine(repo, "ctx-a", false, func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	if code := doRequest(engine); code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
}

// TestMembership_PlatformAdminPasses: a platform admin bypasses membership
// (DB-verified, not JWT-claim).
func TestMembership_PlatformAdminPasses(t *testing.T) {
	repo := &stubUserRepo{
		user:        &context.User{ID: "u1", IsAdmin: true},
		memberships: map[string]string{},
	}
	engine := setupMembershipEngine(repo, "ctx-b", false, func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	if code := doRequest(engine); code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
}

// TestMembership_APIKeyRequestsSkip: API-key requests are scoped to their
// context by AuthMiddleware itself, so membership is not re-checked here.
func TestMembership_APIKeyRequestsSkip(t *testing.T) {
	repo := &stubUserRepo{
		user:        &context.User{ID: "u1", IsAdmin: false},
		memberships: map[string]string{}, // no membership row at all
	}
	engine := setupMembershipEngine(repo, "ctx-b", true, func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	if code := doRequest(engine); code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
}

// TestMembership_PublicRoutesPass: no user and no context (public route) →
// the middleware must not block.
func TestMembership_PublicRoutesPass(t *testing.T) {
	repo := &stubUserRepo{}
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	// No user_id, no context_id.
	engine.GET("/public", RequireContextMembership(repo), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	if code := doRequestPublic(engine); code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
}

func doRequestPublic(engine *gin.Engine) int {
	req := httptest.NewRequest(http.MethodGet, "/public", nil)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	return w.Code
}