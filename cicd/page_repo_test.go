package cicd_test

import (
	"context"

	"github.com/h2hsecure/automata/cicd/support"
	"github.com/h2hsecure/automata/internal/domain/page"
	pagerepo "github.com/h2hsecure/automata/internal/infrastructure/page/repo"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = BeforeEach(func() {
	ctx = context.Background()
	pool = db.Pool
	contextID = support.NewTestContextID()
})

var _ = Describe("Page Repository Integration Tests", func() {
	var repo page.Repository

	BeforeEach(func() {
		repo = pagerepo.New(pool)
	})

	AfterEach(func() {
		_, err := pool.Exec(ctx, "DELETE FROM page_decisions")
		Expect(err).NotTo(HaveOccurred())
		_, err = pool.Exec(ctx, "DELETE FROM page_impressions")
		Expect(err).NotTo(HaveOccurred())
		_, err = pool.Exec(ctx, "DELETE FROM page_pages")
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("Position and buttons round-trip", func() {
		It("should create and retrieve a page with position and buttons", func() {
			p := &page.Page{
				ID:        support.NewTestUUID(),
				ContextID: contextID,
				Slug:      "cookie-banner",
				Title:     "Cookie Banner",
				Content:   "<p>Text</p>",
				IsActive:  true,
				Position:  page.PositionLeft,
				Buttons: []page.Button{
					{Label: "Accept", Href: "#", Action: "dismiss_accept", Variant: "primary"},
					{Label: "Decline", Href: "/privacy", Action: "dismiss_decline", Variant: "secondary"},
				},
			}
			Expect(repo.Create(p)).To(Succeed())

			got, err := repo.GetByID(p.ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(got.Position).To(Equal(page.PositionLeft))
			Expect(got.Buttons).To(HaveLen(2))
			Expect(got.Buttons[0].Label).To(Equal("Accept"))
			Expect(got.Buttons[0].Href).To(Equal("#"))
			Expect(got.Buttons[0].Action).To(Equal("dismiss_accept"))
			Expect(got.Buttons[0].Variant).To(Equal("primary"))
			Expect(got.Buttons[1].Action).To(Equal("dismiss_decline"))

			bySlug, err := repo.GetBySlug(contextID, "cookie-banner")
			Expect(err).NotTo(HaveOccurred())
			Expect(bySlug.Position).To(Equal(page.PositionLeft))
			Expect(bySlug.Buttons).To(HaveLen(2))

			list, err := repo.List(contextID)
			Expect(err).NotTo(HaveOccurred())
			Expect(list).To(HaveLen(1))
			Expect(list[0].Position).To(Equal(page.PositionLeft))
			Expect(list[0].Buttons).To(HaveLen(2))
		})

		It("should round-trip a page with no buttons", func() {
			p := &page.Page{
				ID:        support.NewTestUUID(),
				ContextID: contextID,
				Slug:      "plain",
				Title:     "Plain",
				IsActive:  true,
			}
			Expect(repo.Create(p)).To(Succeed())

			got, err := repo.GetByID(p.ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(got.Position).To(Equal(page.PositionNone))
			if got.Buttons != nil {
				Expect(got.Buttons).To(BeEmpty())
			}
		})

		It("should list only active pages via ListActive", func() {
			active := &page.Page{ID: support.NewTestUUID(), ContextID: contextID, Slug: "active", Title: "A", IsActive: true}
			inactive := &page.Page{ID: support.NewTestUUID(), ContextID: contextID, Slug: "inactive", Title: "B", IsActive: false}
			Expect(repo.Create(active)).To(Succeed())
			Expect(repo.Create(inactive)).To(Succeed())

			got, err := repo.ListActive(contextID)
			Expect(err).NotTo(HaveOccurred())
			Expect(got).To(HaveLen(1))
			Expect(got[0].Slug).To(Equal("active"))
		})

		It("should list a positioned page as active", func() {
			p := &page.Page{
				ID: support.NewTestUUID(), ContextID: contextID, Slug: "banner",
				Title: "B", IsActive: true, Position: page.PositionBottom,
			}
			Expect(repo.Create(p)).To(Succeed())
			got, err := repo.ListActive(contextID)
			Expect(err).NotTo(HaveOccurred())
			Expect(got).To(HaveLen(1))
			Expect(got[0].Position).To(Equal(page.PositionBottom))
		})

		Describe("Decisions and impressions", func() {
			It("should record decisions and report counts", func() {
				p := &page.Page{ID: support.NewTestUUID(), ContextID: contextID, Slug: "d", Title: "D", IsActive: true, Position: page.PositionTop}
				Expect(repo.Create(p)).To(Succeed())

				Expect(repo.RecordDecision(&page.Decision{ContextID: contextID, PageID: p.ID, VisitorID: "v1", Action: "accept", IPHash: "ip1", UserAgent: "ua1"})).To(Succeed())
				Expect(repo.RecordDecision(&page.Decision{ContextID: contextID, PageID: p.ID, VisitorID: "v2", Action: "decline", IPHash: "ip2", UserAgent: "ua2"})).To(Succeed())

				decisions, total, err := repo.ListDecisions(p.ID, 0, 10)
				Expect(err).NotTo(HaveOccurred())
				Expect(total).To(Equal(int64(2)))
				Expect(decisions).To(HaveLen(2))

				counts, err := repo.DecisionCounts(p.ID)
				Expect(err).NotTo(HaveOccurred())
				Expect(counts).To(Equal(map[string]int64{"accept": 1, "decline": 1}))
			})

			It("should cascade decisions and impressions on page delete", func() {
				p := &page.Page{ID: support.NewTestUUID(), ContextID: contextID, Slug: "c", Title: "C", IsActive: true, Position: page.PositionFull}
				Expect(repo.Create(p)).To(Succeed())
				Expect(repo.RecordDecision(&page.Decision{ContextID: contextID, PageID: p.ID, VisitorID: "v1", Action: "accept"})).To(Succeed())
				Expect(repo.RecordImpression(contextID, p.ID, "v1")).To(Succeed())

				Expect(repo.Delete(p.ID)).To(Succeed())

				decisions, total, err := repo.ListDecisions(p.ID, 0, 10)
				Expect(err).NotTo(HaveOccurred())
				Expect(total).To(Equal(int64(0)))
				Expect(decisions).To(BeEmpty())

				var impCount int
				err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM page_impressions WHERE page_id = $1", p.ID).Scan(&impCount)
				Expect(err).NotTo(HaveOccurred())
				Expect(impCount).To(Equal(0))
			})
		})
	})
})
