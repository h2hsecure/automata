package cicd_test

import (
	"context"
	"encoding/json"

	"github.com/hekemen/automata/cicd/support"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var (
)

var _ = BeforeEach(func() {
	ctx = context.Background()
	pool = db.Pool
	tenant = support.NewTestTenantID()
})

var _ = Describe("Form Repository Integration Tests", func() {



	AfterEach(func() {
		// Clean up tables
		_, err := pool.Exec(ctx, "DELETE FROM form_submissions")
		Expect(err).NotTo(HaveOccurred())
		_, err = pool.Exec(ctx, "DELETE FROM forms")
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("Form CRUD", func() {
		It("should create and retrieve a form", func() {
			tenant = support.NewTestTenantID()
			formData := support.NewTestForm(tenant)

			fields, _ := json.Marshal(formData["fields"])
			settings, _ := json.Marshal(formData["settings"])

			query := `
				INSERT INTO forms (id, tenant_id, slug, name, description, fields, settings, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			`

			_, err := pool.Exec(ctx, query,
				formData["id"],
				formData["tenant_id"],
				formData["slug"],
				formData["name"],
				formData["description"],
				fields,
				settings,
				formData["created_at"],
				formData["updated_at"],
			)
			Expect(err).NotTo(HaveOccurred())

			// Verify form was inserted
			var count int
			err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM forms").Scan(&count)
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(1))

			// Retrieve and verify
			var retrievedSlug, retrievedName string
			err = pool.QueryRow(ctx, "SELECT slug, name FROM forms WHERE id = $1", formData["id"]).Scan(&retrievedSlug, &retrievedName)
			Expect(err).NotTo(HaveOccurred())
			Expect(retrievedSlug).To(Equal("test-form"))
			Expect(retrievedName).To(Equal("Test Form"))
		})

		It("should update form fields and settings", func() {
			tenant = support.NewTestTenantID()
			formData := support.NewTestForm(tenant)

			// Insert form
			fields, _ := json.Marshal(formData["fields"])
			settings, _ := json.Marshal(formData["settings"])

			_, err := pool.Exec(ctx, `
				INSERT INTO forms (id, tenant_id, slug, name, description, fields, settings, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			`,
				formData["id"],
				formData["tenant_id"],
				formData["slug"],
				formData["name"],
				formData["description"],
				fields,
				settings,
				formData["created_at"],
				formData["updated_at"],
			)
			Expect(err).NotTo(HaveOccurred())

			// Update form
			newFields := []map[string]interface{}{{"name": "email", "type": "email", "required": true}, {"name": "phone", "type": "tel"}}
			newFieldsBytes, _ := json.Marshal(newFields)

			_, err = pool.Exec(ctx, `
				UPDATE forms SET name = $1, fields = $2, settings = $3
				WHERE id = $4
			`, "Updated Form", newFieldsBytes, "{}", formData["id"])
			Expect(err).NotTo(HaveOccurred())

			// Verify update
			var name string
			var retrievedFields []byte
			err = pool.QueryRow(ctx, "SELECT name, fields FROM forms WHERE id = $1", formData["id"]).Scan(&name, &retrievedFields)
			Expect(err).NotTo(HaveOccurred())
			Expect(name).To(Equal("Updated Form"))

			var fieldsList []map[string]interface{}
			err = json.Unmarshal(retrievedFields, &fieldsList)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(fieldsList)).To(Equal(2))
		})

		It("should delete a form", func() {
			tenant = support.NewTestTenantID()
			formData := support.NewTestForm(tenant)

			// Insert form
			fields, _ := json.Marshal(formData["fields"])
			settings, _ := json.Marshal(formData["settings"])

			_, err := pool.Exec(ctx, `
				INSERT INTO forms (id, tenant_id, slug, name, description, fields, settings, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			`,
				formData["id"],
				formData["tenant_id"],
				formData["slug"],
				formData["name"],
				formData["description"],
				fields,
				settings,
				formData["created_at"],
				formData["updated_at"],
			)
			Expect(err).NotTo(HaveOccurred())

			// Delete form
			_, err = pool.Exec(ctx, "DELETE FROM forms WHERE id = $1", formData["id"])
			Expect(err).NotTo(HaveOccurred())

			// Verify deletion
			var count int
			err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM forms").Scan(&count)
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(0))
		})

		It("should list forms by tenant", func() {
			tenant = support.NewTestTenantID()

			// Insert multiple forms
			for i := 0; i < 5; i++ {
				formData := support.NewTestForm(tenant)
				formData["slug"] = "form-" + string(rune('0'+i))
				formData["name"] = "Form " + string(rune('0'+i))

				fields, _ := json.Marshal(formData["fields"])
				settings, _ := json.Marshal(formData["settings"])

				_, err := pool.Exec(ctx, `
					INSERT INTO forms (id, tenant_id, slug, name, description, fields, settings, created_at, updated_at)
					VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
				`,
					formData["id"],
					formData["tenant_id"],
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

			// Query forms by tenant
			var count int
			err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM forms WHERE tenant_id = $1", tenant).Scan(&count)
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(5))
		})
	})

	Describe("Form submissions", func() {
		It("should create a form submission", func() {
			tenant = support.NewTestTenantID()
			formData := support.NewTestForm(tenant)
			submissionData := support.NewTestFormSubmission(formData["id"].(string), tenant)

			// Insert form first
			fields, _ := json.Marshal(formData["fields"])
			settings, _ := json.Marshal(formData["settings"])

			_, err := pool.Exec(ctx, `
				INSERT INTO forms (id, tenant_id, slug, name, description, fields, settings, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			`,
				formData["id"],
				formData["tenant_id"],
				formData["slug"],
				formData["name"],
				formData["description"],
				fields,
				settings,
				formData["created_at"],
				formData["updated_at"],
			)
			Expect(err).NotTo(HaveOccurred())

			// Insert submission
			data, _ := json.Marshal(submissionData["data"])
			files, _ := json.Marshal(submissionData["files"])

			_, err = pool.Exec(ctx, `
				INSERT INTO form_submissions (id, form_id, tenant_id, data, files, created_at)
				VALUES ($1, $2, $3, $4, $5, $6)
			`,
				submissionData["id"],
				submissionData["form_id"],
				submissionData["tenant_id"],
				data,
				files,
				submissionData["created_at"],
			)
			Expect(err).NotTo(HaveOccurred())

			// Verify submission was inserted
			var count int
			err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM form_submissions").Scan(&count)
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(1))
		})

		It("should retrieve form submissions with pagination", func() {
			tenant = support.NewTestTenantID()
			formData := support.NewTestForm(tenant)

			// Insert form
			fields, _ := json.Marshal(formData["fields"])
			settings, _ := json.Marshal(formData["settings"])

			_, err := pool.Exec(ctx, `
				INSERT INTO forms (id, tenant_id, slug, name, description, fields, settings, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			`,
				formData["id"],
				formData["tenant_id"],
				formData["slug"],
				formData["name"],
				formData["description"],
				fields,
				settings,
				formData["created_at"],
				formData["updated_at"],
			)
			Expect(err).NotTo(HaveOccurred())

			// Insert multiple submissions
			for i := 0; i < 10; i++ {
				submissionData := support.NewTestFormSubmission(formData["id"].(string), tenant)
				submissionData["id"] = support.NewTestUUID()

				data, _ := json.Marshal(submissionData["data"])
				files, _ := json.Marshal(submissionData["files"])

				_, err := pool.Exec(ctx, `
					INSERT INTO form_submissions (id, form_id, tenant_id, data, files, created_at)
					VALUES ($1, $2, $3, $4, $5, $6)
				`,
					submissionData["id"],
					submissionData["form_id"],
					submissionData["tenant_id"],
					data,
					files,
					submissionData["created_at"],
				)
				Expect(err).NotTo(HaveOccurred())
			}

			// Verify total count
			var totalCount int
			err = pool.QueryRow(ctx, `
				SELECT COUNT(*) FROM form_submissions 
				WHERE form_id = $1
			`, formData["id"]).Scan(&totalCount)
			Expect(err).NotTo(HaveOccurred())
			Expect(totalCount).To(Equal(10))
		})

		It("should validate form submission data against schema", func() {
			tenant = support.NewTestTenantID()
			formData := support.NewTestForm(tenant)

			// Insert form
			fields, _ := json.Marshal(formData["fields"])
			settings, _ := json.Marshal(formData["settings"])

			_, err := pool.Exec(ctx, `
				INSERT INTO forms (id, tenant_id, slug, name, description, fields, settings, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			`,
				formData["id"],
				formData["tenant_id"],
				formData["slug"],
				formData["name"],
				formData["description"],
				fields,
				settings,
				formData["created_at"],
				formData["updated_at"],
			)
			Expect(err).NotTo(HaveOccurred())

			// Insert submission with valid data
			submissionData := support.NewTestFormSubmission(formData["id"].(string), tenant)
			data, _ := json.Marshal(submissionData["data"])
			files, _ := json.Marshal(submissionData["files"])

			_, err = pool.Exec(ctx, `
				INSERT INTO form_submissions (id, form_id, tenant_id, data, files, created_at)
				VALUES ($1, $2, $3, $4, $5, $6)
			`,
				submissionData["id"],
				submissionData["form_id"],
				submissionData["tenant_id"],
				data,
				files,
				submissionData["created_at"],
			)
			Expect(err).NotTo(HaveOccurred())

			// Verify submission was inserted
			var count int
			err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM form_submissions").Scan(&count)
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(1))
		})

		It("should handle file attachments in submissions", func() {
			tenant = support.NewTestTenantID()
			formData := support.NewTestForm(tenant)

			// Insert form
			fields, _ := json.Marshal(formData["fields"])
			settings, _ := json.Marshal(formData["settings"])

			_, err := pool.Exec(ctx, `
				INSERT INTO forms (id, tenant_id, slug, name, description, fields, settings, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			`,
				formData["id"],
				formData["tenant_id"],
				formData["slug"],
				formData["name"],
				formData["description"],
				fields,
				settings,
				formData["created_at"],
				formData["updated_at"],
			)
			Expect(err).NotTo(HaveOccurred())

			// Insert submission with file attachments
			submissionData := support.NewTestFormSubmission(formData["id"].(string), tenant)
			submissionData["files"] = []map[string]interface{}{
				{"name": "document.pdf", "size": 1024, "url": "/uploads/document.pdf"},
			}

			data, _ := json.Marshal(submissionData["data"])
			files, _ := json.Marshal(submissionData["files"])

			_, err = pool.Exec(ctx, `
				INSERT INTO form_submissions (id, form_id, tenant_id, data, files, created_at)
				VALUES ($1, $2, $3, $4, $5, $6)
			`,
				submissionData["id"],
				submissionData["form_id"],
				submissionData["tenant_id"],
				data,
				files,
				submissionData["created_at"],
			)
			Expect(err).NotTo(HaveOccurred())

			// Verify files were stored
			var retrievedFiles []byte
			err = pool.QueryRow(ctx, "SELECT files FROM form_submissions WHERE id = $1", submissionData["id"]).Scan(&retrievedFiles)
			Expect(err).NotTo(HaveOccurred())

			var filesList []map[string]interface{}
			err = json.Unmarshal(retrievedFiles, &filesList)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(filesList)).To(Equal(1))
			Expect(filesList[0]["name"]).To(Equal("document.pdf"))
		})
	})
})
