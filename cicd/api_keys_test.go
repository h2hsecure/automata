package cicd_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"

	"github.com/gin-gonic/gin"
	"github.com/h2hsecure/automata/cicd/support"
	infraauth "github.com/h2hsecure/automata/internal/infrastructure/auth"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("API key listing (admin-global, Website column)", func() {
	var (
		engine   *gin.Engine
		adminJWT string
	)

	// mintKeyInContext creates a context with the given name and mints one
	// API key for it through the production admin endpoint.
	mintKeyInContext := func(prefix, contextName string) (contextID, keyName string) {
		contextID = support.NewTestUUID()
		_, err := pool.Exec(ctx, `
			INSERT INTO contexts (id, slug, name, created_at, updated_at)
			VALUES ($1, $2, $3, NOW(), NOW())
		`, contextID, prefix+"-ctx-"+contextID, contextName)
		Expect(err).NotTo(HaveOccurred())

		keyName = prefix + "-key-" + support.NewTestUUID()
		req := httptest.NewRequest("POST", "/api/admin/api-keys", bytes.NewBufferString(`{"name":"`+keyName+`"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+adminJWT)
		req.Header.Set("X-Context-ID", contextID)
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, req)
		Expect(w.Code).To(Equal(http.StatusCreated), "create api key: %s", w.Body.String())
		return
	}

	BeforeEach(func() {
		infraauth.InitForTesting("test-secret-key-for-jwt-signing-32bytes", "test-cookie-secret-32bytes-xxxx!")
		engine = setupCreationServer()
		adminJWT = loginAsAdmin(engine)
	})

	AfterEach(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE email LIKE 'swagger-admin-%@example.com'`)
	})

	// The listing must be admin-global: one request returns the keys of
	// every website, each labeled with its context name (the WebUI's
	// Website column). No X-Context-ID header is sent — the same way the
	// WebUI calls the endpoint.
	It("lists keys from all contexts with their context names", func() {
		_, keyAName := mintKeyInContext("apikey-a", "Website A")
		_, keyBName := mintKeyInContext("apikey-b", "Website B")

		req := httptest.NewRequest("GET", "/api/admin/api-keys", nil)
		req.Header.Set("Authorization", "Bearer "+adminJWT)
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, req)

		Expect(w.Code).To(Equal(http.StatusOK), "body: %s", w.Body.String())
		var resp struct {
			Keys []struct {
				Name        string `json:"name"`
				ContextName string `json:"context_name"`
			} `json:"keys"`
		}
		Expect(json.Unmarshal(w.Body.Bytes(), &resp)).To(Succeed())
		Expect(resp.Keys).To(HaveLen(2))

		byName := map[string]string{}
		for _, k := range resp.Keys {
			byName[k.Name] = k.ContextName
		}
		Expect(byName[keyAName]).To(Equal("Website A"))
		Expect(byName[keyBName]).To(Equal("Website B"))
	})

	// Revocation must work cross-context: the WebUI lists all websites'
	// keys in one table, so a key from a website other than the selected
	// one must be revokable.
	It("revokes a key from another context (X-Context-ID mismatch)", func() {
		ctxA, _ := mintKeyInContext("apikey-a", "Website A")
		_, keyBName := mintKeyInContext("apikey-b", "Website B")

		// Get the DB id of keyB via the global listing.
		req := httptest.NewRequest("GET", "/api/admin/api-keys", nil)
		req.Header.Set("Authorization", "Bearer "+adminJWT)
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, req)
		Expect(w.Code).To(Equal(http.StatusOK), "body: %s", w.Body.String())

		var resp struct {
			Keys []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"keys"`
		}
		Expect(json.Unmarshal(w.Body.Bytes(), &resp)).To(Succeed())
		var keyBID string
		for _, k := range resp.Keys {
			if k.Name == keyBName {
				keyBID = k.ID
			}
		}
		Expect(keyBID).NotTo(BeEmpty())

		// Revoke keyB while the selected context is ctxA (Website A).
		del := httptest.NewRequest("DELETE", "/api/admin/api-keys/"+keyBID, nil)
		del.Header.Set("Authorization", "Bearer "+adminJWT)
		del.Header.Set("X-Context-ID", ctxA)
		wd := httptest.NewRecorder()
		engine.ServeHTTP(wd, del)
		Expect(wd.Code).To(Equal(http.StatusOK), "body: %s", wd.Body.String())
	})
})