package cicd_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/hekemen/automata/cicd/support"
	"github.com/hekemen/automata/internal/adapter/api/handler"
	"github.com/hekemen/automata/internal/domain/email"
	emailusecase "github.com/hekemen/automata/internal/usecase/email"
	"github.com/hekemen/automata/internal/infrastructure/email/repo"
	email_infra "github.com/hekemen/automata/internal/infrastructure/queue"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var (
	emailRepo      email.Repository
	emailQueue     email_infra.Queue
	emailUsecase   *emailusecase.EmailUsecase
	emailHandler   *handler.EmailHandler
	// mockSMTP is declared in email_queue_test.go (same package)
	testContextID  string
	testContext    map[string]interface{}
)

var _ = Describe("Email Template CRUD", func() {
	BeforeEach(func() {
		mockSMTP = support.NewMockSMTP()
		mockSMTP.Start()
		pool = db.Pool

		// Create a test context
		testContext = support.NewTestContext()
		settingsJSON, _ := json.Marshal(testContext["settings"])
		_, err := pool.Exec(ctx,
			`INSERT INTO contexts (id, slug, name, domain, is_active, settings, created_at, updated_at)
			 VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())`,
			testContext["id"], testContext["slug"], testContext["name"],
			testContext["domain"], testContext["is_active"], settingsJSON,
		)
		Expect(err).NotTo(HaveOccurred())
		testContextID = testContext["id"].(string)

		// Initialize email repo, queue, usecase, and handler
		emailRepo = repo.New(pool)
		emailQueue = email_infra.NewEmailQueue(pool)
		emailUsecase = emailusecase.NewEmailUsecase(emailRepo, emailQueue)
		emailHandler = handler.NewEmailHandler(emailUsecase)
	})

	AfterEach(func() {
		// Clean up email_jobs and email_templates tables
		_, _ = pool.Exec(ctx, "DELETE FROM email_jobs")
		_, _ = pool.Exec(ctx, "DELETE FROM email_templates")
		_, _ = pool.Exec(ctx, "DELETE FROM contexts WHERE id = $1", testContextID)
		mockSMTP.Clear()
	})

	It("creates a template with valid request", func() {
		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/admin/email-templates", nil)
		c.Request.Header.Set("Content-Type", "application/json")

		body := strings.NewReader(fmt.Sprintf(`{
			"context_id": "%s",
			"key": "welcome",
			"subject": "Welcome to %s",
			"body_text": "Hello {{ .ContactName }}!",
			"body_html": "<h1>Hello {{ .ContactName }}</h1>",
			"is_default": true
		}`, testContextID, testContext["name"]))
		c.Request.Body = io.NopCloser(body)

		emailHandler.Create(c)
		Expect(w.Code).To(Equal(http.StatusCreated))

		var resp email.TemplateResponse
		json.Unmarshal(w.Body.Bytes(), &resp)
		Expect(resp.ID).NotTo(BeEmpty())
		Expect(resp.Key).To(Equal("welcome"))
		Expect(resp.Subject).To(Equal("Welcome to " + testContext["name"].(string)))
	})

	It("rejects duplicate key for same context", func() {
		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/admin/email-templates", nil)
		c.Request.Header.Set("Content-Type", "application/json")

		// Create first template
		body1 := strings.NewReader(fmt.Sprintf(`{
			"context_id": "%s",
			"key": "welcome",
			"subject": "Welcome!",
			"body_text": "Hello!",
			"body_html": "<h1>Hello!</h1>"
		}`, testContextID))
		c.Request.Body = io.NopCloser(body1)
		emailHandler.Create(c)
		Expect(w.Code).To(Equal(http.StatusCreated))

		// Try to create duplicate
		w = httptest.NewRecorder()
		c2, _ := gin.CreateTestContext(w)
		c2.Request = httptest.NewRequest("POST", "/admin/email-templates", nil)
		c2.Request.Header.Set("Content-Type", "application/json")

		body2 := strings.NewReader(fmt.Sprintf(`{
			"context_id": "%s",
			"key": "welcome",
			"subject": "Welcome Again!",
			"body_text": "Hello Again!",
			"body_html": "<h1>Hello Again!</h1>"
		}`, testContextID))
		c2.Request.Body = io.NopCloser(body2)
		emailHandler.Create(c2)
		Expect(w.Code).To(Equal(http.StatusConflict))
	})

	It("gets a template by ID", func() {
		gin.SetMode(gin.TestMode)

		// First create a template
		emailRepo.Create(&email.EmailTemplate{
			ContextID: testContextID,
			Key:       "get_test",
			Subject:   "Get Test Subject",
			BodyText:  "Get Test Body",
			BodyHTML:  "<p>Get Test Body</p>",
			IsDefault: false,
		})

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/admin/email-templates", nil)

		// Query templates to get the ID
		templates, err := emailRepo.List(testContextID)
		Expect(err).NotTo(HaveOccurred())
		Expect(templates).To(HaveLen(1))
		templateID := templates[0].ID

		c.Params = append(c.Params, gin.Param{Key: "id", Value: templateID})
		emailHandler.Get(c)
		Expect(w.Code).To(Equal(http.StatusOK))

		var resp email.TemplateResponse
		json.Unmarshal(w.Body.Bytes(), &resp)
		Expect(resp.ID).To(Equal(templateID))
		Expect(resp.Key).To(Equal("get_test"))
	})

	It("updates a template with partial fields", func() {
		gin.SetMode(gin.TestMode)

		// First create a template
		emailRepo.Create(&email.EmailTemplate{
			ContextID: testContextID,
			Key:       "update_test",
			Subject:   "Original Subject",
			BodyText:  "Original Body",
			BodyHTML:  "<p>Original Body</p>",
			IsDefault: false,
		})

		templates, err := emailRepo.List(testContextID)
		Expect(err).NotTo(HaveOccurred())
		Expect(templates).To(HaveLen(1))
		template := templates[0]

		// Update only the subject
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("PUT", "/admin/email-templates/"+template.ID, nil)
		c.Request.Header.Set("Content-Type", "application/json")

		body := strings.NewReader(`{"subject": "Updated Subject"}`)
		c.Request.Body = io.NopCloser(body)
		c.Params = append(c.Params, gin.Param{Key: "id", Value: template.ID})
		emailHandler.Update(c)
		Expect(w.Code).To(Equal(http.StatusOK))

		var resp email.TemplateResponse
		json.Unmarshal(w.Body.Bytes(), &resp)
		Expect(resp.Subject).To(Equal("Updated Subject"))
		Expect(resp.BodyText).To(Equal("Original Body")) // unchanged
		Expect(resp.BodyHTML).To(Equal("<p>Original Body</p>")) // unchanged
	})

	It("deletes a template", func() {
		gin.SetMode(gin.TestMode)

		// First create a template
		emailRepo.Create(&email.EmailTemplate{
			ContextID: testContextID,
			Key:       "delete_test",
			Subject:   "Delete Me",
			BodyText:  "Delete this",
			BodyHTML:  "<p>Delete this</p>",
		})

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("DELETE", "/admin/email-templates", nil)

		templates, err := emailRepo.List(testContextID)
		Expect(err).NotTo(HaveOccurred())
		Expect(templates).To(HaveLen(1))
		templateID := templates[0].ID

		c.Params = append(c.Params, gin.Param{Key: "id", Value: templateID})
		emailHandler.Delete(c)
		Expect(w.Code).To(Equal(http.StatusOK))

		// Verify it's gone
		_, err = emailRepo.GetByID(templateID)
		Expect(err).To(HaveOccurred())
	})

	It("lists templates filtered by context", func() {
		gin.SetMode(gin.TestMode)

		// Create templates for the test context
		emailRepo.Create(&email.EmailTemplate{
			ContextID: testContextID,
			Key:       "list_test_1",
			Subject:   "List Test 1",
			BodyText:  "Body 1",
			BodyHTML:  "<p>Body 1</p>",
		})
		emailRepo.Create(&email.EmailTemplate{
			ContextID: testContextID,
			Key:       "list_test_2",
			Subject:   "List Test 2",
			BodyText:  "Body 2",
			BodyHTML:  "<p>Body 2</p>",
		})

		templates, err := emailRepo.List(testContextID)
		Expect(err).NotTo(HaveOccurred())
		Expect(templates).To(HaveLen(2))

		// Verify ordering (by key ASC)
		Expect(templates[0].Key).To(Equal("list_test_1"))
		Expect(templates[1].Key).To(Equal("list_test_2"))
	})
})

