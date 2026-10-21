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

// mcpToolResult parses a tools/call response into (isError, first text item)
// without asserting on isError — for specs that expect tool errors.
func mcpToolResult(w *httptest.ResponseRecorder) (isError bool, text string) {
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
	if len(resp.Result.Content) > 0 {
		text = resp.Result.Content[0].Text
	}
	return resp.Result.IsError, text
}

// mcpTool calls tools/call for the named tool with the given arguments.
func mcpTool(handler http.Handler, key string, id int, name string, args map[string]any) *httptest.ResponseRecorder {
	return mcpCall(handler, key, id, "tools/call", map[string]any{"name": name, "arguments": args})
}

// mintAPIKey creates a new context and mints an API key for it through the
// production admin endpoint.
func mintAPIKey(engine *gin.Engine, adminJWT, prefix string) (apiKey, contextID string) {
	contextID = support.NewTestUUID()
	_, err := pool.Exec(ctx, `
		INSERT INTO contexts (id, slug, name, created_at, updated_at)
		VALUES ($1, $2, $3, NOW(), NOW())
	`, contextID, prefix+"-ctx-"+contextID, prefix+" Context")
	Expect(err).NotTo(HaveOccurred())

	body := `{"name":"` + prefix + "-key-" + support.NewTestUUID() + `"}`
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
	Expect(resp.Key).NotTo(BeEmpty())
	return resp.Key, contextID
}

// setupMCPEngine builds the production engine, mints an API key for a fresh
// context, and returns the engine, key, context ID, and the admin JWT (for
// minting additional keys).
func setupMCPEngine() (engine *gin.Engine, apiKey, contextID, adminJWT string) {
	infraauth.InitForTesting("test-secret-key-for-jwt-signing-32bytes", "test-cookie-secret-32bytes-xxxx!")
	engine = setupCreationServer()
	adminJWT = loginAsAdmin(engine)
	apiKey, contextID = mintAPIKey(engine, adminJWT, "mcp")
	return
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

	It("lists the registered tools", func() {
		w := mcpCall(engine, apiKey, 2, "tools/list", map[string]any{})
		Expect(w.Code).To(Equal(http.StatusOK))
		var resp struct {
			Result struct {
				Tools []struct {
					Name string `json:"name"`
				} `json:"tools"`
			} `json:"result"`
		}
		Expect(json.Unmarshal(w.Body.Bytes(), &resp)).To(Succeed(), "body: %s", w.Body.String())
		names := make([]string, 0, len(resp.Result.Tools))
		for _, t := range resp.Result.Tools {
			names = append(names, t.Name)
		}
		Expect(names).To(ConsistOf(
			"contacts_list", "contacts_get", "contacts_create", "contacts_update",
			"contacts_delete", "contacts_merge", "contacts_activity",
		))
	})
})

