package cicd_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/hekemen/automata/cicd/support"
	"github.com/hekemen/automata/internal/adapter/api/handler"
	ttracking "github.com/hekemen/automata/internal/adapter/tracking"
	cd "github.com/hekemen/automata/internal/domain/contact"
	"github.com/hekemen/automata/internal/domain/email"
	domainform "github.com/hekemen/automata/internal/domain/form"
	crepo "github.com/hekemen/automata/internal/infrastructure/contact/repo"
	"github.com/hekemen/automata/internal/infrastructure/email/repo"
	form_repo "github.com/hekemen/automata/internal/infrastructure/form/repo"
	email_infra "github.com/hekemen/automata/internal/infrastructure/queue"
	trepo "github.com/hekemen/automata/internal/infrastructure/tracking/repo"
	emailusecase "github.com/hekemen/automata/internal/usecase/email"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var (
	emailRepo    email.Repository
	emailQueue   email_infra.Queue
	emailUsecase *emailusecase.EmailUsecase
	emailHandler *handler.EmailHandler
	// mockSMTP is declared in email_queue_test.go (same package)
	testContextID string
	testContext   map[string]interface{}
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
		Expect(resp.BodyText).To(Equal("Original Body"))        // unchanged
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

		variables := map[string]interface{}{
			"Name":    "Alice",
			"Company": "Acme Corp",
		}

		subject, body, htmlBody, err := emailUsecase.RenderTemplate(testContextID, "render_test", variables)
		Expect(err).NotTo(HaveOccurred())
		Expect(subject).To(Equal("Hello Alice!"))
		Expect(body).To(Equal("Dear Alice, welcome to Acme Corp!"))
		Expect(htmlBody).To(Equal("<p>Dear <strong>Alice</strong>, welcome to Acme Corp!</p>"))
	})

	It("renders Go text/template variables correctly", func() {
		emailRepo.Create(&email.EmailTemplate{
			ContextID: testContextID,
			Key:       "undefined_test",
			Subject:   "Hello {{ .Missing }}!",
			BodyText:  "Body {{ .Missing }}",
			BodyHTML:  "<p>Body</p>",
		})

		variables := map[string]interface{}{
			"Other": "value",
		}

		subject, body, htmlBody, err := emailUsecase.RenderTemplate(testContextID, "undefined_test", variables)
		Expect(err).NotTo(HaveOccurred())
		// Go templates output "<no value>" for missing map keys
		Expect(subject).To(Equal("Hello <no value>!"))
		Expect(body).To(Equal("Body <no value>"))
		Expect(htmlBody).To(Equal("<p>Body</p>"))
	})

	It("renders HTML and plain text variants", func() {
		emailRepo.Create(&email.EmailTemplate{
			ContextID: testContextID,
			Key:       "html_test",
			Subject:   "HTML Test {{ .Name }}",
			BodyText:  "Plain text body for {{ .Name }}",
			BodyHTML:  "<h1>HTML body for {{ .Name }}</h1>",
		})

		variables := map[string]interface{}{"Name": "Bob"}
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
			ToArray:   []string{"alice@example.com", "bob@example.com"},
			Template:  "send_test",
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
			To:        "single@example.com",
			Template:  "to_field_test",
			Variables: map[string]string{},
		}
		jobID1, err := emailUsecase.SendEmail(testContextID, req1)
		Expect(err).NotTo(HaveOccurred())
		Expect(jobID1).NotTo(BeEmpty())

		// Array
		req2 := email.SendEmailRequest{
			ToArray:   []string{"one@example.com", "two@example.com"},
			Template:  "to_field_test",
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
			To:        "tester@example.com",
			Template:  "test_send",
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
			To:        "preview@example.com",
			Template:  "preview_test",
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

		emailRepo = repo.New(pool)
		emailQueue = email_infra.NewEmailQueue(pool)
		emailUsecase = emailusecase.NewEmailUsecase(emailRepo, emailQueue)
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
		variables := map[string]interface{}{
			"Name":     "Alice",
			"LastName": "Smith",
		}
		_, body, htmlBody, err := emailUsecase.RenderTemplate(testContextID, "conditional_test", variables)
		Expect(err).NotTo(HaveOccurred())
		Expect(body).To(Equal("Hello Alice, Smith!"))
		Expect(htmlBody).To(ContainSubstring("Alice"))
		Expect(htmlBody).To(ContainSubstring("Smith"))

		// Render without conditional fields
		variables2 := map[string]interface{}{
			"Name": "Bob",
		}
		_, body2, htmlBody2, err := emailUsecase.RenderTemplate(testContextID, "conditional_test", variables2)
		Expect(err).NotTo(HaveOccurred())
		// Go templates output empty strings for missing fields; conditionals evaluate to empty
		Expect(body2).To(Equal("Hello Bob!"))
		Expect(htmlBody2).To(ContainSubstring("Bob"))
		Expect(htmlBody2).ToNot(ContainSubstring("Smith"))
	})

	It("renders nested variables via dot access", func() {
		emailRepo.Create(&email.EmailTemplate{
			ContextID: testContextID,
			Key:       "nested_test",
			Subject:   "Hi {{ .first_name }}",
			BodyText:  "Plan: {{ .custom.plan }}; from {{ .form.name }}",
			BodyHTML:  "<p>Plan: {{ .custom.plan }}</p>",
		})

		variables := map[string]interface{}{
			"first_name": "Ada",
			"custom":     map[string]interface{}{"plan": "pro"},
			"form":       map[string]interface{}{"name": "Web"},
		}

		subject, body, htmlBody, err := emailUsecase.RenderTemplate(testContextID, "nested_test", variables)
		Expect(err).NotTo(HaveOccurred())
		Expect(subject).To(Equal("Hi Ada"))
		Expect(body).To(Equal("Plan: pro; from Web"))
		Expect(htmlBody).To(Equal("<p>Plan: pro</p>"))
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

var _ = Describe("Context Send Email", func() {
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

	It("enqueues email for context send", func() {
		gin.SetMode(gin.TestMode)

		emailRepo.Create(&email.EmailTemplate{
			ContextID: testContextID,
			Key:       "context_test",
			Subject:   "Context: {{ .Name }}",
			BodyText:  "Context body for {{ .Name }}",
			BodyHTML:  "<p>Context HTML</p>",
		})

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/api/context/test-slug/email/send", nil)
		c.Request.Header.Set("Content-Type", "application/json")
		c.Set("context_id", testContextID)
		c.Params = []gin.Param{{Key: "slug", Value: "test-slug"}}

		body := strings.NewReader(`{
			"template": "context_test",
			"to_array": ["context@example.com"],
			"variables": {"Name": "Context User"}
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
		c.Request = httptest.NewRequest("POST", "/api/context/test-slug/email/send", nil)
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
		c1.Request = httptest.NewRequest("POST", "/api/context/test-slug/email/send", nil)
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
		c2.Request = httptest.NewRequest("POST", "/api/context/test-slug/email/send", nil)
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

var _ = Describe("Email Template kind and form binding", func() {
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

	It("creates a form-kind template with form_id", func() {
		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/admin/email-templates", nil)
		c.Request.Header.Set("Content-Type", "application/json")

		formID := "11111111-1111-1111-1111-111111111111"
		body := strings.NewReader(fmt.Sprintf(`{
			"context_id": "%s",
			"key": "form_reply",
			"subject": "Thanks for reaching out",
			"body_text": "Hello {{ .first_name }}",
			"kind": "form",
			"form_id": "%s"
		}`, testContextID, formID))
		c.Request.Body = io.NopCloser(body)

		emailHandler.Create(c)
		Expect(w.Code).To(Equal(http.StatusCreated))

		var resp email.TemplateResponse
		json.Unmarshal(w.Body.Bytes(), &resp)
		Expect(resp.Kind).To(Equal("form"))
		Expect(resp.FormID).NotTo(BeNil())
		Expect(*resp.FormID).To(Equal(formID))
	})

	It("rejects form-kind template without form_id", func() {
		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/admin/email-templates", nil)
		c.Request.Header.Set("Content-Type", "application/json")

		body := strings.NewReader(fmt.Sprintf(`{
			"context_id": "%s",
			"key": "form_noid",
			"subject": "No Form ID",
			"body_text": "Body",
			"kind": "form"
		}`, testContextID))
		c.Request.Body = io.NopCloser(body)

		emailHandler.Create(c)
		Expect(w.Code).To(Equal(http.StatusBadRequest))
	})

	It("rejects invalid kind", func() {
		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/admin/email-templates", nil)
		c.Request.Header.Set("Content-Type", "application/json")

		body := strings.NewReader(fmt.Sprintf(`{
			"context_id": "%s",
			"key": "bad_kind",
			"subject": "Bad Kind",
			"body_text": "Body",
			"kind": "bulk"
		}`, testContextID))
		c.Request.Body = io.NopCloser(body)

		emailHandler.Create(c)
		Expect(w.Code).To(Equal(http.StatusBadRequest))
	})

	It("defaults kind to standalone when omitted", func() {
		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/admin/email-templates", nil)
		c.Request.Header.Set("Content-Type", "application/json")

		body := strings.NewReader(fmt.Sprintf(`{
			"context_id": "%s",
			"key": "no_kind",
			"subject": "Default Kind",
			"body_text": "Body"
		}`, testContextID))
		c.Request.Body = io.NopCloser(body)

		emailHandler.Create(c)
		Expect(w.Code).To(Equal(http.StatusCreated))

		var resp email.TemplateResponse
		json.Unmarshal(w.Body.Bytes(), &resp)
		Expect(resp.Kind).To(Equal("standalone"))
		Expect(resp.FormID).To(BeNil())
	})

	It("updates a template to form kind with form_id", func() {
		gin.SetMode(gin.TestMode)

		emailRepo.Create(&email.EmailTemplate{
			ContextID: testContextID,
			Key:       "kind_flip",
			Subject:   "Flip Subject",
			BodyText:  "Flip Body",
		})

		templates, err := emailRepo.List(testContextID)
		Expect(err).NotTo(HaveOccurred())
		Expect(templates).To(HaveLen(1))
		template := templates[0]
		Expect(template.Kind).To(Equal("standalone"))

		formID := "22222222-2222-2222-2222-222222222222"
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("PUT", "/admin/email-templates/"+template.ID, nil)
		c.Request.Header.Set("Content-Type", "application/json")
		c.Params = append(c.Params, gin.Param{Key: "id", Value: template.ID})

		body := strings.NewReader(fmt.Sprintf(`{"kind": "form", "form_id": "%s"}`, formID))
		c.Request.Body = io.NopCloser(body)
		emailHandler.Update(c)
		Expect(w.Code).To(Equal(http.StatusOK))

		var resp email.TemplateResponse
		json.Unmarshal(w.Body.Bytes(), &resp)
		Expect(resp.Kind).To(Equal("form"))
		Expect(resp.FormID).NotTo(BeNil())
		Expect(*resp.FormID).To(Equal(formID))
	})
})

var _ = Describe("Standalone template send", func() {
	var contactRepo cd.Repository
	var contactID string

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

		contactRepo = crepo.New(pool)
		emailRepo = repo.New(pool)
		emailQueue = email_infra.NewEmailQueue(pool)
		emailUsecase = emailusecase.NewEmailUsecaseWithContact(emailRepo, emailQueue, contactRepo)
		emailHandler = handler.NewEmailHandler(emailUsecase)

		// Create a contact with an email
		contactEmail := "standalone@example.com"
		contactID = uuid.New().String()
		err = contactRepo.Create(&cd.Contact{
			ID:        contactID,
			ContextID: testContextID,
			Email:     &contactEmail,
			FirstName: "Ada",
			LastName:  "Lovelace",
			Company:   "Analytical Engines",
		})
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM email_jobs")
		_, _ = pool.Exec(ctx, "DELETE FROM email_templates")
		_, _ = pool.Exec(ctx, "DELETE FROM contact_activities WHERE context_id = $1", testContextID)
		_, _ = pool.Exec(ctx, "DELETE FROM contacts WHERE id = $1", contactID)
		_, _ = pool.Exec(ctx, "DELETE FROM contexts WHERE id = $1", testContextID)
		mockSMTP.Clear()
	})

	createStandaloneTemplate := func(key, subject, body string) string {
		emailRepo.Create(&email.EmailTemplate{
			ContextID: testContextID,
			Key:       key,
			Subject:   subject,
			BodyText:  body,
			BodyHTML:  "<p>" + body + "</p>",
			Kind:      email.TemplateKindStandalone,
		})
		templates, err := emailRepo.List(testContextID)
		Expect(err).NotTo(HaveOccurred())
		return templates[0].ID
	}

	It("sends a standalone template to a contact and records activity", func() {
		gin.SetMode(gin.TestMode)
		tid := createStandaloneTemplate("solo_send", "Hello {{ .first_name }}", "Hi {{ .first_name }} from {{ .company }}")

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/admin/contexts/"+testContextID+"/email-templates/"+tid+"/send", nil)
		c.Request.Header.Set("Content-Type", "application/json")
		c.Params = []gin.Param{{Key: "id", Value: testContextID}, {Key: "tid", Value: tid}}
		c.Request.Body = io.NopCloser(strings.NewReader(fmt.Sprintf(`{"contact_id": "%s"}`, contactID)))

		emailHandler.SendToContact(c)
		Expect(w.Code).To(Equal(http.StatusAccepted))

		var resp struct {
			JobID string `json:"job_id"`
		}
		json.Unmarshal(w.Body.Bytes(), &resp)
		Expect(resp.JobID).NotTo(BeEmpty())

		// Job enqueued with rendered subject/body
		var subject, body string
		err := pool.QueryRow(ctx,
			"SELECT subject, body FROM email_jobs WHERE id = $1", resp.JobID,
		).Scan(&subject, &body)
		Expect(err).NotTo(HaveOccurred())
		Expect(subject).To(Equal("Hello Ada"))
		Expect(body).To(Equal("Hi Ada from Analytical Engines"))

		// Activity recorded on the contact
		var count int
		err = pool.QueryRow(ctx,
			"SELECT COUNT(*) FROM contact_activities WHERE contact_id = $1 AND type = 'email_sent'", contactID,
		).Scan(&count)
		Expect(err).NotTo(HaveOccurred())
		Expect(count).To(Equal(1))
	})

	It("rejects send to a contact without an email", func() {
		gin.SetMode(gin.TestMode)
		tid := createStandaloneTemplate("no_email", "Hi", "Body")

		noEmailID := uuid.New().String()
		err := contactRepo.Create(&cd.Contact{
			ID:        noEmailID,
			ContextID: testContextID,
			FirstName: "No",
			LastName:  "Email",
		})
		Expect(err).NotTo(HaveOccurred())
		defer pool.Exec(ctx, "DELETE FROM contacts WHERE id = $1", noEmailID)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/admin/contexts/"+testContextID+"/email-templates/"+tid+"/send", nil)
		c.Request.Header.Set("Content-Type", "application/json")
		c.Params = []gin.Param{{Key: "id", Value: testContextID}, {Key: "tid", Value: tid}}
		c.Request.Body = io.NopCloser(strings.NewReader(fmt.Sprintf(`{"contact_id": "%s"}`, noEmailID)))

		emailHandler.SendToContact(c)
		Expect(w.Code).To(Equal(http.StatusBadRequest))
	})

	It("rejects send with a form-kind template", func() {
		gin.SetMode(gin.TestMode)
		formID := "33333333-3333-3333-3333-333333333333"
		emailRepo.Create(&email.EmailTemplate{
			ContextID: testContextID,
			Key:       "form_only",
			Subject:   "Form reply",
			BodyText:  "Body",
			Kind:      email.TemplateKindForm,
			FormID:    &formID,
		})
		templates, err := emailRepo.List(testContextID)
		Expect(err).NotTo(HaveOccurred())
		tid := templates[0].ID

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/admin/contexts/"+testContextID+"/email-templates/"+tid+"/send", nil)
		c.Request.Header.Set("Content-Type", "application/json")
		c.Params = []gin.Param{{Key: "id", Value: testContextID}, {Key: "tid", Value: tid}}
		c.Request.Body = io.NopCloser(strings.NewReader(fmt.Sprintf(`{"contact_id": "%s"}`, contactID)))

		emailHandler.SendToContact(c)
		Expect(w.Code).To(Equal(http.StatusBadRequest))
	})

	It("returns 404 for unknown template id", func() {
		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/admin/contexts/"+testContextID+"/email-templates/99999999-9999-9999-9999-999999999999/send", nil)
		c.Request.Header.Set("Content-Type", "application/json")
		c.Params = []gin.Param{{Key: "id", Value: testContextID}, {Key: "tid", Value: "99999999-9999-9999-9999-999999999999"}}
		c.Request.Body = io.NopCloser(strings.NewReader(fmt.Sprintf(`{"contact_id": "%s"}`, contactID)))

		emailHandler.SendToContact(c)
		Expect(w.Code).To(Equal(http.StatusNotFound))
	})

	It("returns 404 for unknown contact id", func() {
		gin.SetMode(gin.TestMode)
		tid := createStandaloneTemplate("unknown_contact", "Hi", "Body")

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/admin/contexts/"+testContextID+"/email-templates/"+tid+"/send", nil)
		c.Request.Header.Set("Content-Type", "application/json")
		c.Params = []gin.Param{{Key: "id", Value: testContextID}, {Key: "tid", Value: tid}}
		c.Request.Body = io.NopCloser(strings.NewReader(`{"contact_id": "99999999-9999-9999-9999-999999999999"}`))

		emailHandler.SendToContact(c)
		Expect(w.Code).To(Equal(http.StatusNotFound))
	})
})

var _ = Describe("Form-integrated template send", func() {
	var (
		fRepo     domainform.Repository
		fHandler  *handler.FormHandler
		engine    *gin.Engine
		formID    string
		contactID string
	)

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

		fRepo = form_repo.New(pool)
		contactRepo := crepo.New(pool)
		emailRepo = repo.New(pool)
		emailQueue = email_infra.NewEmailQueue(pool)
		emailUsecase = emailusecase.NewEmailUsecaseWithContact(emailRepo, emailQueue, contactRepo)
		fHandler = handler.NewFormHandlerWithEmail(fRepo, contactRepo, trepo.New(pool), emailUsecase)

		engine = gin.New()
		forms := engine.Group("/form")
		forms.Use(ttracking.ContextMiddleware())
		forms.POST("/submit/:slug", fHandler.SubmitForm)

		// Form with an optional email field and a text field
		formID = uuid.New().String()
		err = fRepo.Create(&domainform.Form{
			ID:        formID,
			ContextID: testContextID,
			Slug:      "contact-us",
			Name:      "Contact Us",
			Fields: []domainform.FormField{
				{Slug: "email", Type: "email", Label: "Email", Required: false},
				{Slug: "message", Type: "text", Label: "Message", Required: false},
			},
		})
		Expect(err).NotTo(HaveOccurred())

		// Contact matching the submitter's email
		contactEmail := "submitter@example.com"
		contactID = uuid.New().String()
		err = contactRepo.Create(&cd.Contact{
			ID:        contactID,
			ContextID: testContextID,
			Email:     &contactEmail,
			FirstName: "Ada",
		})
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM email_jobs")
		_, _ = pool.Exec(ctx, "DELETE FROM email_templates")
		_, _ = pool.Exec(ctx, "DELETE FROM form_submissions WHERE context_id = $1", testContextID)
		_, _ = pool.Exec(ctx, "DELETE FROM forms WHERE id = $1", formID)
		_, _ = pool.Exec(ctx, "DELETE FROM contact_activities WHERE context_id = $1", testContextID)
		_, _ = pool.Exec(ctx, "DELETE FROM contacts WHERE id = $1", contactID)
		_, _ = pool.Exec(ctx, "DELETE FROM contexts WHERE id = $1", testContextID)
		mockSMTP.Clear()
	})

	submit := func(data map[string]interface{}) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		body, _ := json.Marshal(map[string]interface{}{"data": data})
		req := httptest.NewRequest("POST", "/form/submit/contact-us", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Context-ID", testContextID)
		engine.ServeHTTP(w, req)
		return w
	}

	It("sends a form-integrated template to the submitter and records activity", func() {
		formIDPtr := formID
		emailRepo.Create(&email.EmailTemplate{
			ContextID: testContextID,
			Key:       "form_reply",
			Subject:   "Re: {{ .form.message }}",
			BodyText:  "Hi {{ .first_name }}, you said: {{ .form.message }} (via {{ .form_name }})",
			Kind:      email.TemplateKindForm,
			FormID:    &formIDPtr,
		})

		w := submit(map[string]interface{}{"email": "submitter@example.com", "message": "Hello"})
		Expect(w.Code).To(Equal(http.StatusCreated))

		var to, subject, body string
		err := pool.QueryRow(ctx,
			"SELECT to_addresses[1], subject, body FROM email_jobs WHERE context_id = $1", testContextID,
		).Scan(&to, &subject, &body)
		Expect(err).NotTo(HaveOccurred())
		Expect(to).To(Equal("submitter@example.com"))
		Expect(subject).To(Equal("Re: Hello"))
		Expect(body).To(Equal("Hi Ada, you said: Hello (via Contact Us)"))

		var count int
		err = pool.QueryRow(ctx,
			"SELECT COUNT(*) FROM contact_activities WHERE contact_id = $1 AND type = 'email_sent'", contactID,
		).Scan(&count)
		Expect(err).NotTo(HaveOccurred())
		Expect(count).To(Equal(1))
	})

	It("skips sending when the submission has no email value", func() {
		formIDPtr := formID
		emailRepo.Create(&email.EmailTemplate{
			ContextID: testContextID,
			Key:       "form_reply_no_email",
			Subject:   "Re",
			BodyText:  "Body",
			Kind:      email.TemplateKindForm,
			FormID:    &formIDPtr,
		})

		w := submit(map[string]interface{}{"message": "no email here"})
		Expect(w.Code).To(Equal(http.StatusCreated))

		var count int
		err := pool.QueryRow(ctx,
			"SELECT COUNT(*) FROM email_jobs WHERE context_id = $1", testContextID,
		).Scan(&count)
		Expect(err).NotTo(HaveOccurred())
		Expect(count).To(Equal(0))
	})

	It("keeps the submission succeeding when the template has broken syntax", func() {
		formIDPtr := formID
		emailRepo.Create(&email.EmailTemplate{
			ContextID: testContextID,
			Key:       "broken_form",
			Subject:   "Re: {{ .form.",
			BodyText:  "Body",
			Kind:      email.TemplateKindForm,
			FormID:    &formIDPtr,
		})

		w := submit(map[string]interface{}{"email": "submitter@example.com", "message": "Hello"})
		Expect(w.Code).To(Equal(http.StatusCreated))

		var count int
		err := pool.QueryRow(ctx,
			"SELECT COUNT(*) FROM email_jobs WHERE context_id = $1", testContextID,
		).Scan(&count)
		Expect(err).NotTo(HaveOccurred())
		Expect(count).To(Equal(0))
	})
})