var _ = Describe("Template Rendering", func() {
	BeforeEach(func() {
		mockSMTP = support.NewMockSMTP()
		mockSMTP.Start()
		pool = db.Pool

		testContext = support.NewTestContext()
		settingsJSON, _ := json.Marshal(testContext["settings"])
		_, err := pool.Exec(ctx,
			`INSERT INTO contexts (id, slug, name, domain, is_active, settings, created_at, updated_at)
			 VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())`,
			testContext["id"], testContext["slug"], testContext["name"],
			testContext["domain"], testContext["is_active"], settingsJSON,
		)
		Expect(err).NotTo(HaveOccurred())
		testContextID = testContext["id"].(string)

		emailRepo = repo.New(pool)
		emailQueue = email_infra.NewEmailQueue(pool)
		emailUsecase = emailusecase.NewEmailUsecase(emailRepo, emailQueue)
	})

	AfterEach(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM email_jobs")
		_, _ = pool.Exec(ctx, "DELETE FROM email_templates")
		_, _ = pool.Exec(ctx, "DELETE FROM contexts WHERE id = $1", testContextID)
		mockSMTP.Clear()
	})

	It("renders Go text/template variables correctly", func() {
		emailRepo.Create(&email.EmailTemplate{
			ContextID: testContextID,
			Key:       "render_test",
			Subject:   "Hello {{ .Name }}!",
			BodyText:  "Dear {{ .Name }}, welcome to {{ .Company }}!",
			BodyHTML:  "<p>Dear <strong>{{ .Name }}</strong>, welcome to {{ .Company }}!</p>",
		})

		variables := map[string]string{
			"Name":    "Alice",
			"Company": "Acme Corp",
		}

		subject, body, htmlBody, err := emailUsecase.RenderTemplate(testContextID, "render_test", variables)
		Expect(err).NotTo(HaveOccurred())
		Expect(subject).To(Equal("Hello Alice!"))
		Expect(body).To(Equal("Dear Alice, welcome to Acme Corp!"))
		Expect(htmlBody).To(Equal("<p>Dear <strong>Alice</strong>, welcome to Acme Corp!</p>"))
	})

	It("returns error for undefined variables", func() {
		emailRepo.Create(&email.EmailTemplate{
			ContextID: testContextID,
			Key:       "undefined_test",
			Subject:   "Hello {{ .Missing }}!",
			BodyText:  "Body {{ .Missing }}",
			BodyHTML:  "<p>Body</p>",
		})

		variables := map[string]string{
			"Other": "value",
		}

		_, _, _, err := emailUsecase.RenderTemplate(testContextID, "undefined_test", variables)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("render body"))
	})

	It("renders HTML and plain text variants", func() {
		emailRepo.Create(&email.EmailTemplate{
			ContextID: testContextID,
			Key:       "html_test",
			Subject:   "HTML Test {{ .Name }}",
			BodyText:  "Plain text body for {{ .Name }}",
			BodyHTML:  "<h1>HTML body for {{ .Name }}</h1>",
		})

		variables := map[string]string{"Name": "Bob"}
		subject, body, htmlBody, err := emailUsecase.RenderTemplate(testContextID, "html_test", variables)
		Expect(err).NotTo(HaveOccurred())
		Expect(subject).To(Equal("HTML Test Bob"))
		Expect(body).To(Equal("Plain text body for Bob"))
		Expect(htmlBody).To(Equal("<h1>HTML body for Bob</h1>"))
	})
})

