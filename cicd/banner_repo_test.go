package cicd_test

import (
	"context"
	"encoding/json"
	"time"

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

var _ = Describe("Banner Repository Integration Tests", func() {



	AfterEach(func() {
		// Clean up tables
		_, err := pool.Exec(ctx, "DELETE FROM banner_clicks")
		Expect(err).NotTo(HaveOccurred())
		_, err = pool.Exec(ctx, "DELETE FROM banner_impressions")
		Expect(err).NotTo(HaveOccurred())
		_, err = pool.Exec(ctx, "DELETE FROM banner_banners")
		Expect(err).NotTo(HaveOccurred())
		_, err = pool.Exec(ctx, "DELETE FROM banner_campaigns")
		Expect(err).NotTo(HaveOccurred())
		_, err = pool.Exec(ctx, "DELETE FROM banner_placements")
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("Banner campaign CRUD", func() {
		It("should create and retrieve a banner campaign", func() {
			contextID = support.NewTestContextID()
			campaignData := support.NewTestCampaign(contextID)

			_, err := pool.Exec(ctx, `
				INSERT INTO banner_campaigns (id, context_id, name, description, start_date, end_date, is_active, 
				                       target_url, tracking_code, impressions, clicks, conversion_rate, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
			`,
				campaignData["id"],
				campaignData["context_id"],
				campaignData["name"],
				campaignData["description"],
				campaignData["start_date"],
				campaignData["end_date"],
				campaignData["is_active"],
				campaignData["target_url"],
				campaignData["tracking_code"],
				campaignData["impressions"],
				campaignData["clicks"],
				campaignData["conversion_rate"],
				campaignData["created_at"],
				campaignData["updated_at"],
			)
			Expect(err).NotTo(HaveOccurred())

			// Verify campaign was inserted
			var count int
			err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM banner_campaigns").Scan(&count)
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(1))

			// Retrieve and verify
			var retrievedID, retrievedName string
			err = pool.QueryRow(ctx, "SELECT id, name FROM banner_campaigns WHERE id = $1", campaignData["id"]).Scan(&retrievedID, &retrievedName)
			Expect(err).NotTo(HaveOccurred())
			Expect(retrievedID).To(Equal(campaignData["id"]))
			Expect(retrievedName).To(Equal("Test Campaign"))
		})

		It("should update a campaign", func() {
			contextID = support.NewTestContextID()
			campaignData := support.NewTestCampaign(contextID)

			// Insert campaign
			campaignBytes, _ := json.Marshal(campaignData)
			_ = campaignBytes

			query := `
				INSERT INTO banner_campaigns (id, context_id, name, description, start_date, end_date, is_active, 
				                       target_url, tracking_code, impressions, clicks, conversion_rate, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
			`

			_, err := pool.Exec(ctx, query,
				campaignData["id"],
				campaignData["context_id"],
				campaignData["name"],
				campaignData["description"],
				campaignData["start_date"],
				campaignData["end_date"],
				campaignData["is_active"],
				campaignData["target_url"],
				campaignData["tracking_code"],
				campaignData["impressions"],
				campaignData["clicks"],
				campaignData["conversion_rate"],
				campaignData["created_at"],
				campaignData["updated_at"],
			)
			Expect(err).NotTo(HaveOccurred())

			// Update campaign
			_, err = pool.Exec(ctx, `
				UPDATE banner_campaigns SET name = $1, description = $2, is_active = $3
				WHERE id = $4
			`, "Updated Campaign", "Updated description", false, campaignData["id"])
			Expect(err).NotTo(HaveOccurred())

			// Verify update
			var name string
			var isActive bool
			err = pool.QueryRow(ctx, "SELECT name, is_active FROM banner_campaigns WHERE id = $1", campaignData["id"]).Scan(&name, &isActive)
			Expect(err).NotTo(HaveOccurred())
			Expect(name).To(Equal("Updated Campaign"))
			Expect(isActive).To(BeFalse())
		})

		It("should delete a campaign", func() {
			contextID = support.NewTestContextID()
			campaignData := support.NewTestCampaign(contextID)

			// Insert campaign
			query := `
				INSERT INTO banner_campaigns (id, context_id, name, description, start_date, end_date, is_active, 
				                       target_url, tracking_code, impressions, clicks, conversion_rate, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
			`

			_, err := pool.Exec(ctx, query,
				campaignData["id"],
				campaignData["context_id"],
				campaignData["name"],
				campaignData["description"],
				campaignData["start_date"],
				campaignData["end_date"],
				campaignData["is_active"],
				campaignData["target_url"],
				campaignData["tracking_code"],
				campaignData["impressions"],
				campaignData["clicks"],
				campaignData["conversion_rate"],
				campaignData["created_at"],
				campaignData["updated_at"],
			)
			Expect(err).NotTo(HaveOccurred())

			// Delete campaign
			_, err = pool.Exec(ctx, "DELETE FROM banner_campaigns WHERE id = $1", campaignData["id"])
			Expect(err).NotTo(HaveOccurred())

			// Verify deletion
			var count int
			err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM banner_campaigns").Scan(&count)
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(0))
		})
	})

	Describe("Banner placement CRUD", func() {
		It("should create and retrieve a banner placement", func() {
			contextID = support.NewTestContextID()
			placementData := support.NewTestPlacement(contextID)

			query := `
				INSERT INTO banner_placements (id, context_id, name, location, css_selector, max_banners, priority, 
				                        is_active, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
			`

			_, err := pool.Exec(ctx, query,
				placementData["id"],
				placementData["context_id"],
				placementData["name"],
				placementData["location"],
				placementData["css_selector"],
				placementData["max_banners"],
				placementData["priority"],
				placementData["is_active"],
				placementData["created_at"],
				placementData["updated_at"],
			)
			Expect(err).NotTo(HaveOccurred())

			// Verify placement was inserted
			var count int
			err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM banner_placements").Scan(&count)
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(1))

			// Retrieve and verify
			var retrievedID, retrievedName string
			err = pool.QueryRow(ctx, "SELECT id, name FROM banner_placements WHERE id = $1", placementData["id"]).Scan(&retrievedID, &retrievedName)
			Expect(err).NotTo(HaveOccurred())
			Expect(retrievedID).To(Equal(placementData["id"]))
			Expect(retrievedName).To(Equal("Test Placement"))
		})
	})

	Describe("Banner CRUD", func() {
		It("should create and retrieve a banner with campaign/placement links", func() {
			contextID = support.NewTestContextID()
			campaignData := support.NewTestCampaign(contextID)
			bannerData := support.NewTestBanner(contextID)

			// Insert campaign first
			campaignBytes, _ := json.Marshal(campaignData)
			_ = campaignBytes

			_, err := pool.Exec(ctx, `
				INSERT INTO banner_campaigns (id, context_id, name, description, start_date, end_date, is_active, 
				                       target_url, tracking_code, impressions, clicks, conversion_rate, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
			`,
				campaignData["id"],
				campaignData["context_id"],
				campaignData["name"],
				campaignData["description"],
				campaignData["start_date"],
				campaignData["end_date"],
				campaignData["is_active"],
				campaignData["target_url"],
				campaignData["tracking_code"],
				campaignData["impressions"],
				campaignData["clicks"],
				campaignData["conversion_rate"],
				campaignData["created_at"],
				campaignData["updated_at"],
			)
			Expect(err).NotTo(HaveOccurred())

			// Insert banner
			placements, _ := json.Marshal(bannerData["placements"])
			abVariants, _ := json.Marshal(bannerData["ab_variants"])

			_, err = pool.Exec(ctx, `
				INSERT INTO banner_banners (id, context_id, name, type, content, link_url, image_url, alt_text,
				                     campaign_id, placements, priority, start_date, end_date, is_active, 
				                     ab_test, ab_variants, impressions, clicks, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20)
			`,
				bannerData["id"],
				bannerData["context_id"],
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

			// Verify banner was inserted
			var count int
			err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM banner_banners").Scan(&count)
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(1))
		})

		It("should list banners by placement code", func() {
			contextID = support.NewTestContextID()

			// Create multiple banners with same placement
			for i := 0; i < 3; i++ {
				bannerData := support.NewTestBanner(contextID)
				bannerData["placements"] = []string{"header"}

				placements, _ := json.Marshal(bannerData["placements"])
				abVariants, _ := json.Marshal(bannerData["ab_variants"])

				_, err := pool.Exec(ctx, `
					INSERT INTO banner_banners (id, context_id, name, type, content, link_url, image_url, alt_text,
					                     campaign_id, placements, priority, start_date, end_date, is_active, 
					                     ab_test, ab_variants, impressions, clicks, created_at, updated_at)
					VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20)
				`,
					bannerData["id"],
					bannerData["context_id"],
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
			}

			// Query banners by placement
			var count int
			err := pool.QueryRow(ctx, `
				SELECT COUNT(*) FROM banner_banners 
				WHERE context_id = $1 AND placements @> '["header"]'::jsonb
			`, contextID).Scan(&count)
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(3))
		})

		It("should filter banners by active campaign date range", func() {
			contextID = support.NewTestContextID()
			bannerData := support.NewTestBanner(contextID)

			// Insert banner with future dates
			placements, _ := json.Marshal(bannerData["placements"])
			abVariants, _ := json.Marshal(bannerData["ab_variants"])

			_, err := pool.Exec(ctx, `
				INSERT INTO banner_banners (id, context_id, name, type, content, link_url, image_url, alt_text,
				                     campaign_id, placements, priority, start_date, end_date, is_active, 
				                     ab_test, ab_variants, impressions, clicks, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20)
			`,
				bannerData["id"],
				bannerData["context_id"],
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

			// Query banners in date range
			now := time.Now().UTC()
			var count int
			err = pool.QueryRow(ctx, `
				SELECT COUNT(*) FROM banner_banners 
				WHERE context_id = $1 
				AND start_date <= $2 
				AND end_date >= $2
			`, contextID, now).Scan(&count)
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(1))
		})

		It("should update banner impression/click counts", func() {
			contextID = support.NewTestContextID()
			bannerData := support.NewTestBanner(contextID)

			// Insert banner
			placements, _ := json.Marshal(bannerData["placements"])
			abVariants, _ := json.Marshal(bannerData["ab_variants"])

			_, err := pool.Exec(ctx, `
				INSERT INTO banner_banners (id, context_id, name, type, content, link_url, image_url, alt_text,
				                     campaign_id, placements, priority, start_date, end_date, is_active, 
				                     ab_test, ab_variants, impressions, clicks, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20)
			`,
				bannerData["id"],
				bannerData["context_id"],
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

			// Update counts
			_, err = pool.Exec(ctx, `
				UPDATE banner_banners SET impressions = impressions + 10, clicks = clicks + 2
				WHERE id = $1
			`, bannerData["id"])
			Expect(err).NotTo(HaveOccurred())

			// Verify counts
			var impressions, clicks int64
			err = pool.QueryRow(ctx, "SELECT impressions, clicks FROM banner_banners WHERE id = $1", bannerData["id"]).Scan(&impressions, &clicks)
			Expect(err).NotTo(HaveOccurred())
			Expect(impressions).To(Equal(int64(10)))
			Expect(clicks).To(Equal(int64(2)))
		})

		It("should delete banner and verify cascade", func() {
			contextID = support.NewTestContextID()
			bannerData := support.NewTestBanner(contextID)
			visitorID := support.NewTestUUID()

			// Insert banner
			placements, _ := json.Marshal(bannerData["placements"])
			abVariants, _ := json.Marshal(bannerData["ab_variants"])

			_, err := pool.Exec(ctx, `
				INSERT INTO banner_banners (id, context_id, name, type, content, link_url, image_url, alt_text,
				                     campaign_id, placements, priority, start_date, end_date, is_active, 
				                     ab_test, ab_variants, impressions, clicks, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20)
			`,
				bannerData["id"],
				bannerData["context_id"],
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

			// Insert related impressions and clicks
			_, err = pool.Exec(ctx, `
				INSERT INTO banner_impressions (id, banner_id, context_id, visitor_id, placement_id, created_at)
				VALUES ($1, $2, $3, $4, $5, $6)
			`, support.NewTestUUID(), bannerData["id"], contextID, visitorID, support.NewTestUUID(), time.Now().UTC())
			Expect(err).NotTo(HaveOccurred())

			_, err = pool.Exec(ctx, `
				INSERT INTO banner_clicks (id, banner_id, context_id, visitor_id, created_at)
				VALUES ($1, $2, $3, $4, $5)
			`, support.NewTestUUID(), bannerData["id"], contextID, visitorID, time.Now().UTC())
			Expect(err).NotTo(HaveOccurred())

			// Delete banner (should cascade)
			_, err = pool.Exec(ctx, "DELETE FROM banner_banners WHERE id = $1", bannerData["id"])
			Expect(err).NotTo(HaveOccurred())

			// Verify cascade deletion
			var impCount, clickCount int
			err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM banner_impressions WHERE banner_id = $1", bannerData["id"]).Scan(&impCount)
			Expect(err).NotTo(HaveOccurred())
			Expect(impCount).To(Equal(0))

			err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM banner_clicks WHERE banner_id = $1", bannerData["id"]).Scan(&clickCount)
			Expect(err).NotTo(HaveOccurred())
			Expect(clickCount).To(Equal(0))
		})
	})
})
