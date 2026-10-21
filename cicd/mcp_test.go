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
			"forms_list", "forms_get", "forms_create", "forms_update",
			"forms_delete", "forms_submit", "form_submissions_list", "form_submissions_get",
			"banners_list", "banners_get", "banners_create", "banners_update",
			"banners_delete",
			"placements_list", "placements_get", "placements_create", "placements_update",
			"placements_delete",
			"campaigns_list", "campaigns_get", "campaigns_create", "campaigns_update",
			"campaigns_delete",
			"tracking_dashboard", "tracking_events", "tracking_visitors",
			"tracking_visitor_events", "tracking_visitor_get",
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

var _ = Describe("MCP forms tools", func() {
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

	createForm := func(key string, slug string) (id, slugOut string) {
		w := mcpTool(engine, key, 1, "forms_create", map[string]any{
			"slug":        slug,
			"name":        "MCP Form",
			"description": "created by mcp spec",
			"fields": []map[string]any{
				{"slug": "email", "type": "email", "label": "Email"},
			},
		})
		Expect(w.Code).To(Equal(http.StatusOK), "body: %s", w.Body.String())
		text := mcpToolText(w)
		var f struct {
			ID   string `json:"ID"`
			Slug string `json:"Slug"`
		}
		Expect(json.Unmarshal([]byte(text), &f)).To(Succeed(), "text: %s", text)
		Expect(f.Slug).To(Equal(slug))
		Expect(f.ID).NotTo(BeEmpty())
		return f.ID, f.Slug
	}

	It("forms_create then forms_get (and forms_list contains it)", func() {
		slug := "mcp-form-" + support.NewTestUUID()
		_, gotSlug := createForm(apiKey, slug)

		w := mcpTool(engine, apiKey, 2, "forms_get", map[string]any{"form_id": gotSlug})
		Expect(w.Code).To(Equal(http.StatusOK))
		text := mcpToolText(w)
		Expect(text).To(ContainSubstring(slug))
		Expect(text).To(ContainSubstring("MCP Form"))

		w = mcpTool(engine, apiKey, 3, "forms_list", map[string]any{})
		Expect(w.Code).To(Equal(http.StatusOK))
		Expect(mcpToolText(w)).To(ContainSubstring(slug))
	})

	It("forms_update then forms_delete", func() {
		slug := "mcp-form-" + support.NewTestUUID()
		formID, _ := createForm(apiKey, slug)

		w := mcpTool(engine, apiKey, 2, "forms_update", map[string]any{
			"form_id": formID,
			"name":    "Renamed Form",
			"fields": []map[string]any{
				{"slug": "email", "type": "email", "label": "Email"},
			},
		})
		Expect(w.Code).To(Equal(http.StatusOK))
		Expect(mcpToolText(w)).To(ContainSubstring("Renamed Form"))

		w = mcpTool(engine, apiKey, 3, "forms_delete", map[string]any{"form_id": formID})
		Expect(w.Code).To(Equal(http.StatusOK))
		Expect(mcpToolText(w)).To(ContainSubstring(`"deleted":true`))

		_, text := mcpToolResult(mcpTool(engine, apiKey, 4, "forms_get", map[string]any{"form_id": slug}))
		Expect(text).NotTo(ContainSubstring(slug))
	})

	It("forms_submit then form_submissions_list", func() {
		slug := "mcp-form-" + support.NewTestUUID()
		formID, gotSlug := createForm(apiKey, slug)

		w := mcpTool(engine, apiKey, 2, "forms_submit", map[string]any{
			"form_id":     gotSlug,
			"data":        map[string]any{"email": "sub-" + support.NewTestUUID() + "@example.com"},
			"visitor_id":  "visitor-mcp-" + support.NewTestUUID(),
		})
		Expect(w.Code).To(Equal(http.StatusOK), "body: %s", w.Body.String())
		text := mcpToolText(w)
		var sub struct {
			ID string `json:"ID"`
		}
		Expect(json.Unmarshal([]byte(text), &sub)).To(Succeed(), "text: %s", text)
		Expect(sub.ID).NotTo(BeEmpty())

		w = mcpTool(engine, apiKey, 3, "form_submissions_list", map[string]any{"form_id": formID, "page": 1, "limit": 10})
		Expect(w.Code).To(Equal(http.StatusOK))
		listText := mcpToolText(w)
		Expect(listText).To(ContainSubstring(`"total":1`))
		Expect(listText).To(ContainSubstring(`"has_more":false`))
		Expect(listText).To(ContainSubstring(sub.ID))
	})

	It("form_submissions_get", func() {
		slug := "mcp-form-" + support.NewTestUUID()
		_, gotSlug := createForm(apiKey, slug)

		w := mcpTool(engine, apiKey, 2, "forms_submit", map[string]any{
			"form_id": gotSlug,
			"data":    map[string]any{"email": "sub2-" + support.NewTestUUID() + "@example.com"},
		})
		Expect(w.Code).To(Equal(http.StatusOK), "body: %s", w.Body.String())
		var sub struct {
			ID string `json:"ID"`
		}
		Expect(json.Unmarshal([]byte(mcpToolText(w)), &sub)).To(Succeed())

		w = mcpTool(engine, apiKey, 3, "form_submissions_get", map[string]any{"submission_id": sub.ID})
		Expect(w.Code).To(Equal(http.StatusOK))
		Expect(mcpToolText(w)).To(ContainSubstring(sub.ID))
	})
})

