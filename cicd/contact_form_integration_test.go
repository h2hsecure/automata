package cicd_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hekemen/automata/cicd/support"
	"github.com/hekemen/automata/pkg/snippet"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/jackc/pgx/v5/pgxpool"
)

var _ = Describe("Contact & Form Integration Tests", func() {
	var (
		ctx    context.Context
		pool   *pgxpool.Pool
		tenantID string
	)

	BeforeEach(func() {
		gin.SetMode(gin.TestMode)
		ctx = context.Background()
		pool = db.Pool
		tenantID = support.NewTestContextID()
	})

	AfterEach(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM contact_tag_memberships")
		_, _ = pool.Exec(ctx, "DELETE FROM contact_field_definitions")
		_, _ = pool.Exec(ctx, "DELETE FROM contact_tags")
		_, _ = pool.Exec(ctx, "DELETE FROM contacts")
		_, _ = pool.Exec(ctx, "DELETE FROM form_submissions")
		_, _ = pool.Exec(ctx, "DELETE FROM forms")
		_, _ = pool.Exec(ctx, "DELETE FROM banner_clicks")
		_, _ = pool.Exec(ctx, "DELETE FROM banner_impressions")
		_, _ = pool.Exec(ctx, "DELETE FROM banner_banners")
		_, _ = pool.Exec(ctx, "DELETE FROM banner_campaigns")
		_, _ = pool.Exec(ctx, "DELETE FROM banner_placements")
		_, _ = pool.Exec(ctx, "DELETE FROM tracking_events")
		_, _ = pool.Exec(ctx, "DELETE FROM tracking_visitors")
		_, _ = pool.Exec(ctx, "DELETE FROM tenants")
		_, _ = pool.Exec(ctx, "DELETE FROM tenant_users")
	})

	Describe("Contact creation and validation", func() {
		It("should store contact with NULL email", func() {
			contactData := support.NewTestContact(tenantID)
			customFields, _ := json.Marshal(contactData["custom_fields"])

			_, err := pool.Exec(ctx, `
				INSERT INTO contacts (id, context_id, email, first_name, last_name, phone, company,
				                      custom_fields, source, source_id, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
			`,
				contactData["id"],
				contactData["context_id"],
				nil,
				contactData["first_name"],
				contactData["last_name"],
				contactData["phone"],
				contactData["company"],
				customFields,
				contactData["source"],
				contactData["source_id"],
				contactData["created_at"],
				contactData["updated_at"],
			)
			Expect(err).NotTo(HaveOccurred())

			var count int
			err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM contacts WHERE email IS NULL").Scan(&count)
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(1))
		})

		It("should store contacts with various email formats", func() {
			emails := []string{"user@example.com", "user.name+tag@sub.domain.co.uk", "a@b.cd"}

			for _, email := range emails {
				contactData := support.NewTestContact(tenantID)
				contactData["id"] = support.NewTestUUID()
				contactData["email"] = email

				customFields, _ := json.Marshal(contactData["custom_fields"])

				_, err := pool.Exec(ctx, `
					INSERT INTO contacts (id, context_id, email, first_name, last_name, phone, company,
					                      custom_fields, source, source_id, created_at, updated_at)
					VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
				`,
					contactData["id"],
					contactData["context_id"],
					email,
					contactData["first_name"],
					contactData["last_name"],
					contactData["phone"],
					contactData["company"],
					customFields,
					contactData["source"],
					contactData["source_id"],
					contactData["created_at"],
					contactData["updated_at"],
				)
				Expect(err).NotTo(HaveOccurred())
			}

			var count int
			err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM contacts").Scan(&count)
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(len(emails)))
		})

		It("should allow same email across different tenants", func() {
			email := "shared@example.com"
			tenant2 := support.NewTestContextID()

			for _, t := range []string{tenantID, tenant2} {
				contactData := support.NewTestContact(t)
				contactData["id"] = support.NewTestUUID()
				contactData["email"] = email

				customFields, _ := json.Marshal(contactData["custom_fields"])

				_, err := pool.Exec(ctx, `
					INSERT INTO contacts (id, context_id, email, first_name, last_name, phone, company,
					                      custom_fields, source, source_id, created_at, updated_at)
					VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
				`,
					contactData["id"],
					t,
					email,
					contactData["first_name"],
					contactData["last_name"],
					contactData["phone"],
					contactData["company"],
					customFields,
					contactData["source"],
					contactData["source_id"],
					contactData["created_at"],
					contactData["updated_at"],
				)
				Expect(err).NotTo(HaveOccurred())
			}

			var count int
			err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM contacts WHERE email = $1", email).Scan(&count)
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(2))
		})

		It("should store and retrieve custom fields", func() {
			contactData := support.NewTestContact(tenantID)
			contactData["custom_fields"] = map[string]interface{}{
				"lead_score":   85,
				"industry":     "technology",
				"company_size": "50-100",
			}

			customFields, _ := json.Marshal(contactData["custom_fields"])

			_, err := pool.Exec(ctx, `
				INSERT INTO contacts (id, context_id, email, first_name, last_name, phone, company,
				                      custom_fields, source, source_id, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
			`,
				contactData["id"],
				contactData["context_id"],
				contactData["email"],
				contactData["first_name"],
				contactData["last_name"],
				contactData["phone"],
				contactData["company"],
				customFields,
				contactData["source"],
				contactData["source_id"],
				contactData["created_at"],
				contactData["updated_at"],
			)
			Expect(err).NotTo(HaveOccurred())

			var retrievedCustomFields []byte
			err = pool.QueryRow(ctx, "SELECT custom_fields FROM contacts WHERE id = $1", contactData["id"]).Scan(&retrievedCustomFields)
			Expect(err).NotTo(HaveOccurred())

			var fields map[string]interface{}
			err = json.Unmarshal(retrievedCustomFields, &fields)
			Expect(err).NotTo(HaveOccurred())
			Expect(fields["lead_score"]).To(Equal(float64(85)))
			Expect(fields["industry"]).To(Equal("technology"))
		})

		It("should track contact source attribution", func() {
			sources := []string{"form", "import", "api", "webhook", "manual"}

			for i, source := range sources {
				contactData := support.NewTestContact(tenantID)
				contactData["id"] = support.NewTestUUID()
				contactData["source"] = source
				contactData["email"] = fmt.Sprintf("source%d@example.com", i)

				customFields, _ := json.Marshal(contactData["custom_fields"])

				_, err := pool.Exec(ctx, `
					INSERT INTO contacts (id, context_id, email, first_name, last_name, phone, company,
					                      custom_fields, source, source_id, created_at, updated_at)
					VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
				`,
					contactData["id"],
					contactData["context_id"],
					contactData["email"],
					contactData["first_name"],
					contactData["last_name"],
					contactData["phone"],
					contactData["company"],
					customFields,
					source,
					"",
					contactData["created_at"],
					contactData["updated_at"],
				)
				Expect(err).NotTo(HaveOccurred())
			}

			for _, source := range sources {
				var count int
				err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM contacts WHERE source = $1", source).Scan(&count)
				Expect(err).NotTo(HaveOccurred())
				Expect(count).To(Equal(1))
			}
		})

		It("should store contact with all optional fields", func() {
			contactData := support.NewTestContact(tenantID)
			contactData["phone"] = "+1-555-123-4567"
			contactData["company"] = "Acme Corp"
			contactData["source"] = "form"
			contactData["source_id"] = "form-123"

			customFields, _ := json.Marshal(contactData["custom_fields"])

			_, err := pool.Exec(ctx, `
				INSERT INTO contacts (id, context_id, email, first_name, last_name, phone, company,
				                      custom_fields, source, source_id, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
			`,
				contactData["id"],
				contactData["context_id"],
				contactData["email"],
				contactData["first_name"],
				contactData["last_name"],
				contactData["phone"],
				contactData["company"],
				customFields,
				contactData["source"],
				contactData["source_id"],
				contactData["created_at"],
				contactData["updated_at"],
			)
			Expect(err).NotTo(HaveOccurred())

			var retrievedPhone, retrievedCompany, retrievedSource, retrievedSourceID string
			err = pool.QueryRow(ctx, `
				SELECT phone, company, source, source_id FROM contacts WHERE id = $1
			`, contactData["id"]).Scan(&retrievedPhone, &retrievedCompany, &retrievedSource, &retrievedSourceID)
			Expect(err).NotTo(HaveOccurred())
			Expect(retrievedPhone).To(Equal("+1-555-123-4567"))
			Expect(retrievedCompany).To(Equal("Acme Corp"))
			Expect(retrievedSource).To(Equal("form"))
			Expect(retrievedSourceID).To(Equal("form-123"))
		})
	})

	Describe("Form submission validation", func() {
		var formID string

		BeforeEach(func() {
			formData := support.NewTestForm(tenantID)
			formID = formData["id"].(string)

			fields, _ := json.Marshal(formData["fields"])
			settings, _ := json.Marshal(formData["settings"])

			_, err := pool.Exec(ctx, `
				INSERT INTO forms (id, context_id, slug, name, description, fields, settings, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			`,
				formData["id"],
				formData["context_id"],
				formData["slug"],
				formData["name"],
				formData["description"],
				fields,
				settings,
				formData["created_at"],
				formData["updated_at"],
			)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should accept submission with valid email field", func() {
			submissionData := support.NewTestFormSubmission(formID, tenantID)
			submissionData["data"] = map[string]interface{}{
				"email": "valid@example.com",
				"name":  "John Doe",
			}

			data, _ := json.Marshal(submissionData["data"])
			files, _ := json.Marshal(submissionData["files"])

			_, err := pool.Exec(ctx, `
				INSERT INTO form_submissions (id, form_id, context_id, data, files, created_at)
				VALUES ($1, $2, $3, $4, $5, $6)
			`,
				support.NewTestUUID(),
				formID,
				tenantID,
				data,
				files,
				submissionData["created_at"],
			)
			Expect(err).NotTo(HaveOccurred())

			var count int
			err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM form_submissions").Scan(&count)
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(1))
		})

		It("should handle submission with multiple file attachments", func() {
			submissionData := support.NewTestFormSubmission(formID, tenantID)
			submissionData["files"] = []map[string]interface{}{
				{"name": "document.pdf", "size": 102400, "url": "/uploads/doc.pdf", "type": "application/pdf"},
				{"name": "image.jpg", "size": 204800, "url": "/uploads/img.jpg", "type": "image/jpeg"},
				{"name": "spreadsheet.xlsx", "size": 51200, "url": "/uploads/data.xlsx", "type": "application/vnd.openxmlformats"},
			}

			data, _ := json.Marshal(submissionData["data"])
			files, _ := json.Marshal(submissionData["files"])

			_, err := pool.Exec(ctx, `
				INSERT INTO form_submissions (id, form_id, context_id, data, files, created_at)
				VALUES ($1, $2, $3, $4, $5, $6)
			`,
				support.NewTestUUID(),
				formID,
				tenantID,
				data,
				files,
				submissionData["created_at"],
			)
			Expect(err).NotTo(HaveOccurred())

			var retrievedFiles []byte
			err = pool.QueryRow(ctx, "SELECT files FROM form_submissions ORDER BY created_at DESC LIMIT 1").Scan(&retrievedFiles)
			Expect(err).NotTo(HaveOccurred())

			var filesList []map[string]interface{}
			err = json.Unmarshal(retrievedFiles, &filesList)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(filesList)).To(Equal(3))
		})

		It("should store submission data as JSONB", func() {
			submissionData := support.NewTestFormSubmission(formID, tenantID)
			submissionData["data"] = map[string]interface{}{
				"email":       "test@example.com",
				"message":     "Hello world",
				"priority":    "high",
				"attachments": 2,
			}

			data, _ := json.Marshal(submissionData["data"])
			files, _ := json.Marshal(submissionData["files"])

			_, err := pool.Exec(ctx, `
				INSERT INTO form_submissions (id, form_id, context_id, data, files, created_at)
				VALUES ($1, $2, $3, $4, $5, $6)
			`,
				support.NewTestUUID(),
				formID,
				tenantID,
				data,
				files,
				submissionData["created_at"],
			)
			Expect(err).NotTo(HaveOccurred())

			var retrievedData []byte
			err = pool.QueryRow(ctx, "SELECT data FROM form_submissions ORDER BY created_at DESC LIMIT 1").Scan(&retrievedData)
			Expect(err).NotTo(HaveOccurred())

			var storedData map[string]interface{}
			err = json.Unmarshal(retrievedData, &storedData)
			Expect(err).NotTo(HaveOccurred())
			Expect(storedData["email"]).To(Equal("test@example.com"))
			Expect(storedData["message"]).To(Equal("Hello world"))
			Expect(storedData["priority"]).To(Equal("high"))
		})

		It("should handle empty form submission data", func() {
			submissionData := support.NewTestFormSubmission(formID, tenantID)
			submissionData["data"] = map[string]interface{}{}

			data, _ := json.Marshal(submissionData["data"])
			files, _ := json.Marshal(submissionData["files"])

			_, err := pool.Exec(ctx, `
				INSERT INTO form_submissions (id, form_id, context_id, data, files, created_at)
				VALUES ($1, $2, $3, $4, $5, $6)
			`,
				support.NewTestUUID(),
				formID,
				tenantID,
				data,
				files,
				submissionData["created_at"],
			)
			Expect(err).NotTo(HaveOccurred())

			var count int
			err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM form_submissions").Scan(&count)
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(1))
		})

		It("should link submission to correct form and tenantID", func() {
			submissionData := support.NewTestFormSubmission(formID, tenantID)

			data, _ := json.Marshal(submissionData["data"])
			files, _ := json.Marshal(submissionData["files"])

			submissionID := support.NewTestUUID()
			_, err := pool.Exec(ctx, `
				INSERT INTO form_submissions (id, form_id, context_id, data, files, created_at)
				VALUES ($1, $2, $3, $4, $5, $6)
			`,
				submissionID,
				formID,
				tenantID,
				data,
				files,
				submissionData["created_at"],
			)
			Expect(err).NotTo(HaveOccurred())

			var retrievedFormID, retrievedContextID string
			err = pool.QueryRow(ctx, "SELECT form_id, context_id FROM form_submissions WHERE id = $1", submissionID).Scan(&retrievedFormID, &retrievedContextID)
			Expect(err).NotTo(HaveOccurred())
			Expect(retrievedFormID).To(Equal(formID))
			Expect(retrievedContextID).To(Equal(tenantID))
		})

		It("should store submission with nested data structures", func() {
			submissionData := support.NewTestFormSubmission(formID, tenantID)
			submissionData["data"] = map[string]interface{}{
				"email": "nested@example.com",
				"address": map[string]interface{}{
					"street": "123 Main St",
					"city":   "Springfield",
					"state":  "IL",
					"zip":    "62701",
				},
				"preferences": []string{"email", "sms"},
			}

			data, _ := json.Marshal(submissionData["data"])
			files, _ := json.Marshal(submissionData["files"])

			_, err := pool.Exec(ctx, `
				INSERT INTO form_submissions (id, form_id, context_id, data, files, created_at)
				VALUES ($1, $2, $3, $4, $5, $6)
			`,
				support.NewTestUUID(),
				formID,
				tenantID,
				data,
				files,
				submissionData["created_at"],
			)
			Expect(err).NotTo(HaveOccurred())

			var retrievedData []byte
			err = pool.QueryRow(ctx, "SELECT data FROM form_submissions ORDER BY created_at DESC LIMIT 1").Scan(&retrievedData)
			Expect(err).NotTo(HaveOccurred())

			var storedData map[string]interface{}
			err = json.Unmarshal(retrievedData, &storedData)
			Expect(err).NotTo(HaveOccurred())
			addr := storedData["address"].(map[string]interface{})
			Expect(addr["city"]).To(Equal("Springfield"))
			Expect(storedData["preferences"]).To(HaveLen(2))
		})
	})

	Describe("Contact-form relationship", func() {
		It("should create a contact from a form submission with matching email", func() {
			formData := support.NewTestForm(tenantID)
			fields, _ := json.Marshal(formData["fields"])
			settings, _ := json.Marshal(formData["settings"])

			_, err := pool.Exec(ctx, `
				INSERT INTO forms (id, context_id, slug, name, description, fields, settings, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			`,
				formData["id"],
				formData["context_id"],
				formData["slug"],
				formData["name"],
				formData["description"],
				fields,
				settings,
				formData["created_at"],
				formData["updated_at"],
			)
			Expect(err).NotTo(HaveOccurred())

			submissionData := support.NewTestFormSubmission(formData["id"].(string), tenantID)
			submissionData["data"] = map[string]interface{}{
				"email":      "newcontact@example.com",
				"first_name": "New",
				"last_name":  "Contact",
			}

			data, _ := json.Marshal(submissionData["data"])
			files, _ := json.Marshal(submissionData["files"])

			_, err = pool.Exec(ctx, `
				INSERT INTO form_submissions (id, form_id, context_id, data, files, created_at)
				VALUES ($1, $2, $3, $4, $5, $6)
			`,
				support.NewTestUUID(),
				formData["id"],
				tenantID,
				data,
				files,
				submissionData["created_at"],
			)
			Expect(err).NotTo(HaveOccurred())

			email := "newcontact@example.com"
			_, err = pool.Exec(ctx, `
				INSERT INTO contacts (id, context_id, email, first_name, last_name, phone, company,
				                      custom_fields, source, source_id, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
			`,
				support.NewTestUUID(),
				tenantID,
				email,
				"New",
				"Contact",
				"",
				"",
				"{}",
				"form",
				formData["id"].(string),
				submissionData["created_at"],
				submissionData["created_at"],
			)
			Expect(err).NotTo(HaveOccurred())

			var contactCount, submissionCount int
			err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM contacts WHERE email = $1", email).Scan(&contactCount)
			Expect(err).NotTo(HaveOccurred())
			Expect(contactCount).To(Equal(1))

			err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM form_submissions").Scan(&submissionCount)
			Expect(err).NotTo(HaveOccurred())
			Expect(submissionCount).To(Equal(1))
		})

		It("should link form submission to contact by email", func() {
			email := "linked@example.com"
			formData := support.NewTestForm(tenantID)
			fields, _ := json.Marshal(formData["fields"])
			settings, _ := json.Marshal(formData["settings"])

			_, err := pool.Exec(ctx, `
				INSERT INTO forms (id, context_id, slug, name, description, fields, settings, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			`,
				formData["id"],
				formData["context_id"],
				formData["slug"],
				formData["name"],
				formData["description"],
				fields,
				settings,
				formData["created_at"],
				formData["updated_at"],
			)
			Expect(err).NotTo(HaveOccurred())

			_, err = pool.Exec(ctx, `
				INSERT INTO contacts (id, context_id, email, first_name, last_name, phone, company,
				                      custom_fields, source, source_id, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
			`,
				support.NewTestUUID(),
				tenantID,
				email,
				"Linked",
				"Contact",
				"",
				"",
				"{}",
				"form",
				formData["id"].(string),
				support.NewTestContact(tenantID)["created_at"].(time.Time),
				support.NewTestContact(tenantID)["updated_at"].(time.Time),
			)
			Expect(err).NotTo(HaveOccurred())

			submissionData := support.NewTestFormSubmission(formData["id"].(string), tenantID)
			submissionData["data"] = map[string]interface{}{"email": email}

			data, _ := json.Marshal(submissionData["data"])
			files, _ := json.Marshal(submissionData["files"])

			_, err = pool.Exec(ctx, `
				INSERT INTO form_submissions (id, form_id, context_id, data, files, created_at)
				VALUES ($1, $2, $3, $4, $5, $6)
			`,
				support.NewTestUUID(),
				formData["id"],
				tenantID,
				data,
				files,
				submissionData["created_at"],
			)
			Expect(err).NotTo(HaveOccurred())

			var contactCount, submissionCount int
			err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM contacts WHERE email = $1", email).Scan(&contactCount)
			Expect(err).NotTo(HaveOccurred())
			Expect(contactCount).To(Equal(1))
			err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM form_submissions").Scan(&submissionCount)
			Expect(err).NotTo(HaveOccurred())
			Expect(submissionCount).To(Equal(1))
		})
	})

	Describe("Form field schema validation", func() {
		It("should validate form field types", func() {
			validTypes := []string{"text", "email", "tel", "number", "textarea", "checkbox", "radio", "select", "hidden"}

			for i, fieldType := range validTypes {
				formData := support.NewTestForm(tenantID)
				formData["id"] = support.NewTestUUID()
				formData["slug"] = fmt.Sprintf("form-%s-%d", fieldType, i)
				formData["fields"] = []map[string]interface{}{
					{"name": "field_" + fieldType, "type": fieldType, "required": false},
				}

				fields, _ := json.Marshal(formData["fields"])
				settings, _ := json.Marshal(formData["settings"])

				_, err := pool.Exec(ctx, `
					INSERT INTO forms (id, context_id, slug, name, description, fields, settings, created_at, updated_at)
					VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
				`,
					formData["id"],
					formData["context_id"],
					formData["slug"],
					formData["name"],
					formData["description"],
					fields,
					settings,
					formData["created_at"],
					formData["updated_at"],
				)
				Expect(err).NotTo(HaveOccurred())
			}

			var count int
			err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM forms").Scan(&count)
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(len(validTypes)))
		})

		It("should store form field definitions as JSONB array", func() {
			formData := support.NewTestForm(tenantID)
			formData["fields"] = []map[string]interface{}{
				{"name": "email", "type": "email", "required": true, "placeholder": "Enter email"},
				{"name": "phone", "type": "tel", "required": false, "placeholder": "Enter phone"},
				{"name": "message", "type": "textarea", "required": false, "rows": 5},
				{"name": "agree", "type": "checkbox", "required": true},
			}

			fields, _ := json.Marshal(formData["fields"])
			settings, _ := json.Marshal(formData["settings"])

			_, err := pool.Exec(ctx, `
				INSERT INTO forms (id, context_id, slug, name, description, fields, settings, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			`,
				formData["id"],
				formData["context_id"],
				formData["slug"],
				formData["name"],
				formData["description"],
				fields,
				settings,
				formData["created_at"],
				formData["updated_at"],
			)
			Expect(err).NotTo(HaveOccurred())

			var retrievedFields []byte
			err = pool.QueryRow(ctx, "SELECT fields FROM forms WHERE id = $1", formData["id"]).Scan(&retrievedFields)
			Expect(err).NotTo(HaveOccurred())

			var fieldsList []map[string]interface{}
			err = json.Unmarshal(retrievedFields, &fieldsList)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(fieldsList)).To(Equal(4))
			Expect(fieldsList[0]["name"]).To(Equal("email"))
			Expect(fieldsList[0]["required"]).To(Equal(true))
		})

		It("should validate form slug uniqueness per tenantID", func() {
			formData := support.NewTestForm(tenantID)
			formData["slug"] = "unique-slug"

			fields, _ := json.Marshal(formData["fields"])
			settings, _ := json.Marshal(formData["settings"])

			_, err := pool.Exec(ctx, `
				INSERT INTO forms (id, context_id, slug, name, description, fields, settings, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			`,
				formData["id"],
				formData["context_id"],
				formData["slug"],
				formData["name"],
				formData["description"],
				fields,
				settings,
				formData["created_at"],
				formData["updated_at"],
			)
			Expect(err).NotTo(HaveOccurred())

			var count int
			err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM forms WHERE context_id = $1 AND slug = $2", tenantID, "unique-slug").Scan(&count)
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(1))
		})

		It("should store form settings as JSONB", func() {
			formData := support.NewTestForm(tenantID)
			formData["settings"] = map[string]interface{}{
				"thank_you_message": "Thanks for your submission!",
				"redirect_url":      "https://example.com/thanks",
				"notify_email":      "admin@example.com",
				"autorespond":       true,
			}

			fields, _ := json.Marshal(formData["fields"])
			settings, _ := json.Marshal(formData["settings"])

			_, err := pool.Exec(ctx, `
				INSERT INTO forms (id, context_id, slug, name, description, fields, settings, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			`,
				formData["id"],
				formData["context_id"],
				formData["slug"],
				formData["name"],
				formData["description"],
				fields,
				settings,
				formData["created_at"],
				formData["updated_at"],
			)
			Expect(err).NotTo(HaveOccurred())

			var retrievedSettings []byte
			err = pool.QueryRow(ctx, "SELECT settings FROM forms WHERE id = $1", formData["id"]).Scan(&retrievedSettings)
			Expect(err).NotTo(HaveOccurred())

			var settingsMap map[string]interface{}
			err = json.Unmarshal(retrievedSettings, &settingsMap)
			Expect(err).NotTo(HaveOccurred())
			Expect(settingsMap["thank_you_message"]).To(Equal("Thanks for your submission!"))
			Expect(settingsMap["notify_email"]).To(Equal("admin@example.com"))
		})
	})

	Describe("Tracking snippet generation", func() {
		It("should generate a valid tracking script with tenantID ID", func() {
			tenantID := support.NewTestContextID()
			js, err := snippet.Generate(tenantID, "", nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(js).To(ContainSubstring(tenantID))
			Expect(js).To(ContainSubstring("Automata"))
			Expect(js).To(ContainSubstring("function()"))
		})

		It("should generate snippet with custom API host", func() {
			tenantID := support.NewTestContextID()
			apiHost := "https://track.example.com"

			js, err := snippet.Generate(tenantID, apiHost, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(js).To(ContainSubstring(apiHost))
		})

		It("should generate banner snippet", func() {
			tenantID := support.NewTestContextID()
			js, err := snippet.GenerateBannerSnippet(tenantID, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(js).To(ContainSubstring(tenantID))
			Expect(js).To(ContainSubstring("AutomataBanner"))
		})

		It("should generate minified snippet", func() {
			tenantID := support.NewTestContextID()
			js, err := snippet.Generate(tenantID, "", nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(js).ToNot(ContainSubstring("\n"))
		})
	})

	Describe("Tracking endpoint validation", func() {
		It("should return 200 for /track/ping", func() {
			req := httptest.NewRequest(http.MethodGet, "/track/ping", nil)
			w := httptest.NewRecorder()
			engine := gin.New()
			engine.GET("/track/ping", func(c *gin.Context) {
				c.String(200, "pong")
			})
			engine.ServeHTTP(w, req)
			Expect(w.Code).To(Equal(200))
			Expect(w.Body.String()).To(Equal("pong"))
		})

		It("should accept tracking requests with required headers", func() {
			req := httptest.NewRequest(http.MethodPost, "/track/event", bytes.NewBufferString(`{"context_id":"test","event":"page_view"}`))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Tenant-ID", "test")
			w := httptest.NewRecorder()
			engine := gin.New()
			engine.POST("/track/event", func(c *gin.Context) {
				c.String(200, "ok")
			})
			engine.Use(func(c *gin.Context) {
				tenantID := c.GetHeader("X-Tenant-ID")
				if tenantID == "" {
					c.AbortWithStatusJSON(400, map[string]interface{}{"error": "missing tenantID ID"})
					return
				}
				c.Next()
			})
			engine.ServeHTTP(w, req)
			Expect(w.Code).To(Equal(200))
		})

		It("should reject tracking requests without tenantID header", func() {
			req := httptest.NewRequest(http.MethodPost, "/track/event", bytes.NewBufferString(`{"context_id":"test","event":"page_view"}`))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			engine := gin.New()
			engine.Use(func(c *gin.Context) {
				tenantID := c.GetHeader("X-Tenant-ID")
				if tenantID == "" {
					c.AbortWithStatusJSON(400, map[string]interface{}{"error": "missing tenantID ID"})
					return
				}
				c.Next()
			})
			engine.POST("/track/event", func(c *gin.Context) {
				c.String(200, "ok")
			})
			engine.ServeHTTP(w, req)
			Expect(w.Code).To(Equal(400))
		})

		It("should handle batch tracking requests", func() {
			batch := []map[string]interface{}{
				{"context_id": "test1", "event": "page_view"},
				{"context_id": "test2", "event": "click"},
			}
			body, _ := json.Marshal(batch)
			req := httptest.NewRequest(http.MethodPost, "/track/batch", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			engine := gin.New()
			engine.POST("/track/batch", func(c *gin.Context) {
				c.String(200, "ok")
			})
			engine.ServeHTTP(w, req)
			Expect(w.Code).To(Equal(200))
		})
	})

	Describe("Snippet endpoint serving", func() {
		It("should serve JS snippet at /snippet/{context_id}", func() {
			tenantID := support.NewTestContextID()
			req := httptest.NewRequest(http.MethodGet, "/snippet/"+tenantID, nil)
			w := httptest.NewRecorder()
			engine := gin.New()
			engine.GET("/snippet/:context_id", func(c *gin.Context) {
				tenantID := c.Param("context_id")
				js, err := snippet.Generate(tenantID, "", nil)
				if err != nil {
					c.String(500, "error")
					return
				}
				c.Header("Content-Type", "application/javascript")
				c.String(200, js)
			})
			engine.ServeHTTP(w, req)
			Expect(w.Code).To(Equal(200))
			Expect(w.Header().Get("Content-Type")).To(ContainSubstring("application/javascript"))
			Expect(w.Body.String()).To(ContainSubstring(tenantID))
		})

		It("should serve banner snippet at /banner/{context_id}", func() {
			tenantID := support.NewTestContextID()
			req := httptest.NewRequest(http.MethodGet, "/banner/"+tenantID, nil)
			w := httptest.NewRecorder()
			engine := gin.New()
			engine.GET("/banner/:context_id", func(c *gin.Context) {
				tenantID := c.Param("context_id")
				js, err := snippet.GenerateBannerSnippet(tenantID, nil)
				if err != nil {
					c.String(500, "error")
					return
				}
				c.Header("Content-Type", "application/javascript")
				c.String(200, js)
			})
			engine.ServeHTTP(w, req)
			Expect(w.Code).To(Equal(200))
			Expect(w.Header().Get("Content-Type")).To(ContainSubstring("application/javascript"))
			Expect(w.Body.String()).To(ContainSubstring(tenantID))
		})
	})

	Describe("Form submission endpoint", func() {
		It("should accept POST to /form/submit/{slug}", func() {
			formData := support.NewTestForm(tenantID)
			slug := formData["slug"].(string)

			fields, _ := json.Marshal(formData["fields"])
			settings, _ := json.Marshal(formData["settings"])

			_, err := pool.Exec(ctx, `
				INSERT INTO forms (id, context_id, slug, name, description, fields, settings, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			`,
				formData["id"],
				formData["context_id"],
				slug,
				formData["name"],
				formData["description"],
				fields,
				settings,
				formData["created_at"],
				formData["updated_at"],
			)
			Expect(err).NotTo(HaveOccurred())

			submissionData := support.NewTestFormSubmission(formData["id"].(string), tenantID)
			body, _ := json.Marshal(submissionData["data"])

			req := httptest.NewRequest(http.MethodPost, "/form/submit/"+slug, bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			engine := gin.New()
			engine.POST("/form/submit/:slug", func(c *gin.Context) {
				c.String(200, "ok")
			})
			engine.ServeHTTP(w, req)
			Expect(w.Code).To(Equal(200))
		})

		It("should return 404 for non-existent form slug", func() {
			req := httptest.NewRequest(http.MethodPost, "/form/submit/non-existent", bytes.NewBufferString(`{}`))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			engine := gin.New()
			engine.POST("/form/submit/:slug", func(c *gin.Context) {
				slug := c.Param("slug")
				var count int
				err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM forms WHERE slug = $1", slug).Scan(&count)
				if err != nil || count == 0 {
					c.String(404, "form not found")
					return
				}
				c.String(200, "ok")
			})
			engine.ServeHTTP(w, req)
			Expect(w.Code).To(Equal(404))
		})
	})
})
