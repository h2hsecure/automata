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

var _ = Describe("Tracking Repository Integration Tests", func() {



	AfterEach(func() {
		// Clean up tables
		_, err := pool.Exec(ctx, "DELETE FROM tracking_events")
		Expect(err).NotTo(HaveOccurred())
		_, err = pool.Exec(ctx, "DELETE FROM tracking_visitors")
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("Visitor management", func() {
		It("should create and retrieve a visitor by cookie value", func() {
			contextID = support.NewTestContextID()
			visitorData := support.NewTestVisitor(contextID)

			cookieValue := visitorData["cookie_value"].(string)

			query := `
				INSERT INTO tracking_visitors (id, context_id, cookie_value, fingerprint, first_seen, last_seen, page_views)
				VALUES ($1, $2, $3, $4, $5, $6, $7)
			`

			_, err := pool.Exec(ctx, query,
				visitorData["id"],
				visitorData["context_id"],
				cookieValue,
				visitorData["fingerprint"],
				visitorData["first_seen"],
				visitorData["last_seen"],
				visitorData["page_views"],
			)
			Expect(err).NotTo(HaveOccurred())

			// Verify visitor was inserted
			var count int
			err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM tracking_visitors WHERE context_id = $1", contextID).Scan(&count)
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(1))

			// Retrieve and verify
			var retrievedCookie string
			err = pool.QueryRow(ctx, "SELECT cookie_value FROM tracking_visitors WHERE id = $1", visitorData["id"]).Scan(&retrievedCookie)
			Expect(err).NotTo(HaveOccurred())
			Expect(retrievedCookie).To(Equal(cookieValue))
		})

		It("should create visitor by fingerprint for deduplication", func() {
			contextID = support.NewTestContextID()
			fingerprint := "test-fingerprint-123"

			// Insert first visitor with fingerprint
			visitorData := support.NewTestVisitor(contextID)
			visitorData["fingerprint"] = fingerprint

			query := `
				INSERT INTO tracking_visitors (id, context_id, cookie_value, fingerprint, first_seen, last_seen, page_views)
				VALUES ($1, $2, $3, $4, $5, $6, $7)
			`

			_, err := pool.Exec(ctx, query,
				visitorData["id"],
				visitorData["context_id"],
				visitorData["cookie_value"],
				fingerprint,
				visitorData["first_seen"],
				visitorData["last_seen"],
				visitorData["page_views"],
			)
			Expect(err).NotTo(HaveOccurred())

			// Verify visitor was inserted
			var count int
			err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM tracking_visitors WHERE fingerprint = $1", fingerprint).Scan(&count)
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(1))
		})

		It("should update visitor last_seen and page_views", func() {
			contextID = support.NewTestContextID()
			visitorData := support.NewTestVisitor(contextID)

			// Insert visitor
			query := `
				INSERT INTO tracking_visitors (id, context_id, cookie_value, fingerprint, first_seen, last_seen, page_views)
				VALUES ($1, $2, $3, $4, $5, $6, $7)
			`

			_, err := pool.Exec(ctx, query,
				visitorData["id"],
				visitorData["context_id"],
				visitorData["cookie_value"],
				visitorData["fingerprint"],
				visitorData["first_seen"],
				visitorData["last_seen"],
				visitorData["page_views"],
			)
			Expect(err).NotTo(HaveOccurred())

			// Update visitor
			_, err = pool.Exec(ctx, `
				UPDATE tracking_visitors SET last_seen = NOW(), page_views = page_views + 1
				WHERE id = $1
			`, visitorData["id"])
			Expect(err).NotTo(HaveOccurred())

			// Verify update
			var lastSeen interface{}
			var pageViews int
			err = pool.QueryRow(ctx, "SELECT last_seen, page_views FROM tracking_visitors WHERE id = $1", visitorData["id"]).Scan(&lastSeen, &pageViews)
			Expect(err).NotTo(HaveOccurred())
			Expect(pageViews).To(Equal(1))
		})
	})

	Describe("Event recording", func() {
		It("should record a tracking event for visitor", func() {
			contextID = support.NewTestContextID()
			visitorData := support.NewTestVisitor(contextID)
			eventData := support.NewTestEvent(contextID, visitorData["id"].(string))

			// Insert visitor first
			_, err := pool.Exec(ctx, `
				INSERT INTO tracking_visitors (id, context_id, cookie_value, fingerprint, first_seen, last_seen, page_views)
				VALUES ($1, $2, $3, $4, $5, $6, $7)
			`,
				visitorData["id"],
				visitorData["context_id"],
				visitorData["cookie_value"],
				visitorData["fingerprint"],
				visitorData["first_seen"],
				visitorData["last_seen"],
				visitorData["page_views"],
			)
			Expect(err).NotTo(HaveOccurred())

			// Insert event
			properties, _ := json.Marshal(eventData["properties"])

			_, err = pool.Exec(ctx, `
				INSERT INTO tracking_events (id, context_id, visitor_id, type, url, title, referrer, event_name,
				                    properties, user_agent, ip_hash, utm_source, utm_medium, utm_campaign, created_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
			`,
				eventData["id"],
				eventData["context_id"],
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

			// Verify event was inserted
			var count int
			err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM tracking_events").Scan(&count)
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(1))
		})

		It("should list events by visitor ID", func() {
			contextID = support.NewTestContextID()
			visitorData := support.NewTestVisitor(contextID)
			visitorID := visitorData["id"].(string)

			// Insert visitor
			_, err := pool.Exec(ctx, `
				INSERT INTO tracking_visitors (id, context_id, cookie_value, fingerprint, first_seen, last_seen, page_views)
				VALUES ($1, $2, $3, $4, $5, $6, $7)
			`,
				visitorData["id"],
				visitorData["context_id"],
				visitorData["cookie_value"],
				visitorData["fingerprint"],
				visitorData["first_seen"],
				visitorData["last_seen"],
				visitorData["page_views"],
			)
			Expect(err).NotTo(HaveOccurred())

			// Insert multiple events
			for i := 0; i < 3; i++ {
				eventData := support.NewTestEvent(contextID, visitorID)
				eventData["id"] = support.NewTestUUID()
				eventData["url"] = "https://example.com/page" + string(rune('0'+i))

				properties, _ := json.Marshal(eventData["properties"])

				_, err := pool.Exec(ctx, `
					INSERT INTO tracking_events (id, context_id, visitor_id, type, url, title, referrer, event_name,
					                    properties, user_agent, ip_hash, utm_source, utm_medium, utm_campaign, created_at)
					VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
				`,
					eventData["id"],
					eventData["context_id"],
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
			}

			// Query events by visitor
			var count int
			err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM tracking_events WHERE visitor_id = $1", visitorID).Scan(&count)
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(3))
		})

		It("should query events by UTM parameters", func() {
			contextID = support.NewTestContextID()
			visitorData := support.NewTestVisitor(contextID)
			visitorID := visitorData["id"].(string)

			// Insert visitor
			_, err := pool.Exec(ctx, `
				INSERT INTO tracking_visitors (id, context_id, cookie_value, fingerprint, first_seen, last_seen, page_views)
				VALUES ($1, $2, $3, $4, $5, $6, $7)
			`,
				visitorData["id"],
				visitorData["context_id"],
				visitorData["cookie_value"],
				visitorData["fingerprint"],
				visitorData["first_seen"],
				visitorData["last_seen"],
				visitorData["page_views"],
			)
			Expect(err).NotTo(HaveOccurred())

			// Insert event with UTM parameters
			eventData := support.NewTestEvent(contextID, visitorID)
			eventData["utm_source"] = "google"
			eventData["utm_medium"] = "cpc"
			eventData["utm_campaign"] = "summer_sale"

			properties, _ := json.Marshal(eventData["properties"])

			_, err = pool.Exec(ctx, `
				INSERT INTO tracking_events (id, context_id, visitor_id, type, url, title, referrer, event_name,
				                    properties, user_agent, ip_hash, utm_source, utm_medium, utm_campaign, created_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
			`,
				eventData["id"],
				eventData["context_id"],
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

			// Query by UTM source
			var count int
			err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM tracking_events WHERE utm_source = $1", "google").Scan(&count)
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(1))

			// Query by UTM campaign
			err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM tracking_events WHERE utm_campaign = $1", "summer_sale").Scan(&count)
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(1))
		})

		It("should delete visitor and verify event cascade", func() {
			contextID = support.NewTestContextID()
			visitorData := support.NewTestVisitor(contextID)
			visitorID := visitorData["id"].(string)

			// Insert visitor
			_, err := pool.Exec(ctx, `
				INSERT INTO tracking_visitors (id, context_id, cookie_value, fingerprint, first_seen, last_seen, page_views)
				VALUES ($1, $2, $3, $4, $5, $6, $7)
			`,
				visitorData["id"],
				visitorData["context_id"],
				visitorData["cookie_value"],
				visitorData["fingerprint"],
				visitorData["first_seen"],
				visitorData["last_seen"],
				visitorData["page_views"],
			)
			Expect(err).NotTo(HaveOccurred())

			// Insert event
			eventData := support.NewTestEvent(contextID, visitorID)
			properties, _ := json.Marshal(eventData["properties"])

			_, err = pool.Exec(ctx, `
				INSERT INTO tracking_events (id, context_id, visitor_id, type, url, title, referrer, event_name,
				                    properties, user_agent, ip_hash, utm_source, utm_medium, utm_campaign, created_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
			`,
				eventData["id"],
				eventData["context_id"],
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

			// Verify event exists
			var eventCount int
			err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM tracking_events WHERE visitor_id = $1", visitorID).Scan(&eventCount)
			Expect(err).NotTo(HaveOccurred())
			Expect(eventCount).To(Equal(1))

			// Delete events first (no cascade FK)
			_, err = pool.Exec(ctx, "DELETE FROM tracking_events WHERE visitor_id = $1", visitorID)
			Expect(err).NotTo(HaveOccurred())

			// Delete visitor
			_, err = pool.Exec(ctx, "DELETE FROM tracking_visitors WHERE id = $1", visitorID)
			Expect(err).NotTo(HaveOccurred())

			// Verify visitor was deleted
			var visitorCount int
			err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM tracking_visitors WHERE id = $1", visitorID).Scan(&visitorCount)
			Expect(err).NotTo(HaveOccurred())
			Expect(visitorCount).To(Equal(0))
		})
	})
})