var _ = Describe("MCP banner/placement/campaign tools", func() {
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

	It("banners create/update/list/delete", func() {
		// campaign first (banners can reference it)
		w := mcpTool(engine, apiKey, 1, "campaigns_create", map[string]any{"name": "MCP Camp"})
		Expect(w.Code).To(Equal(http.StatusOK), "body: %s", w.Body.String())
		var camp struct {
			ID string `json:"ID"`
		}
		Expect(json.Unmarshal([]byte(mcpToolText(w)), &camp)).To(Succeed())
		Expect(camp.ID).NotTo(BeEmpty())

		w = mcpTool(engine, apiKey, 2, "campaigns_list", map[string]any{})
		Expect(w.Code).To(Equal(http.StatusOK))
		Expect(mcpToolText(w)).To(ContainSubstring("MCP Camp"))

		w = mcpTool(engine, apiKey, 3, "banners_create", map[string]any{
			"name":        "MCP Banner",
			"type":        "image",
			"campaign_id": camp.ID,
		})
		Expect(w.Code).To(Equal(http.StatusOK), "body: %s", w.Body.String())
		var ban struct {
			ID string `json:"ID"`
		}
		Expect(json.Unmarshal([]byte(mcpToolText(w)), &ban)).To(Succeed())
		Expect(ban.ID).NotTo(BeEmpty())

		w = mcpTool(engine, apiKey, 4, "banners_list", map[string]any{"limit": 100})
		Expect(w.Code).To(Equal(http.StatusOK))
		text := mcpToolText(w)
		Expect(text).To(ContainSubstring("MCP Banner"))
		Expect(text).To(ContainSubstring(`"total":1`))

		w = mcpTool(engine, apiKey, 5, "banners_update", map[string]any{"banner_id": ban.ID, "name": "Renamed Banner"})
		Expect(w.Code).To(Equal(http.StatusOK), "body: %s", w.Body.String())

		w = mcpTool(engine, apiKey, 6, "banners_get", map[string]any{"banner_id": ban.ID})
		Expect(w.Code).To(Equal(http.StatusOK))
		Expect(mcpToolText(w)).To(ContainSubstring("Renamed Banner"))

		w = mcpTool(engine, apiKey, 7, "banners_delete", map[string]any{"banner_id": ban.ID})
		Expect(w.Code).To(Equal(http.StatusOK))

		_, text = mcpToolResult(mcpTool(engine, apiKey, 8, "banners_get", map[string]any{"banner_id": ban.ID}))
		Expect(text).NotTo(ContainSubstring(ban.ID))
	})

	It("placements create/list/delete", func() {
		w := mcpTool(engine, apiKey, 1, "placements_create", map[string]any{
			"name":         "P1",
			"css_selector": ".slot",
		})
		Expect(w.Code).To(Equal(http.StatusOK), "body: %s", w.Body.String())
		var pl struct {
			ID string `json:"ID"`
		}
		Expect(json.Unmarshal([]byte(mcpToolText(w)), &pl)).To(Succeed())
		Expect(pl.ID).NotTo(BeEmpty())

		w = mcpTool(engine, apiKey, 2, "placements_list", map[string]any{})
		Expect(w.Code).To(Equal(http.StatusOK))
		Expect(mcpToolText(w)).To(ContainSubstring("P1"))

		w = mcpTool(engine, apiKey, 3, "placements_delete", map[string]any{"placement_id": pl.ID})
		Expect(w.Code).To(Equal(http.StatusOK))

		w = mcpTool(engine, apiKey, 4, "placements_list", map[string]any{})
		Expect(mcpToolText(w)).NotTo(ContainSubstring("P1"))
	})

	It("campaigns get/update/delete", func() {
		w := mcpTool(engine, apiKey, 1, "campaigns_create", map[string]any{"name": "MCP Camp2"})
		Expect(w.Code).To(Equal(http.StatusOK), "body: %s", w.Body.String())
		var camp struct {
			ID string `json:"ID"`
		}
		Expect(json.Unmarshal([]byte(mcpToolText(w)), &camp)).To(Succeed())

		w = mcpTool(engine, apiKey, 2, "campaigns_get", map[string]any{"campaign_id": camp.ID})
		Expect(w.Code).To(Equal(http.StatusOK))
		Expect(mcpToolText(w)).To(ContainSubstring("MCP Camp2"))

		w = mcpTool(engine, apiKey, 3, "campaigns_update", map[string]any{"campaign_id": camp.ID, "name": "MCP Camp2 Renamed"})
		Expect(w.Code).To(Equal(http.StatusOK), "body: %s", w.Body.String())

		w = mcpTool(engine, apiKey, 4, "campaigns_get", map[string]any{"campaign_id": camp.ID})
		Expect(mcpToolText(w)).To(ContainSubstring("MCP Camp2 Renamed"))

		w = mcpTool(engine, apiKey, 5, "campaigns_delete", map[string]any{"campaign_id": camp.ID})
		Expect(w.Code).To(Equal(http.StatusOK))

		_, text := mcpToolResult(mcpTool(engine, apiKey, 6, "campaigns_get", map[string]any{"campaign_id": camp.ID}))
		Expect(text).NotTo(ContainSubstring(camp.ID))
	})
})