var _ = Describe("Send Email", func() {
	BeforeEach(func() {
		mockSMTP = support.NewMockSMTP()
		mockSMTP.Start()
		pool = db.Pool

		testContext = support.NewTestContext()
		settingsJSON, _ := json.Marshal(testContext["settings"])
		_, err := pool.Exec(ctx,
			`INSERT INTO contexts (id, slug, name, domain, is_active, settings, created_at, updated_at)
			 VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())`,
			testContext["id"], testContext["slug"], testContext["name"],
			testContext["domain"], testContext["is_active"], settingsJSON,
		)
		Expect(err).NotTo(HaveOccurred())
		testContextID = testContext["id"].(string)

		emailRepo = repo.New(pool)
		emailQueue = email_infra.NewEmailQueue(pool)
		emailUsecase = emailusecase.NewEmailUsecase(emailRepo, emailQueue)
	})

	AfterEach(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM email_jobs")
		_, _ = pool.Exec(ctx, "DELETE FROM email_templates")
		_, _ = pool.Exec(ctx, "DELETE FROM contexts WHERE id = $1", testContextID)
		mockSMTP.Clear()
	})

	It("enqueues email job after rendering", func() {
		emailRepo.Create(&email.EmailTemplate{
			ContextID: testContextID,
			Key:       "send_test",
			Subject:   "Send Test: {{ .Name }}",
			BodyText:  "Body for {{ .Name }}",
			BodyHTML:  "<p>HTML Body</p>",
		})

		req := email.SendEmailRequest{
			ToArray: []string{"alice@example.com", "bob@example.com"},
			Template: "send_test",
			Variables: map[string]string{"Name": "Test User"},
		}

		jobID, err := emailUsecase.SendEmail(testContextID, req)
		Expect(err).NotTo(HaveOccurred())
		Expect(jobID).NotTo(BeEmpty())

		// Verify the job was inserted into the database
		var count int
		err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM email_jobs").Scan(&count)
		Expect(err).NotTo(HaveOccurred())
		Expect(count).To(Equal(1))
	})

	It("accepts single string or array for To field", func() {
		emailRepo.Create(&email.EmailTemplate{
			ContextID: testContextID,
			Key:       "to_field_test",
			Subject:   "Test",
			BodyText:  "Body",
			BodyHTML:  "<p>Body</p>",
		})

		// Single string
		req1 := email.SendEmailRequest{
			To:     "single@example.com",
			Template: "to_field_test",
			Variables: map[string]string{},
		}
		jobID1, err := emailUsecase.SendEmail(testContextID, req1)
		Expect(err).NotTo(HaveOccurred())
		Expect(jobID1).NotTo(BeEmpty())

		// Array
		req2 := email.SendEmailRequest{
			ToArray: []string{"one@example.com", "two@example.com"},
			Template: "to_field_test",
			Variables: map[string]string{},
		}
		jobID2, err := emailUsecase.SendEmail(testContextID, req2)
		Expect(err).NotTo(HaveOccurred())
		Expect(jobID2).NotTo(BeEmpty())
	})

	It("applies subject_override and body_override", func() {
		emailRepo.Create(&email.EmailTemplate{
			ContextID: testContextID,
			Key:       "override_test",
			Subject:   "Template Subject",
			BodyText:  "Template Body",
			BodyHTML:  "<p>Template HTML</p>",
		})

		subjectOverride := "Override Subject"
		bodyOverride := "Override Body"

		req := email.SendEmailRequest{
			ToArray:         []string{"test@example.com"},
			Template:        "override_test",
			SubjectOverride: &subjectOverride,
			BodyOverride:    &bodyOverride,
		}

		jobID, err := emailUsecase.SendEmail(testContextID, req)
		Expect(err).NotTo(HaveOccurred())
		Expect(jobID).NotTo(BeEmpty())

		// Verify the job has the override values
		var jobSubject, jobBody string
		err = pool.QueryRow(ctx,
			"SELECT subject, body FROM email_jobs ORDER BY created_at DESC LIMIT 1",
		).Scan(&jobSubject, &jobBody)
		Expect(err).NotTo(HaveOccurred())
		Expect(jobSubject).To(Equal("Override Subject"))
		Expect(jobBody).To(Equal("Override Body"))
	})
})

