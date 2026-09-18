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

var _ = Describe("API Handler Integration Tests", func() {



	AfterEach(func() {
		// Clean up tables (in reverse dependency order)
		_, err := pool.Exec(ctx, "DELETE FROM banner_clicks")
		Expect(err).NotTo(HaveOccurred())
		_, err = pool.Exec(ctx, "DELETE FROM banner_impressions")
		Expect(err).NotTo(HaveOccurred())
		_, err = pool.Exec(ctx, "DELETE FROM banners")
		Expect(err).NotTo(HaveOccurred())
		_, err = pool.Exec(ctx, "DELETE FROM form_submissions")
		Expect(err).NotTo(HaveOccurred())
		_, err = pool.Exec(ctx, "DELETE FROM forms")
		Expect(err).NotTo(HaveOccurred())
		Expect(err).NotTo(HaveOccurred())
		_, err = pool.Exec(ctx, "DELETE FROM contacts")
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("Contact API endpoints", func() {
		It("should create a contact via POST /api/contacts", func() {
			tenant = support.NewTestTenantID()
			contactData := support.NewTestContact(tenant)

			email := contactData["email"].(string)

			_, err := pool.Exec(ctx, `
				INSERT INTO contacts (id, tenant_id, email, first_name, last_name, phone, company,
				                      custom_fields, source, source_id, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
			`,
				contactData["id"],
				contactData["tenant_id"],
				email,
				contactData["first_name"],
				contactData["last_name"],
				contactData["phone"],
				contactData["company"],
				contactData["custom_fields"],
				contactData["source"],
				contactData["source_id"],
				contactData["created_at"],
				contactData["updated_at"],
			)
			Expect(err).NotTo(HaveOccurred())

			// Verify contact was created
			var count int
			err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM contacts").Scan(&count)
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(1))
		})

		It("should retrieve a contact via GET /api/contacts/:id", func() {
			tenant = support.NewTestTenantID()
			contactData := support.NewTestContact(tenant)

			email := contactData["email"].(string)

			_, err := pool.Exec(ctx, `
				INSERT INTO contacts (id, tenant_id, email, first_name, last_name, phone, company,
				                      custom_fields, source, source_id, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
			`,
				contactData["id"],
				contactData["tenant_id"],
				email,
				contactData["first_name"],
				contactData["last_name"],
				contactData["phone"],
				contactData["company"],
				contactData["custom_fields"],
				contactData["source"],
				contactData["source_id"],
				contactData["created_at"],
				contactData["updated_at"],
			)
			Expect(err).NotTo(HaveOccurred())

			// Verify contact can be retrieved
			var retrievedID string
			err = pool.QueryRow(ctx, "SELECT id FROM contacts WHERE id = $1", contactData["id"]).Scan(&retrievedID)
			Expect(err).NotTo(HaveOccurred())
			Expect(retrievedID).To(Equal(contactData["id"]))
		})

		It("should update a contact via PUT /api/contacts/:id", func() {
			tenant = support.NewTestTenantID()
			contactData := support.NewTestContact(tenant)

			email := contactData["email"].(string)

			_, err := pool.Exec(ctx, `
				INSERT INTO contacts (id, tenant_id, email, first_name, last_name, phone, company,
				                      custom_fields, source, source_id, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
			`,
				contactData["id"],
				contactData["tenant_id"],
				email,
				contactData["first_name"],
				contactData["last_name"],
				contactData["phone"],
				contactData["company"],
				contactData["custom_fields"],
				contactData["source"],
				contactData["source_id"],
				contactData["created_at"],
				contactData["updated_at"],
			)
			Expect(err).NotTo(HaveOccurred())

			// Update contact
			_, err = pool.Exec(ctx, `
				UPDATE contacts SET first_name = $1, last_name = $2, company = $3
				WHERE id = $4
			`, "Updated", "Name", "Updated Corp", contactData["id"])
			Expect(err).NotTo(HaveOccurred())

			// Verify update
			var firstName, lastName, company string
			err = pool.QueryRow(ctx, "SELECT first_name, last_name, company FROM contacts WHERE id = $1", contactData["id"]).Scan(&firstName, &lastName, &company)
			Expect(err).NotTo(HaveOccurred())
			Expect(firstName).To(Equal("Updated"))
			Expect(lastName).To(Equal("Name"))
			Expect(company).To(Equal("Updated Corp"))
		})

		It("should delete a contact via DELETE /api/contacts/:id", func() {
			tenant = support.NewTestTenantID()
			contactData := support.NewTestContact(tenant)

			email := contactData["email"].(string)

			_, err := pool.Exec(ctx, `
				INSERT INTO contacts (id, tenant_id, email, first_name, last_name, phone, company,
				                      custom_fields, source, source_id, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
			`,
				contactData["id"],
				contactData["tenant_id"],
				email,
				contactData["first_name"],
				contactData["last_name"],
				contactData["phone"],
				contactData["company"],
				contactData["custom_fields"],
				contactData["source"],
				contactData["source_id"],
				contactData["created_at"],
				contactData["updated_at"],
			)
			Expect(err).NotTo(HaveOccurred())

			// Delete contact
			_, err = pool.Exec(ctx, "DELETE FROM contacts WHERE id = $1", contactData["id"])
			Expect(err).NotTo(HaveOccurred())

			// Verify contact was deleted
			var count int
			err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM contacts WHERE id = $1", contactData["id"]).Scan(&count)
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(0))
		})

		It("should list contacts with pagination via GET /api/contacts", func() {
			tenant = support.NewTestTenantID()

			// Insert multiple contacts
			for i := 0; i < 5; i++ {
				contactData := support.NewTestContact(tenant)
				contactData["email"] = "user" + string(rune('0'+i)) + "@example.com"

				customFields, _ := json.Marshal(contactData["custom_fields"])

				_, err := pool.Exec(ctx, `
					INSERT INTO contacts (id, tenant_id, email, first_name, last_name, phone, company,
					                      custom_fields, source, source_id, created_at, updated_at)
					VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
				`,
					contactData["id"],
					contactData["tenant_id"],
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
			}

			// Verify total count
			var totalCount int
			err := pool.QueryRow(ctx, `
				SELECT COUNT(*) FROM contacts
			`).Scan(&totalCount)
			Expect(err).NotTo(HaveOccurred())
			Expect(totalCount).To(Equal(5))
		})
	})

	Describe("Form API endpoints", func() {
		It("should submit a form via POST /api/forms/:slug/submit", func() {
			tenant = support.NewTestTenantID()
			formData := support.NewTestForm(tenant)

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

			// Submit form (insert submission)
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

			// Verify submission was created
			var count int
			err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM form_submissions").Scan(&count)
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(1))
		})

		It("should retrieve a form via GET /api/forms/:slug", func() {
			tenant = support.NewTestTenantID()
			formData := support.NewTestForm(tenant)

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

			// Verify form can be retrieved
			var retrievedSlug string
			err = pool.QueryRow(ctx, "SELECT slug FROM forms WHERE id = $1", formData["id"]).Scan(&retrievedSlug)
			Expect(err).NotTo(HaveOccurred())
			Expect(retrievedSlug).To(Equal("test-form"))
		})
	})

	Describe("Tracking API endpoints", func() {
		It("should record a tracking event via POST /api/tracking", func() {
			tenant = support.NewTestTenantID()
			visitorData := support.NewTestVisitor(tenant)
			visitorID := visitorData["id"].(string)

			// Insert visitor first
			_, err := pool.Exec(ctx, `
				INSERT INTO visitors (id, tenant_id, cookie_value, fingerprint, first_seen, last_seen, page_views)
				VALUES ($1, $2, $3, $4, $5, $6, $7)
			`,
				visitorData["id"],
				visitorData["tenant_id"],
				visitorData["cookie_value"],
				visitorData["fingerprint"],
				visitorData["first_seen"],
				visitorData["last_seen"],
				visitorData["page_views"],
			)
			Expect(err).NotTo(HaveOccurred())

			// Record tracking event
			eventData := support.NewTestEvent(tenant, visitorID)
			properties, _ := json.Marshal(eventData["properties"])

			_, err = pool.Exec(ctx, `
				INSERT INTO events (id, tenant_id, visitor_id, type, url, title, referrer, event_name,
				                    properties, user_agent, ip_hash, utm_source, utm_medium, utm_campaign, created_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
			`,
				eventData["id"],
				eventData["tenant_id"],
				eventData["visitor_id"],
				eventData["type"],
				eventData["url"],
				eventData["title"],
				eventData["referrer"],
				eventData["event_name"],
				properties,
				eventData["user_agent"],
				eventData["ip_hash"],
				eventData["utm_source"],
				eventData["utm_medium"],
				eventData["utm_campaign"],
				eventData["created_at"],
			)
			Expect(err).NotTo(HaveOccurred())

			// Verify event was recorded
			var count int
			err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM events").Scan(&count)
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(1))
		})
	})

	Describe("Banner API endpoints", func() {
		It("should get active banners via GET /api/banners/:placement", func() {
			tenant = support.NewTestTenantID()
			bannerData := support.NewTestBanner(tenant)

			// Insert banner first
			placements, _ := json.Marshal(bannerData["placements"])
			abVariants, _ := json.Marshal(bannerData["ab_variants"])

			_, err := pool.Exec(ctx, `
				INSERT INTO banners (id, tenant_id, name, type, content, link_url, image_url, alt_text,
				                     campaign_id, placements, priority, start_date, end_date, is_active, 
				                     ab_test, ab_variants, impressions, clicks, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20)
			`,
				bannerData["id"],
				bannerData["tenant_id"],
				bannerData["name"],
				bannerData["type"],
				bannerData["content"],
				bannerData["link_url"],
				bannerData["image_url"],
				bannerData["alt_text"],
				bannerData["campaign_id"],
				placements,
				bannerData["priority"],
				bannerData["start_date"],
				bannerData["end_date"],
				bannerData["is_active"],
				bannerData["ab_test"],
				abVariants,
				bannerData["impressions"],
				bannerData["clicks"],
				bannerData["created_at"],
				bannerData["updated_at"],
			)
			Expect(err).NotTo(HaveOccurred())

			// Verify banner can be retrieved by placement
			var count int
			err = pool.QueryRow(ctx, `
				SELECT COUNT(*) FROM banners 
				WHERE tenant_id = $1 AND placements @> '["header"]'::jsonb
			`, tenant).Scan(&count)
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(1))
		})
	})
})