var _ = Describe("MCP tracking tools", func() {
	var (
		engine    *gin.Engine
		apiKey    string
		contextID string
	)

	seedVisitor := func() string {
		v := support.NewTestVisitor(contextID)
		_, err := pool.Exec(ctx, `
			INSERT INTO tracking_visitors (id, context_id, cookie_value, fingerprint, first_seen, last_seen, page_views)
			VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			v["id"], v["context_id"], v["cookie_value"], v["fingerprint"], v["first_seen"], v["last_seen"], v["page_views"])
		Expect(err).NotTo(HaveOccurred())
		return v["id"].(string)
	}

	cookieOf := func(visitorID string) string {
		var cookie string
		Expect(pool.QueryRow(ctx, "SELECT cookie_value FROM tracking_visitors WHERE id = $1", visitorID).Scan(&cookie)).To(Succeed())
		return cookie
	}

	// tracking_events.visitor_id stores the cookie value, not the visitor UUID.
	seedEvent := func(cookieValue string) {
		e := support.NewTestEvent(contextID, cookieValue)
		_, err := pool.Exec(ctx, `
			INSERT INTO tracking_events (id, context_id, visitor_id, type, url, title, referrer, event_name, properties, user_agent, ip_hash, utm_source, utm_medium, utm_campaign, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, '{}'::jsonb, $9, $10, $11, $12, $13, NOW())`,
			e["id"], e["context_id"], cookieValue, e["type"], e["url"], e["title"], e["referrer"], e["event_name"], e["user_agent"], e["ip_hash"], e["utm_source"], e["utm_medium"], e["utm_campaign"])
		Expect(err).NotTo(HaveOccurred())
	}

	BeforeEach(func() {
		engine, apiKey, contextID, _ = setupMCPEngine()
	})

	AfterEach(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM tracking_events WHERE context_id = $1", contextID)
		_, _ = pool.Exec(ctx, "DELETE FROM tracking_visitors WHERE context_id = $1", contextID)
		_, _ = pool.Exec(ctx, "DELETE FROM api_keys")
		_, _ = pool.Exec(ctx, `DELETE FROM contexts WHERE id = $1`, contextID)
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE email LIKE 'swagger-admin-%@example.com'`)
	})

	It("tracking_dashboard returns metrics", func() {
		vid := seedVisitor()
		seedEvent(cookieOf(vid))

		w := mcpTool(engine, apiKey, 1, "tracking_dashboard", map[string]any{"top_n": 5})
		Expect(w.Code).To(Equal(http.StatusOK), "body: %s", w.Body.String())
		text := mcpToolText(w)
		Expect(text).To(ContainSubstring(`"PageViews"`))
		Expect(text).To(ContainSubstring(`"TotalVisitors"`))
	})

	It("tracking_events and tracking_visitors are paged", func() {
		vid := seedVisitor()
		cookie := cookieOf(vid)
		seedEvent(cookie)
		seedEvent(cookie)
		seedEvent(cookie)

		w := mcpTool(engine, apiKey, 1, "tracking_events", map[string]any{"limit": 2})
		Expect(w.Code).To(Equal(http.StatusOK), "body: %s", w.Body.String())
		text := mcpToolText(w)
		Expect(text).To(ContainSubstring(`"total":3`))
		Expect(text).To(ContainSubstring(`"has_more":true`))

		w = mcpTool(engine, apiKey, 2, "tracking_visitors", map[string]any{"limit": 10})
		Expect(w.Code).To(Equal(http.StatusOK), "body: %s", w.Body.String())
		Expect(mcpToolText(w)).To(ContainSubstring(vid))
	})

	It("tracking_visitor_events and tracking_visitor_get", func() {
		vid := seedVisitor()
		cookie := cookieOf(vid)
		seedEvent(cookie)
		seedEvent(cookie)

		w := mcpTool(engine, apiKey, 1, "tracking_visitor_events", map[string]any{"visitor_id": vid, "limit": 10})
		Expect(w.Code).To(Equal(http.StatusOK), "body: %s", w.Body.String())
		Expect(mcpToolText(w)).To(ContainSubstring(`"total":2`))

		w = mcpTool(engine, apiKey, 2, "tracking_visitor_get", map[string]any{"visitor_id": vid})
		Expect(w.Code).To(Equal(http.StatusOK), "body: %s", w.Body.String())
		Expect(mcpToolText(w)).To(ContainSubstring(vid))
	})
})