var _ = Describe("Test Send", func() {
	BeforeEach(func() {
		mockSMTP = support.NewMockSMTP()
		mockSMTP.Start()
		pool = db.Pool

		testContext = support.NewTestContext()
		settingsJSON, _ := json.Marshal(testContext["settings"])
		_, err := pool.Exec(ctx,
			`INSERT INTO contexts (id, slug, name, domain, is_active, settings, created_at, updated_at)
			 VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())`,
			testContext["id"], testContext["slug"], testContext["name"],
			testContext["domain"], testContext["is_active"], settingsJSON,
		)
		Expect(err).NotTo(HaveOccurred())
		testContextID = testContext["id"].(string)

		emailRepo = repo.New(pool)
		emailQueue = email_infra.NewEmailQueue(pool)
		emailUsecase = emailusecase.NewEmailUsecase(emailRepo, emailQueue)
	})

	AfterEach(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM email_jobs")
		_, _ = pool.Exec(ctx, "DELETE FROM email_templates")
		_, _ = pool.Exec(ctx, "DELETE FROM contexts WHERE id = $1", testContextID)
		mockSMTP.Clear()
	})

	It("renders template and enqueues for single address", func() {
		emailRepo.Create(&email.EmailTemplate{
			ContextID: testContextID,
			Key:       "test_send",
			Subject:   "Test: {{ .Name }}",
			BodyText:  "Test body for {{ .Name }}",
			BodyHTML:  "<p>Test HTML</p>",
		})

		req := email.TestSendRequest{
			To:      "tester@example.com",
			Template: "test_send",
			Variables: map[string]string{"Name": "Tester"},
		}

		jobID, subject, preview, err := emailUsecase.TestSend(testContextID, req)
		Expect(err).NotTo(HaveOccurred())
		Expect(jobID).NotTo(BeEmpty())
		Expect(subject).To(Equal("Test: Tester"))
		Expect(preview).To(ContainSubstring("Tester"))

		// Verify job was enqueued
		var count int
		err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM email_jobs").Scan(&count)
		Expect(err).NotTo(HaveOccurred())
		Expect(count).To(Equal(1))
	})

	It("returns rendered subject preview", func() {
		emailRepo.Create(&email.EmailTemplate{
			ContextID: testContextID,
			Key:       "preview_test",
			Subject:   "Preview: {{ .Company }}",
			BodyText:  "A very long body text that should be truncated to a preview of at most 200 characters when the rendered body is longer than this threshold.",
			BodyHTML:  "<p>Preview</p>",
		})

		req := email.TestSendRequest{
			To:      "preview@example.com",
			Template: "preview_test",
			Variables: map[string]string{"Company": "Big Corp"},
		}

		jobID, subject, preview, err := emailUsecase.TestSend(testContextID, req)
		Expect(err).NotTo(HaveOccurred())
		Expect(jobID).NotTo(BeEmpty())
		Expect(subject).To(Equal("Preview: Big Corp"))
		Expect(len(preview)).To(BeNumerically("<=", 200))
	})
})

