package cicd_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"

	"github.com/gin-gonic/gin"
	"github.com/h2hsecure/automata/cicd/support"
	bhandler "github.com/h2hsecure/automata/internal/adapter/api/handler"
	ttracking "github.com/h2hsecure/automata/internal/adapter/tracking"
	crepo "github.com/h2hsecure/automata/internal/infrastructure/contact/repo"
	formRepo "github.com/h2hsecure/automata/internal/infrastructure/form/repo"
	"github.com/h2hsecure/automata/internal/infrastructure/queue"
	trepo "github.com/h2hsecure/automata/internal/infrastructure/tracking/repo"
	wh_svc "github.com/h2hsecure/automata/internal/infrastructure/webhook"
	wh_repo "github.com/h2hsecure/automata/internal/infrastructure/webhook/repo"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Form submission webhook delivery", func() {
	var (
		w         *httptest.ResponseRecorder
		engine    *gin.Engine
		mockServer *support.MockWebhookServer
		contextID string
		formID    string
	)

	BeforeEach(func() {
		gin.SetMode(gin.TestMode)
		engine = gin.New()
		mockServer = support.NewMockWebhookServer()

		// The shared suite runs only the core DB migrations; the webhook
		// endpoints table has its own runner (idempotent).
		Expect(wh_repo.RunMigrations(pool)).To(Succeed())

		contextID = support.NewTestUUID()
		formID = support.NewTestUUID()

		_, err := pool.Exec(ctx, `
			INSERT INTO contexts (id, slug, name, created_at, updated_at)
			VALUES ($1, $2, $3, NOW(), NOW())
		`, contextID, "wh-ctx-"+contextID, "Webhook Delivery Test Context")
		Expect(err).NotTo(HaveOccurred())

		_, err = pool.Exec(ctx, `
			INSERT INTO webhook_endpoints (id, context_id, name, url, events, active, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, true, NOW(), NOW())
		`, support.NewTestUUID(), contextID, "delivery-test", mockServer.URL(), "form.submitted")
		Expect(err).NotTo(HaveOccurred())

		fields, _ := json.Marshal([]map[string]interface{}{{"slug": "email", "type": "email", "required": true}})
		settings, _ := json.Marshal(map[string]interface{}{})
		_, err = pool.Exec(ctx, `
			INSERT INTO forms (id, context_id, slug, name, description, fields, settings, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, NOW(), NOW())
		`, formID, contextID, "wh-form", "Wh Form", "test form", fields, settings)
		Expect(err).NotTo(HaveOccurred())

		formHandler := bhandler.NewFormHandlerWithEmailAndWebhooks(
			formRepo.New(pool), crepo.New(pool), trepo.New(pool), nil,
			wh_svc.NewService(wh_repo.New(pool), queue.NewWebhookQueue(pool)),
		)
		forms := engine.Group("/form")
		forms.Use(ttracking.ContextMiddleware())
		forms.POST("/submit/:slug", formHandler.SubmitForm)

		w = httptest.NewRecorder()
	})

	AfterEach(func() {
		mockServer.Close()
		_, _ = pool.Exec(ctx, "DELETE FROM webhook_deliveries WHERE context_id = $1", contextID)
		_, _ = pool.Exec(ctx, "DELETE FROM webhook_endpoints WHERE context_id = $1", contextID)
		_, _ = pool.Exec(ctx, "DELETE FROM form_submissions WHERE context_id = $1", contextID)
		_, _ = pool.Exec(ctx, "DELETE FROM forms WHERE id = $1", formID)
	})

	It("enqueues a webhook delivery for the form.submitted event", func() {
		body := `{"data": {"email": "user@example.com"}}`
		req := httptest.NewRequest("POST", "/form/submit/wh-form", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Context-ID", contextID)
		engine.ServeHTTP(w, req)

		Expect(w.Code).To(Equal(http.StatusCreated), "body: %s", w.Body.String())

		var status, dbFormID string
		err := pool.QueryRow(ctx,
			"SELECT status, form_id FROM webhook_deliveries WHERE context_id = $1",
			contextID,
		).Scan(&status, &dbFormID)
		Expect(err).NotTo(HaveOccurred())
		Expect(status).To(Equal("pending"))
		Expect(dbFormID).To(Equal(formID))
	})
})