var _ = Describe("MCP wiring (full engine)", func() {
	var (
		engine    *gin.Engine
		apiKey    string
		contextID string
	)

	BeforeEach(func() {
		engine, apiKey, contextID, _ = setupMCPEngine()
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

var _ = Describe("MCP contacts tools", func() {
	var (
		engine    *gin.Engine
		apiKey    string
		contextID string
		adminJWT  string
	)

	BeforeEach(func() {
		engine, apiKey, contextID, adminJWT = setupMCPEngine()
	})

	AfterEach(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM api_keys")
		_, _ = pool.Exec(ctx, `DELETE FROM contexts WHERE id = $1`, contextID)
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE email LIKE 'swagger-admin-%@example.com'`)
	})

	createContact := func(key string, args map[string]any) string {
		w := mcpTool(engine, key, 1, "contacts_create", args)
		Expect(w.Code).To(Equal(http.StatusOK), "body: %s", w.Body.String())
		text := mcpToolText(w)
		var c struct {
			ID string `json:"ID"`
		}
		Expect(json.Unmarshal([]byte(text), &c)).To(Succeed(), "text: %s", text)
		Expect(c.ID).NotTo(BeEmpty())
		return c.ID
	}

	It("contacts_create then contacts_get round-trip", func() {
		email := "mcp-" + support.NewTestUUID() + "@example.com"
		id := createContact(apiKey, map[string]any{"email": email, "first_name": "MCP", "company": "Acme"})

		w := mcpTool(engine, apiKey, 2, "contacts_get", map[string]any{"contact_id": id})
		Expect(w.Code).To(Equal(http.StatusOK))
		text := mcpToolText(w)
		Expect(text).To(ContainSubstring("Acme"))
		Expect(text).To(ContainSubstring(email))
	})

	It("contacts_list returns paged shape", func() {
		createContact(apiKey, map[string]any{"email": "mcp-list-" + support.NewTestUUID() + "@example.com"})

		w := mcpTool(engine, apiKey, 2, "contacts_list", map[string]any{"page": 1, "limit": 20})
		Expect(w.Code).To(Equal(http.StatusOK))
		text := mcpToolText(w)
		Expect(text).To(ContainSubstring(`"items"`))
		Expect(text).To(ContainSubstring(`"total":1`))
		Expect(text).To(ContainSubstring(`"page":1`))
		Expect(text).To(ContainSubstring(`"limit":20`))
		Expect(text).To(ContainSubstring(`"has_more":false`))
	})

	It("contacts_update and contacts_delete", func() {
		id := createContact(apiKey, map[string]any{"email": "mcp-ud-" + support.NewTestUUID() + "@example.com"})

		w := mcpTool(engine, apiKey, 2, "contacts_update", map[string]any{"contact_id": id, "company": "Globe"})
		Expect(w.Code).To(Equal(http.StatusOK))
		Expect(mcpToolText(w)).To(ContainSubstring("Globe"))

		w = mcpTool(engine, apiKey, 3, "contacts_get", map[string]any{"contact_id": id})
		Expect(mcpToolText(w)).To(ContainSubstring("Globe"))

		w = mcpTool(engine, apiKey, 4, "contacts_delete", map[string]any{"contact_id": id})
		Expect(w.Code).To(Equal(http.StatusOK))
		Expect(mcpToolText(w)).To(ContainSubstring(`"deleted":true`))

		w = mcpTool(engine, apiKey, 5, "contacts_get", map[string]any{"contact_id": id})
		Expect(w.Code).To(Equal(http.StatusOK))
		isError, _ := mcpToolResult(w)
		Expect(isError).To(BeTrue())
	})

	It("contacts_merge keeps one contact and removes the other", func() {
		idA := createContact(apiKey, map[string]any{"email": "mcp-mg-a-" + support.NewTestUUID() + "@example.com"})
		idB := createContact(apiKey, map[string]any{"email": "mcp-mg-b-" + support.NewTestUUID() + "@example.com"})

		w := mcpTool(engine, apiKey, 3, "contacts_merge", map[string]any{
			"keep_contact_id": idA, "merge_contact_id": idB,
		})
		Expect(w.Code).To(Equal(http.StatusOK))
		Expect(mcpToolText(w)).To(ContainSubstring(`"merged":true`))

		w = mcpTool(engine, apiKey, 4, "contacts_get", map[string]any{"contact_id": idA})
		Expect(w.Code).To(Equal(http.StatusOK))
		_, text := mcpToolResult(w)
		Expect(text).NotTo(BeEmpty())

		w = mcpTool(engine, apiKey, 5, "contacts_get", map[string]any{"contact_id": idB})
		Expect(w.Code).To(Equal(http.StatusOK))
		isError, _ := mcpToolResult(w)
		Expect(isError).To(BeTrue())
	})

	It("contacts_activity returns items", func() {
		id := createContact(apiKey, map[string]any{"email": "mcp-act-" + support.NewTestUUID() + "@example.com"})

		w := mcpTool(engine, apiKey, 2, "contacts_activity", map[string]any{
			"contact_id": id, "page": 1, "limit": 10,
		})
		Expect(w.Code).To(Equal(http.StatusOK))
		Expect(mcpToolText(w)).To(ContainSubstring(`"items"`))
	})

	It("does not leak other context's contacts", func() {
		emailA := "mcp-lease-a-" + support.NewTestUUID() + "@example.com"
		emailB := "mcp-lease-b-" + support.NewTestUUID() + "@example.com"
		keyB, ctxB := mintAPIKey(engine, adminJWT, "mcp-b")
		DeferCleanup(func() {
			_, _ = pool.Exec(ctx, `DELETE FROM contexts WHERE id = $1`, ctxB)
		})

		createContact(apiKey, map[string]any{"email": emailA})
		createContact(keyB, map[string]any{"email": emailB})

		w := mcpTool(engine, apiKey, 3, "contacts_list", map[string]any{})
		Expect(w.Code).To(Equal(http.StatusOK))
		text := mcpToolText(w)
		Expect(text).To(ContainSubstring(emailA))
		Expect(text).ToNot(ContainSubstring(emailB))

		w = mcpTool(engine, keyB, 4, "contacts_list", map[string]any{})
		Expect(w.Code).To(Equal(http.StatusOK))
		text = mcpToolText(w)
		Expect(text).To(ContainSubstring(emailB))
		Expect(text).ToNot(ContainSubstring(emailA))
	})

	It("clamps limit and page", func() {
		w := mcpTool(engine, apiKey, 1, "contacts_list", map[string]any{"page": 0, "limit": 500})
		Expect(w.Code).To(Equal(http.StatusOK))
		text := mcpToolText(w)
		Expect(text).To(ContainSubstring(`"limit":100`))
		Expect(text).To(ContainSubstring(`"page":1`))
	})
})