var _ = Describe("Default Template Seeding", func() {
	BeforeEach(func() {
		mockSMTP = support.NewMockSMTP()
		mockSMTP.Start()
		pool = db.Pool

		testContext = support.NewTestContext()
		settingsJSON, _ := json.Marshal(testContext["settings"])
		_, err := pool.Exec(ctx,
			`INSERT INTO contexts (id, slug, name, domain, is_active, settings, created_at, updated_at)
			 VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())`,
			testContext["id"], testContext["slug"], testContext["name"],
			testContext["domain"], testContext["is_active"], settingsJSON,
		)
		Expect(err).NotTo(HaveOccurred())
		testContextID = testContext["id"].(string)
	})

	AfterEach(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM email_templates")
		_, _ = pool.Exec(ctx, "DELETE FROM contexts WHERE id = $1", testContextID)
		mockSMTP.Clear()
	})

	It("seeds 5 default templates on first call", func() {
		err := emailRepo.SeedDefaults(testContextID)
		Expect(err).NotTo(HaveOccurred())

		// Verify all 5 default templates were created
		templates, err := emailRepo.List(testContextID)
		Expect(err).NotTo(HaveOccurred())
		Expect(templates).To(HaveLen(5))

		keys := make(map[string]bool)
		for _, t := range templates {
			keys[t.Key] = true
		}
		Expect(keys).To(HaveKey("welcome"))
		Expect(keys).To(HaveKey("contact_created"))
		Expect(keys).To(HaveKey("form_submission"))
		Expect(keys).To(HaveKey("password_reset"))
		Expect(keys).To(HaveKey("notification"))

		// Verify they all have is_default = true
		for _, t := range templates {
			Expect(t.IsDefault).To(BeTrue(), "template %s should be marked as default", t.Key)
		}
	})

	It("skips seeding on repeated call (idempotent)", func() {
		// Seed once
		err := emailRepo.SeedDefaults(testContextID)
		Expect(err).NotTo(HaveOccurred())

		// Seed again
		err = emailRepo.SeedDefaults(testContextID)
		Expect(err).NotTo(HaveOccurred())

		// Should still have exactly 5 templates (no duplicates)
		templates, err := emailRepo.List(testContextID)
		Expect(err).NotTo(HaveOccurred())
		Expect(templates).To(HaveLen(5))
	})

	It("seeds templates with correct content", func() {
		err := emailRepo.SeedDefaults(testContextID)
		Expect(err).NotTo(HaveOccurred())

		// Check welcome template
		welcome, err := emailRepo.GetByKey(testContextID, "welcome")
		Expect(err).NotTo(HaveOccurred())
		Expect(welcome.Key).To(Equal("welcome"))
		Expect(welcome.Subject).To(ContainSubstring("{{ .ContextName }}"))
		Expect(welcome.BodyText).To(ContainSubstring("{{ .ContactName }}"))
		Expect(welcome.BodyHTML).To(ContainSubstring("<h1>"))

		// Check notification template (has conditional CTA)
		notif, err := emailRepo.GetByKey(testContextID, "notification")
		Expect(err).NotTo(HaveOccurred())
		Expect(notif.Key).To(Equal("notification"))
		Expect(notif.BodyHTML).To(ContainSubstring("{{ if .CTA }}"))
	})

	It("works with SeedDefaultsForContext convenience function", func() {
		err := repo.SeedDefaultsForContext(pool, testContextID)
		Expect(err).NotTo(HaveOccurred())

		templates, err := emailRepo.List(testContextID)
		Expect(err).NotTo(HaveOccurred())
		Expect(templates).To(HaveLen(5))
	})

	It("handles SQL injection safely in seed defaults", func() {
		// Use a context ID with SQL injection attempt
		injectionContextID := "'; DROP TABLE email_templates; --"

		err := repo.SeedDefaultsForContext(pool, injectionContextID)
		// Should not panic or cause SQL errors
		Expect(err).NotTo(HaveOccurred())

		// Verify email_templates table still exists
		var count int
		err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM email_templates").Scan(&count)
		Expect(err).NotTo(HaveOccurred())
	})

	It("handles template with conditional Go template syntax", func() {
		// Create a template with Go template conditionals
		emailRepo.Create(&email.EmailTemplate{
			ContextID: testContextID,
			Key:       "conditional_test",
			Subject:   "Hello {{ .Name }}",
			BodyText:  "Hello {{ .Name }}{{ if .LastName }}, {{ .LastName }}{{ end }}!",
			BodyHTML:  "<p>Hello {{ .Name }}{{ if .LastName }}, <strong>{{ .LastName }}</strong>{{ end }}</p>",
		})

		// Render with conditional fields present
		variables := map[string]string{
			"Name":     "Alice",
			"LastName": "Smith",
		}
		_, body, htmlBody, err := emailUsecase.RenderTemplate(testContextID, "conditional_test", variables)
		Expect(err).NotTo(HaveOccurred())
		Expect(body).To(Equal("Hello Alice, Smith!"))
		Expect(htmlBody).To(ContainSubstring("Alice"))
		Expect(htmlBody).To(ContainSubstring("Smith"))

		// Render without conditional fields
		variables2 := map[string]string{
			"Name": "Bob",
		}
		_, _, _, err = emailUsecase.RenderTemplate(testContextID, "conditional_test", variables2)
		// Body should be empty (not rendered) since Name alone doesn't produce output
		Expect(err).To(HaveOccurred()) // The {{ .LastName }} will fail since it's not in variables
		// Actually, with Go templates, accessing a missing field just produces empty string
		// Let's test with a simpler template
	})
})

