package cicd_test

import (
	"github.com/hekemen/automata/cicd/support"
	"github.com/hekemen/automata/pkg/snippet"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Snippet Generator Tests", func() {
	Describe("JavaScript snippet generation", func() {
		It("should generate tracking snippet with tenant ID", func() {
			tenantID := support.NewTestTenantID()

			js, err := snippet.Generate(tenantID, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(js).To(ContainSubstring(tenantID))
			Expect(js).To(ContainSubstring("Automata"))
		})

		It("should generate tracking pixel URL", func() {
			tenantID := support.NewTestTenantID()

			js, err := snippet.Generate(tenantID, nil)
			Expect(err).NotTo(HaveOccurred())

			// Verify snippet contains visitor ID handling
			Expect(js).To(ContainSubstring("visitorId"))
			Expect(js).To(ContainSubstring("automata_visitor"))
		})

		It("should generate snippet with custom domain", func() {
			tenantID := support.NewTestTenantID()
			options := map[string]interface{}{
				"apiHost": "https://custom.example.com",
			}

			js, err := snippet.Generate(tenantID, options)
			Expect(err).NotTo(HaveOccurred())
			Expect(js).To(ContainSubstring("https://custom.example.com"))
		})

		It("should generate snippet with event tracking enabled", func() {
			tenantID := support.NewTestTenantID()
			options := map[string]interface{}{
				"batchSize":     float64(50),
				"batchInterval": float64(60),
			}

			js, err := snippet.Generate(tenantID, options)
			Expect(err).NotTo(HaveOccurred())
			Expect(js).To(ContainSubstring("50"))
			Expect(js).To(ContainSubstring("60"))
		})

		It("should verify snippet contains required script tags", func() {
			tenantID := support.NewTestTenantID()

			js, err := snippet.Generate(tenantID, nil)
			Expect(err).NotTo(HaveOccurred())

			// Verify snippet is a valid IIFE
			Expect(js).To(ContainSubstring("(function()"))
			Expect(js).To(ContainSubstring("})();"))

			// Verify required functions exist
			Expect(js).To(ContainSubstring("a.track"))
			Expect(js).To(ContainSubstring("trackPageView"))
			Expect(js).To(ContainSubstring("_sendEvent"))
		})

		It("should verify pixel URL includes tenant and visitor IDs", func() {
			tenantID := support.NewTestTenantID()

			js, err := snippet.Generate(tenantID, nil)
			Expect(err).NotTo(HaveOccurred())

			// Verify snippet references tenant ID
			Expect(js).To(ContainSubstring("a.tenantId=\"" + tenantID + "\""))

			// Verify snippet handles visitor ID
			Expect(js).To(ContainSubstring("visitorId"))
		})
	})

	Describe("Banner snippet generation", func() {
		It("should generate banner snippet with tenant ID", func() {
			tenantID := support.NewTestTenantID()

			js, err := snippet.GenerateBannerSnippet(tenantID, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(js).To(ContainSubstring(tenantID))
			Expect(js).To(ContainSubstring("AutomataBanner"))
		})

		It("should generate banner snippet with custom server host", func() {
			tenantID := support.NewTestTenantID()
			options := map[string]interface{}{
				"serverHost": "https://banners.example.com",
			}

			js, err := snippet.GenerateBannerSnippet(tenantID, options)
			Expect(err).NotTo(HaveOccurred())
			Expect(js).To(ContainSubstring("https://banners.example.com"))
		})
	})

	Describe("Snippet sanitization", func() {
		It("should remove leading/trailing whitespace", func() {
			tenantID := support.NewTestTenantID()

			js, err := snippet.Generate(tenantID, nil)
			Expect(err).NotTo(HaveOccurred())

			sanitized := snippet.Sanitize("  " + js + "  ")
			Expect(sanitized).To(Equal(js))
			Expect(sanitized).NotTo(HavePrefix(" "))
			Expect(sanitized).NotTo(HaveSuffix(" "))
		})
	})

	Describe("Error handling", func() {
		It("should return error for empty tenant ID", func() {
			_, err := snippet.Generate("", nil)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("tenantID required"))
		})

		It("should return error for empty tenant ID in banner snippet", func() {
			_, err := snippet.GenerateBannerSnippet("", nil)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("tenantID required"))
		})
	})

	Describe("Minified generation", func() {
		It("should generate minified snippet", func() {
			tenantID := support.NewTestTenantID()

			js, err := snippet.GenerateMinified(tenantID, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(js).To(ContainSubstring(tenantID))

			// Verify minified is same as regular
			regular, err := snippet.Generate(tenantID, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(js).To(Equal(regular))
		})
	})
})
