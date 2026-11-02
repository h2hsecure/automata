package cicd_test

import (
	"time"

	"github.com/h2hsecure/automata/cicd/support"
	"github.com/h2hsecure/automata/internal/domain/banner"
	brepo "github.com/h2hsecure/automata/internal/infrastructure/banner/repo"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Banner position/buttons persistence", func() {

	AfterEach(func() {
		_, err := pool.Exec(ctx, "DELETE FROM banner_banners")
		Expect(err).NotTo(HaveOccurred())
	})

	It("round-trips position and buttons through the repo", func() {
		contextID = support.NewTestContextID()
		repo := brepo.New(pool)

		b := &banner.Banner{
			ContextID: contextID,
			Name:      "Cookie banner",
			Type:      banner.BannerTypeHTML,
			Content:   "We use cookies",
			Position:  "fullpage",
			Buttons: []banner.Button{
				{ID: "b1", Label: "Accept", Action: "accept", Dismisses: true},
				{ID: "b2", Label: "Cookie Policy", Action: "view-policy", LinkURL: "/cookie-policy"},
			},
		}
		Expect(repo.CreateBanner(b)).To(Succeed())
		Expect(b.ID).NotTo(BeEmpty())

		got, err := repo.GetBanner(b.ID)
		Expect(err).NotTo(HaveOccurred())
		Expect(got.Position).To(Equal("fullpage"))
		Expect(got.Buttons).To(HaveLen(2))
		Expect(got.Buttons[0].Label).To(Equal("Accept"))
		Expect(got.Buttons[0].Action).To(Equal("accept"))
		Expect(got.Buttons[0].Dismisses).To(BeTrue())
		Expect(got.Buttons[1].LinkURL).To(Equal("/cookie-policy"))
		Expect(got.Buttons[1].Dismisses).To(BeFalse())
	})

	It("keeps legacy rows on the legacy path (empty position, no buttons)", func() {
		contextID = support.NewTestContextID()

		// Insert without position/buttons columns: DB defaults ('' and '[]')
		// apply, mimicking a pre-migration row.
		_, err := pool.Exec(ctx, `
			INSERT INTO banner_banners (id, context_id, name, type, content, link_url, placements, is_active, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, '', '[]'::jsonb, true, NOW(), NOW())
		`, support.NewTestUUID(), contextID, "Legacy banner", "html", "old content")
		Expect(err).NotTo(HaveOccurred())

		repo := brepo.New(pool)
		b, _, err := repo.ListBanners(contextID, banner.ListOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(b).To(HaveLen(1))
		Expect(b[0].Position).To(Equal(""))
		Expect(b[0].Buttons).ToNot(BeNil())
		Expect(b[0].Buttons).To(BeEmpty())

		// The legacy banner must still be returned by the active-placement
		// query used for snippet/config serving.
		active, err := repo.ListActiveBannersForPlacement(contextID, "", time.Now().UTC())
		Expect(err).NotTo(HaveOccurred())
		Expect(active).To(HaveLen(1))
		Expect(active[0].Position).To(Equal(""))
		Expect(active[0].Buttons).To(BeEmpty())
	})
})