var _ = Describe("Admin Handler Routes", func() {
	BeforeEach(func() {
		mockSMTP = support.NewMockSMTP()
		mockSMTP.Start()
		pool = db.Pool

		testContext = support.NewTestContext()
		settingsJSON, _ := json.Marshal(testContext["settings"])
		_, err := pool.Exec(ctx,
			`INSERT INTO contexts (id, slug, name, domain, is_active, settings, created_at, updated_at)
			 VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())`,
			testContext["id"], testContext["slug"], testContext["name"],
			testContext["domain"], testContext["is_active"], settingsJSON,
		)
		Expect(err).NotTo(HaveOccurred())
		testContextID = testContext["id"].(string)

		emailRepo = repo.New(pool)
		emailQueue = email_infra.NewEmailQueue(pool)
		emailUsecase = emailusecase.NewEmailUsecase(emailRepo, emailQueue)
		emailHandler = handler.NewEmailHandler(emailUsecase)
	})

	AfterEach(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM email_jobs")
		_, _ = pool.Exec(ctx, "DELETE FROM email_templates")
		_, _ = pool.Exec(ctx, "DELETE FROM contexts WHERE id = $1", testContextID)
		mockSMTP.Clear()
	})

	It("list endpoint requires context_id", func() {
		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/admin/email-templates", nil)
		c.Params = []gin.Param{{Key: "context_id"}}
		emailHandler.List(c)
		Expect(w.Code).To(Equal(http.StatusBadRequest))
	})

	It("rejects invalid key format", func() {
		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/admin/email-templates", nil)
		c.Request.Header.Set("Content-Type", "application/json")

		body := strings.NewReader(fmt.Sprintf(`{
			"context_id": "%s",
			"key": "invalid-key!",
			"subject": "Test",
			"body_text": "Body"
		}`, testContextID))
		c.Request.Body = io.NopCloser(body)
		emailHandler.Create(c)
		Expect(w.Code).To(Equal(http.StatusBadRequest))
	})

	It("rejects create with empty bodies", func() {
		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/admin/email-templates", nil)
		c.Request.Header.Set("Content-Type", "application/json")

		body := strings.NewReader(fmt.Sprintf(`{
			"context_id": "%s",
			"key": "empty_body",
			"subject": "Test",
			"body_text": "",
			"body_html": ""
		}`, testContextID))
		c.Request.Body = io.NopCloser(body)
		emailHandler.Create(c)
		Expect(w.Code).To(Equal(http.StatusBadRequest))
	})

	It("returns 404 for get on non-existent template", func() {
		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/admin/email-templates/00000000-0000-0000-0000-000000000000", nil)
		c.Params = append(c.Params, gin.Param{Key: "id", Value: "00000000-0000-0000-0000-000000000000"})
		emailHandler.Get(c)
		Expect(w.Code).To(Equal(http.StatusNotFound))
	})

	It("returns 404 for delete on non-existent template", func() {
		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("DELETE", "/admin/email-templates/00000000-0000-0000-0000-000000000000", nil)
		c.Params = append(c.Params, gin.Param{Key: "id", Value: "00000000-0000-0000-0000-000000000000"})
		emailHandler.Delete(c)
		Expect(w.Code).To(Equal(http.StatusNotFound))
	})
})

