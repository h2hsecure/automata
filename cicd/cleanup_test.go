package cicd_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// Suite-wide isolation: many specs insert rows into shared tables and ginkgo
// runs specs in random order, so exact-count assertions on the full table are
// order-dependent. Wipe the shared row tables after every spec (child tables
// first to satisfy FK constraints; ON DELETE CASCADE children are listed for
// explicitness). Specs always create their own fixtures in BeforeEach/It, so
// nothing depends on rows left behind by earlier specs.
//
// NOT wiped: users, contexts, user_contexts, context_users, email_templates
// (bootstrap assertions and seed data that specs rely on).
var _ = AfterEach(func() {
	// Contact family
	_, err := pool.Exec(ctx, "DELETE FROM contact_tag_memberships")
	Expect(err).NotTo(HaveOccurred())
	_, err = pool.Exec(ctx, "DELETE FROM contact_field_definitions")
	Expect(err).NotTo(HaveOccurred())
	_, err = pool.Exec(ctx, "DELETE FROM contact_tags")
	Expect(err).NotTo(HaveOccurred())
	_, err = pool.Exec(ctx, "DELETE FROM contact_activities")
	Expect(err).NotTo(HaveOccurred())
	_, err = pool.Exec(ctx, "DELETE FROM contact_cookie_mappings")
	Expect(err).NotTo(HaveOccurred())
	_, err = pool.Exec(ctx, "DELETE FROM contacts")
	Expect(err).NotTo(HaveOccurred())
	// Form family (form_submissions has no ON DELETE CASCADE to forms)
	_, err = pool.Exec(ctx, "DELETE FROM form_submissions")
	Expect(err).NotTo(HaveOccurred())
	_, err = pool.Exec(ctx, "DELETE FROM forms")
	Expect(err).NotTo(HaveOccurred())
	// Banner family (impressions/clicks cascade from banner_banners)
	_, err = pool.Exec(ctx, "DELETE FROM banner_impressions")
	Expect(err).NotTo(HaveOccurred())
	_, err = pool.Exec(ctx, "DELETE FROM banner_clicks")
	Expect(err).NotTo(HaveOccurred())
	_, err = pool.Exec(ctx, "DELETE FROM banner_banners")
	Expect(err).NotTo(HaveOccurred())
	_, err = pool.Exec(ctx, "DELETE FROM banner_campaigns")
	Expect(err).NotTo(HaveOccurred())
	_, err = pool.Exec(ctx, "DELETE FROM banner_placements")
	Expect(err).NotTo(HaveOccurred())
	// Tracking and async work tables
	_, err = pool.Exec(ctx, "DELETE FROM tracking_events")
	Expect(err).NotTo(HaveOccurred())
	_, err = pool.Exec(ctx, "DELETE FROM email_jobs")
	Expect(err).NotTo(HaveOccurred())
	_, err = pool.Exec(ctx, "DELETE FROM email_history")
	Expect(err).NotTo(HaveOccurred())
	_, err = pool.Exec(ctx, "DELETE FROM webhook_deliveries")
	Expect(err).NotTo(HaveOccurred())
})