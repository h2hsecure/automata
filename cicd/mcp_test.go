package cicd_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"

	"github.com/gin-gonic/gin"
	"github.com/hekemen/automata/cicd/support"
	mcpadapter "github.com/hekemen/automata/internal/adapter/mcp"
	infraauth "github.com/hekemen/automata/internal/infrastructure/auth"
	auth_repo "github.com/hekemen/automata/internal/infrastructure/auth/repo"
	ctxrepo "github.com/hekemen/automata/internal/infrastructure/context/repo"
	"golang.org/x/crypto/bcrypt"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// mcpCall POSTs a JSON-RPC request to /api/mcp on the given handler with an
// optional Bearer key (empty key → no Authorization header).
func mcpCall(handler http.Handler, key string, id int, method string, params map[string]any) *httptest.ResponseRecorder {
	body, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  method,
		"params":  params,
	})
	Expect(err).NotTo(HaveOccurred())
	req := httptest.NewRequest("POST", "/api/mcp", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	return w
}

// mcpToolText unmarshals the JSON-RPC response, asserts the call succeeded
// (isError is false), and returns the text of the first content item.
func mcpToolText(w *httptest.ResponseRecorder) string {
	var resp struct {
		Result struct {
			IsError bool `json:"isError"`
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"result"`
	}
	Expect(json.Unmarshal(w.Body.Bytes(), &resp)).To(Succeed(), "body: %s", w.Body.String())
	Expect(resp.Result.IsError).To(BeFalse(), "body: %s", w.Body.String())
	Expect(resp.Result.Content).NotTo(BeEmpty(), "body: %s", w.Body.String())
	return resp.Result.Content[0].Text
}

var _ = Describe("MCP server (POST /api/mcp)", func() {
	var (
		engine   *gin.Engine
		apiKey   string
		adminJWT string
		contextID string
	)

	initParams := map[string]any{
		"protocolVersion": "2025-06-18",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "cicd", "version": "1.0"},
	}

	BeforeEach(func() {
		infraauth.InitForTesting("test-secret-key-for-jwt-signing-32bytes", "test-cookie-secret-32bytes-xxxx!")

		// Mint a real API key through the production admin endpoint (same
		// flow as swagger_endpoints_test.go).
		full := setupCreationServer()
		adminJWT = loginAsAdmin(full)
		contextID = support.NewTestUUID()
		_, err := pool.Exec(ctx, `
			INSERT INTO contexts (id, slug, name, created_at, updated_at)
			VALUES ($1, $2, $3, NOW(), NOW())
		`, contextID, "mcp-ctx-"+contextID, "MCP Test Context")
		Expect(err).NotTo(HaveOccurred())

		body := `{"name":"mcp-test-key-` + support.NewTestUUID() + `"}`
		req := httptest.NewRequest("POST", "/api/admin/api-keys", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+adminJWT)
		req.Header.Set("X-Context-ID", contextID)
		w := httptest.NewRecorder()
		full.ServeHTTP(w, req)
		Expect(w.Code).To(Equal(http.StatusCreated), "create api key: %s", w.Body.String())
		var resp struct {
			Key string `json:"key"`
		}
		Expect(json.Unmarshal(w.Body.Bytes(), &resp)).To(Succeed())
		Expect(resp.Key).NotTo(BeEmpty())
		apiKey = resp.Key

		// Minimal engine serving only the MCP route against the same pool.
		gin.SetMode(gin.TestMode)
		engine = gin.New()
		deps := mcpadapter.Deps{
			AuthService: infraauth.NewServiceWithAPIKeys(ctxrepo.NewUserPostgresRepo(pool), auth_repo.NewApiKeyPostgresRepo(pool)),
		}
		engine.POST("/api/mcp", gin.WrapH(mcpadapter.NewHandler(deps)))
	})

	AfterEach(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM api_keys")
		_, _ = pool.Exec(ctx, `DELETE FROM contexts WHERE id = $1`, contextID)
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE email LIKE 'swagger-admin-%@example.com' OR email LIKE 'mcp-expired@%'`)
	})

	It("rejects a request with no Authorization header", func() {
		w := mcpCall(engine, "", 1, "initialize", initParams)
		Expect(w.Code).To(Equal(http.StatusUnauthorized))
	})

	It("rejects an invalid API key", func() {
		w := mcpCall(engine, "sk-invalid", 1, "initialize", initParams)
		Expect(w.Code).To(Equal(http.StatusUnauthorized))
	})

	It("rejects an expired API key", func() {
		userID := support.NewTestUUID()
		hash, err := bcrypt.GenerateFromPassword([]byte("sk-expired-test"), bcrypt.DefaultCost)
		Expect(err).NotTo(HaveOccurred())
		_, err = pool.Exec(ctx, `
			INSERT INTO users (id, email, password_hash, is_admin, created_at, updated_at)
			VALUES ($1, $2, $3, false, NOW(), NOW())
		`, userID, "mcp-expired@"+contextID+".example.com", string(hash))
		Expect(err).NotTo(HaveOccurred())
		_, err = pool.Exec(ctx, `
			INSERT INTO api_keys (id, context_id, user_id, key_hash, name, expires_at, created_at)
			VALUES ($1, $2, $3, $4, 'mcp-expired-key', NOW() - interval '1 hour', NOW())
		`, support.NewTestUUID(), contextID, userID, string(hash))
		Expect(err).NotTo(HaveOccurred())

		w := mcpCall(engine, "sk-expired-test", 1, "initialize", initParams)
		Expect(w.Code).To(Equal(http.StatusUnauthorized))
	})

	It("rejects a JWT (only API keys are accepted)", func() {
		w := mcpCall(engine, adminJWT, 1, "initialize", initParams)
		Expect(w.Code).To(Equal(http.StatusUnauthorized))
	})

	It("serves initialize with a valid API key", func() {
		w := mcpCall(engine, apiKey, 1, "initialize", initParams)
		Expect(w.Code).To(Equal(http.StatusOK))
		Expect(w.Body.String()).To(ContainSubstring("serverInfo"))
	})

	It("returns an empty tool list", func() {
		w := mcpCall(engine, apiKey, 2, "tools/list", map[string]any{})
		Expect(w.Code).To(Equal(http.StatusOK))
		var resp struct {
			Result struct {
				Tools []any `json:"tools"`
			} `json:"result"`
		}
		Expect(json.Unmarshal(w.Body.Bytes(), &resp)).To(Succeed(), "body: %s", w.Body.String())
		Expect(resp.Result.Tools).To(BeEmpty())
	})
})