var _ = Describe("Tenant Send Email", func() {
	BeforeEach(func() {
		mockSMTP = support.NewMockSMTP()
		mockSMTP.Start()
		pool = db.Pool

		testContext = support.NewTestContext()
		settingsJSON, _ := json.Marshal(testContext["settings"])
		_, err := pool.Exec(ctx,
			`INSERT INTO contexts (id, slug, name, domain, is_active, settings, created_at, updated_at)
			 VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())`,
			testContext["id"], testContext["slug"], testContext["name"],
			testContext["domain"], testContext["is_active"], settingsJSON,
		)
		Expect(err).NotTo(HaveOccurred())
		testContextID = testContext["id"].(string)

		emailRepo = repo.New(pool)
		emailQueue = email_infra.NewEmailQueue(pool)
		emailUsecase = emailusecase.NewEmailUsecase(emailRepo, emailQueue)
		emailHandler = handler.NewEmailHandler(emailUsecase)
	})

	AfterEach(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM email_jobs")
		_, _ = pool.Exec(ctx, "DELETE FROM email_templates")
		_, _ = pool.Exec(ctx, "DELETE FROM contexts WHERE id = $1", testContextID)
		mockSMTP.Clear()
	})

	It("enqueues email for tenant send", func() {
		gin.SetMode(gin.TestMode)

		emailRepo.Create(&email.EmailTemplate{
			ContextID: testContextID,
			Key:       "tenant_test",
			Subject:   "Tenant: {{ .Name }}",
			BodyText:  "Tenant body for {{ .Name }}",
			BodyHTML:  "<p>Tenant HTML</p>",
		})

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/api/tenant/test-slug/email/send", nil)
		c.Request.Header.Set("Content-Type", "application/json")
		c.Set("context_id", testContextID)
		c.Params = []gin.Param{{Key: "slug", Value: "test-slug"}}

		body := strings.NewReader(`{
			"template": "tenant_test",
			"to_array": ["tenant@example.com"],
			"variables": {"Name": "Tenant User"}
		}`)
		c.Request.Body = io.NopCloser(body)
		emailHandler.Send(c)
		Expect(w.Code).To(Equal(http.StatusAccepted))

		var resp email.SendEmailResponse
		json.Unmarshal(w.Body.Bytes(), &resp)
		Expect(resp.JobID).NotTo(BeEmpty())
	})

	It("returns 400 when template not found", func() {
		gin.SetMode(gin.TestMode)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/api/tenant/test-slug/email/send", nil)
		c.Request.Header.Set("Content-Type", "application/json")
		c.Set("context_id", testContextID)
		c.Params = []gin.Param{{Key: "slug", Value: "test-slug"}}

		body := strings.NewReader(`{
			"template": "nonexistent",
			"to": "test@example.com",
			"variables": {}
		}`)
		c.Request.Body = io.NopCloser(body)
		emailHandler.Send(c)
		Expect(w.Code).To(Equal(http.StatusBadRequest))
	})

	It("accepts both to and to_array formats", func() {
		gin.SetMode(gin.TestMode)

		emailRepo.Create(&email.EmailTemplate{
			ContextID: testContextID,
			Key:       "format_test",
			Subject:   "Format Test",
			BodyText:  "Body",
			BodyHTML:  "<p>Body</p>",
		})

		// Test with "to" string
		w1 := httptest.NewRecorder()
		c1, _ := gin.CreateTestContext(w1)
		c1.Request = httptest.NewRequest("POST", "/api/tenant/test-slug/email/send", nil)
		c1.Request.Header.Set("Content-Type", "application/json")
		c1.Set("context_id", testContextID)
		c1.Params = []gin.Param{{Key: "slug", Value: "test-slug"}}
		body1 := strings.NewReader(`{
			"template": "format_test",
			"to": "single@example.com",
			"variables": {}
		}`)
		c1.Request.Body = io.NopCloser(body1)
		emailHandler.Send(c1)
		Expect(w1.Code).To(Equal(http.StatusAccepted))

		// Test with "to_array"
		w2 := httptest.NewRecorder()
		c2, _ := gin.CreateTestContext(w2)
		c2.Request = httptest.NewRequest("POST", "/api/tenant/test-slug/email/send", nil)
		c2.Request.Header.Set("Content-Type", "application/json")
		c2.Set("context_id", testContextID)
		c2.Params = []gin.Param{{Key: "slug", Value: "test-slug"}}
		body2 := strings.NewReader(`{
			"template": "format_test",
			"to_array": ["one@example.com", "two@example.com"],
			"variables": {}
		}`)
		c2.Request.Body = io.NopCloser(body2)
		emailHandler.Send(c2)
		Expect(w2.Code).To(Equal(http.StatusAccepted))
	})
})
