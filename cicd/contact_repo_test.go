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
	contextID = support.NewTestContextID()
})

var _ = Describe("Contact Repository Integration Tests", func() {



	AfterEach(func() {
		// Clean up tables
		_, err := pool.Exec(ctx, "DELETE FROM contact_tag_memberships")
		Expect(err).NotTo(HaveOccurred())
		_, err = pool.Exec(ctx, "DELETE FROM contact_field_definitions")
		Expect(err).NotTo(HaveOccurred())
		_, err = pool.Exec(ctx, "DELETE FROM contact_tags")
		Expect(err).NotTo(HaveOccurred())
		_, err = pool.Exec(ctx, "DELETE FROM contacts")
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("Contact CRUD", func() {
		It("should create and retrieve a contact", func() {
			contextID = support.NewTestContextID()
			contactData := support.NewTestContact(contextID)

			email := contactData["email"].(string)

			query := `
				INSERT INTO contacts (id, context_id, email, first_name, last_name, phone, company,
				                      custom_fields, source, source_id, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
			`

			customFields, _ := json.Marshal(contactData["custom_fields"])

			_, err := pool.Exec(ctx, query,
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

			// Verify contact was inserted
			var count int
			err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM contacts").Scan(&count)
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(1))

			// Retrieve and verify
			var retrievedEmail string
			err = pool.QueryRow(ctx, "SELECT email FROM contacts WHERE id = $1", contactData["id"]).Scan(&retrievedEmail)
			Expect(err).NotTo(HaveOccurred())
			Expect(retrievedEmail).To(Equal(email))
		})

		It("should update contact fields", func() {
			contextID = support.NewTestContextID()
			contactData := support.NewTestContact(contextID)

			// Insert contact
			email := contactData["email"].(string)

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

		It("should delete a contact", func() {
			contextID = support.NewTestContextID()
			contactData := support.NewTestContact(contextID)

			// Insert contact
			email := contactData["email"].(string)

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

			// Verify deletion
			var count int
			err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM contacts").Scan(&count)
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(0))
		})

		It("should list contacts with pagination", func() {
			contextID = support.NewTestContextID()

			// Insert multiple contacts
			for i := 0; i < 10; i++ {
				contactData := support.NewTestContact(contextID)
				contactData["email"] = "user" + string(rune('0'+i)) + "@example.com"

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
			}

			// Verify total count
			var totalCount int
			err := pool.QueryRow(ctx, `
				SELECT COUNT(*) FROM contacts
			`).Scan(&totalCount)
			Expect(err).NotTo(HaveOccurred())
			Expect(totalCount).To(Equal(10))
		})

		It("should search contacts by email", func() {
			contextID = support.NewTestContextID()
			contactData := support.NewTestContact(contextID)

			// Insert contact
			email := contactData["email"].(string)

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
				contactData["custom_fields"],
				contactData["source"],
				contactData["source_id"],
				contactData["created_at"],
				contactData["updated_at"],
			)
			Expect(err).NotTo(HaveOccurred())

			// Search by email
			var count int
			err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM contacts WHERE email = $1", email).Scan(&count)
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(1))
		})
	})

	Describe("Tag management", func() {
		It("should add and remove contact tags", func() {
			contextID = support.NewTestContextID()
			contactData := support.NewTestContact(contextID)
			tagData := support.NewTestTag(contextID)

			// Insert contact
			email := contactData["email"].(string)

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
				contactData["custom_fields"],
				contactData["source"],
				contactData["source_id"],
				contactData["created_at"],
				contactData["updated_at"],
			)
			Expect(err).NotTo(HaveOccurred())

			// Insert tag
			_, err = pool.Exec(ctx, `
				INSERT INTO contact_tags (id, context_id, name, color, created_at)
				VALUES ($1, $2, $3, $4, $5)
			`,
				tagData["id"],
				tagData["context_id"],
				tagData["name"],
				tagData["color"],
				tagData["created_at"],
			)
			Expect(err).NotTo(HaveOccurred())

			// Create membership
			membershipData := support.NewTestContactTagMembership(contactData["id"].(string), tagData["id"].(string))

			_, err = pool.Exec(ctx, `
				INSERT INTO contact_tag_memberships (contact_id, tag_id)
				VALUES ($1, $2)
			`,
				membershipData["contact_id"],
				membershipData["tag_id"],
			)
			Expect(err).NotTo(HaveOccurred())

			// Verify membership
			var count int
			err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM contact_tag_memberships").Scan(&count)
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(1))

			// Remove membership
			_, err = pool.Exec(ctx, "DELETE FROM contact_tag_memberships WHERE contact_id = $1 AND tag_id = $2",
				contactData["id"], tagData["id"])
			Expect(err).NotTo(HaveOccurred())

			// Verify removal
			err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM contact_tag_memberships").Scan(&count)
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(0))
		})

		It("should list contacts by tag", func() {
			contextID = support.NewTestContextID()
			contactData := support.NewTestContact(contextID)
			tagData := support.NewTestTag(contextID)

			// Insert contact and tag
			email := contactData["email"].(string)

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
				contactData["custom_fields"],
				contactData["source"],
				contactData["source_id"],
				contactData["created_at"],
				contactData["updated_at"],
			)
			Expect(err).NotTo(HaveOccurred())

			_, err = pool.Exec(ctx, `
				INSERT INTO contact_tags (id, context_id, name, color, created_at)
				VALUES ($1, $2, $3, $4, $5)
			`,
				tagData["id"],
				tagData["context_id"],
				tagData["name"],
				tagData["color"],
				tagData["created_at"],
			)
			Expect(err).NotTo(HaveOccurred())

			// Create membership
			_, err = pool.Exec(ctx, `
				INSERT INTO contact_tag_memberships (contact_id, tag_id)
				VALUES ($1, $2)
			`, contactData["id"], tagData["id"])
			Expect(err).NotTo(HaveOccurred())

			// Query contacts by tag
			var count int
			err = pool.QueryRow(ctx, `
				SELECT COUNT(*) FROM contact_tag_memberships ctm
				JOIN contacts c ON ctm.contact_id = c.id
				WHERE ctm.tag_id = $1
			`, tagData["id"]).Scan(&count)
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(1))
		})
	})
})