var _ = Describe("MCP wiring (full engine)", func() {
	var (
		engine    *gin.Engine
		adminJWT  string
		apiKey    string
		contextID string
	)

	BeforeEach(func() {
		infraauth.InitForTesting("test-secret-key-for-jwt-signing-32bytes", "test-cookie-secret-32bytes-xxxx!")
		engine = setupCreationServer()
		adminJWT = loginAsAdmin(engine)

		contextID = support.NewTestUUID()
		_, err := pool.Exec(ctx, `
			INSERT INTO contexts (id, slug, name, created_at, updated_at)
			VALUES ($1, $2, $3, NOW(), NOW())
		`, contextID, "mcp-wire-"+contextID, "MCP Wiring Context")
		Expect(err).NotTo(HaveOccurred())

		body := `{"name":"mcp-wire-key-` + support.NewTestUUID() + `"}`
		req := httptest.NewRequest("POST", "/api/admin/api-keys", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+adminJWT)
		req.Header.Set("X-Context-ID", contextID)
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, req)
		Expect(w.Code).To(Equal(http.StatusCreated), "create api key: %s", w.Body.String())
		var resp struct {
			Key string `json:"key"`
		}
		Expect(json.Unmarshal(w.Body.Bytes(), &resp)).To(Succeed())
		apiKey = resp.Key
	})

	AfterEach(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM api_keys")
		_, _ = pool.Exec(ctx, `DELETE FROM contexts WHERE id = $1`, contextID)
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE email LIKE 'swagger-admin-%@example.com'`)
	})

	It("serves initialize over the production engine", func() {
		w := mcpCall(engine, apiKey, 1, "initialize", map[string]any{
			"protocolVersion": "2025-06-18",
			"capabilities":    map[string]any{},
			"clientInfo":      map[string]any{"name": "cicd", "version": "1.0"},
		})
		Expect(w.Code).To(Equal(http.StatusOK))
	})

	It("requires auth for GET /api/mcp-skill.md", func() {
		req := httptest.NewRequest("GET", "/api/mcp-skill.md", bytes.NewBuffer(nil))
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, req)
		Expect(w.Code).To(Equal(http.StatusUnauthorized))
	})

	It("serves the skill file with a valid API key", func() {
		req := httptest.NewRequest("GET", "/api/mcp-skill.md", bytes.NewBuffer(nil))
		req.Header.Set("Authorization", "Bearer "+apiKey)
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, req)
		Expect(w.Code).To(Equal(http.StatusOK))
		Expect(w.Header().Get("Content-Type")).To(HavePrefix("text/markdown"))
		Expect(w.Body.String()).To(ContainSubstring("Automata MCP"))
	})
})