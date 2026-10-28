package cicd_test

import (
	"github.com/h2hsecure/automata/cicd/support"
	crepo "github.com/h2hsecure/automata/internal/infrastructure/contact/repo"
	trepo "github.com/h2hsecure/automata/internal/infrastructure/tracking/repo"
	formuc "github.com/h2hsecure/automata/internal/usecase/form"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Visitor Contact Uplift", func() {
	var contextID string

	BeforeEach(func() {
		contextID = support.NewTestContextID()
	})

	AfterEach(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM contact_activities WHERE context_id = $1", contextID)
		_, _ = pool.Exec(ctx, "DELETE FROM contacts WHERE context_id = $1", contextID)
		_, _ = pool.Exec(ctx, "DELETE FROM tracking_visitors WHERE context_id = $1", contextID)
		_, _ = pool.Exec(ctx, "DELETE FROM tracking_events WHERE context_id = $1", contextID)
	})

	It("should unhide and fill the hidden contact when there is no email collision", func() {
		hiddenID := support.NewTestUUID()
		_, err := pool.Exec(ctx, `
			INSERT INTO contacts (id, context_id, source, source_id, hidden)
			VALUES ($1, $2, 'visitor', 'v-up-1', true)
		`, hiddenID, contextID)
		Expect(err).NotTo(HaveOccurred())
		_, err = pool.Exec(ctx, `
			INSERT INTO tracking_visitors (id, context_id, cookie_value, contact_id)
			VALUES ($1, $2, 'v-up-1', $3)
		`, support.NewTestUUID(), contextID, hiddenID)
		Expect(err).NotTo(HaveOccurred())

		contact, err := formuc.IdentifyVisitor(crepo.New(pool), trepo.New(pool), contextID, "demo-form", "v-up-1", map[string]interface{}{
			"email":      "ann@example.com",
			"first_name": "Ann",
			"last_name":  "Smith",
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(contact).NotTo(BeNil())
		Expect(contact.ID).To(Equal(hiddenID))

		var hidden bool
		var source, firstName string
		var email *string
		err = pool.QueryRow(ctx, "SELECT hidden, source, email, first_name FROM contacts WHERE id = $1", hiddenID).Scan(&hidden, &source, &email, &firstName)
		Expect(err).NotTo(HaveOccurred())
		Expect(hidden).To(BeFalse())
		Expect(source).To(Equal("form:demo-form"))
		Expect(email).NotTo(BeNil())
		Expect(*email).To(Equal("ann@example.com"))
		Expect(firstName).To(Equal("Ann"))

		// The visitor link stays on the uplifted contact.
		var visitorContactID string
		err = pool.QueryRow(ctx, "SELECT contact_id FROM tracking_visitors WHERE context_id = $1 AND cookie_value = 'v-up-1'", contextID).Scan(&visitorContactID)
		Expect(err).NotTo(HaveOccurred())
		Expect(visitorContactID).To(Equal(hiddenID))
	})

	It("should merge the hidden contact into the existing contact on an email collision", func() {
		hiddenID := support.NewTestUUID()
		existingID := support.NewTestUUID()
		_, err := pool.Exec(ctx, `
			INSERT INTO contacts (id, context_id, source, source_id, hidden)
			VALUES ($1, $2, 'visitor', 'v-up-2', true)
		`, hiddenID, contextID)
		Expect(err).NotTo(HaveOccurred())
		_, err = pool.Exec(ctx, `
			INSERT INTO contacts (id, context_id, email, source, hidden)
			VALUES ($1, $2, 'ann@example.com', 'form:other', false)
		`, existingID, contextID)
		Expect(err).NotTo(HaveOccurred())
		_, err = pool.Exec(ctx, `
			INSERT INTO tracking_visitors (id, context_id, cookie_value, contact_id)
			VALUES ($1, $2, 'v-up-2', $3)
		`, support.NewTestUUID(), contextID, hiddenID)
		Expect(err).NotTo(HaveOccurred())

		// A page_visit activity on the hidden contact must move with the merge.
		activityID := support.NewTestUUID()
		_, err = pool.Exec(ctx, `
			INSERT INTO contact_activities (id, contact_id, context_id, type, data, source_id)
			VALUES ($1, $2, $3, 'page_visit', '{}'::jsonb, '')
		`, activityID, hiddenID, contextID)
		Expect(err).NotTo(HaveOccurred())

		contact, err := formuc.IdentifyVisitor(crepo.New(pool), trepo.New(pool), contextID, "demo-form", "v-up-2", map[string]interface{}{
			"email": "ann@example.com",
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(contact).NotTo(BeNil())
		Expect(contact.ID).To(Equal(existingID))

		// The hidden contact is deleted; the existing contact remains.
		var hiddenCount, existingCount int
		err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM contacts WHERE id = $1", hiddenID).Scan(&hiddenCount)
		Expect(err).NotTo(HaveOccurred())
		Expect(hiddenCount).To(Equal(0))
		err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM contacts WHERE id = $1", existingID).Scan(&existingCount)
		Expect(err).NotTo(HaveOccurred())
		Expect(existingCount).To(Equal(1))

		// The activity was reassigned to the existing contact.
		var activityContactID string
		err = pool.QueryRow(ctx, "SELECT contact_id FROM contact_activities WHERE id = $1", activityID).Scan(&activityContactID)
		Expect(err).NotTo(HaveOccurred())
		Expect(activityContactID).To(Equal(existingID))

		// The visitor link was repointed to the existing contact.
		var visitorContactID string
		err = pool.QueryRow(ctx, "SELECT contact_id FROM tracking_visitors WHERE context_id = $1 AND cookie_value = 'v-up-2'", contextID).Scan(&visitorContactID)
		Expect(err).NotTo(HaveOccurred())
		Expect(visitorContactID).To(Equal(existingID))
	})
})