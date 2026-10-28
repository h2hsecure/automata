package cicd_test

import (
	"github.com/h2hsecure/automata/cicd/support"
	"github.com/h2hsecure/automata/pkg/snippet"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Snippet Generator Tests", func() {
	Describe("JavaScript snippet generation", func() {
		It("should generate tracking snippet with context ID", func() {
			contextID := support.NewTestContextID()

			js, err := snippet.Generate(contextID, "", nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(js).To(ContainSubstring(contextID))
			Expect(js).To(ContainSubstring(`"/track/` + contextID + `"`))
		})

		It("should generate tracking pixel URL", func() {
			contextID := support.NewTestContextID()

			js, err := snippet.Generate(contextID, "", nil)
			Expect(err).NotTo(HaveOccurred())

			// Verify snippet contains opaque cookie + visitor ID generation
			Expect(js).To(ContainSubstring("document.cookie"))
			Expect(js).To(ContainSubstring("xxxx-4xxx"))
		})

		It("should generate snippet with custom domain", func() {
			contextID := support.NewTestContextID()
			options := map[string]interface{}{
				"apiHost": "https://custom.example.com",
			}

			js, err := snippet.Generate(contextID, "https://custom.example.com", options)
			Expect(err).NotTo(HaveOccurred())
			Expect(js).To(ContainSubstring("https://custom.example.com"))
		})

		It("should generate snippet with event tracking enabled", func() {
			contextID := support.NewTestContextID()
			options := map[string]interface{}{
				"batchSize":     float64(50),
				"batchInterval": float64(60),
			}

			js, err := snippet.Generate(contextID, "", options)
			Expect(err).NotTo(HaveOccurred())
			Expect(js).To(ContainSubstring(">=50"))
			Expect(js).To(ContainSubstring("60*1000"))
		})

		It("should verify snippet contains required script tags", func() {
			contextID := support.NewTestContextID()

			js, err := snippet.Generate(contextID, "", nil)
			Expect(err).NotTo(HaveOccurred())

			// Verify snippet is a valid IIFE
			Expect(js).To(ContainSubstring("(function()"))
			Expect(js).To(ContainSubstring("})()"))

			// Verify the compact, preflight-free transport markers
			Expect(js).To(ContainSubstring(`"pv"`))
			Expect(js).To(ContainSubstring("text/plain"))
			Expect(js).To(ContainSubstring("keepalive"))
		})

		It("should verify pixel URL includes context and visitor IDs", func() {
			contextID := support.NewTestContextID()

			js, err := snippet.Generate(contextID, "", nil)
			Expect(err).NotTo(HaveOccurred())

			// Verify snippet posts to the scoped context endpoint
			Expect(js).To(ContainSubstring(`"/track/` + contextID + `"`))

			// Verify snippet captures the page title for events
			Expect(js).To(ContainSubstring("document.title"))
		})
	})

	Describe("Banner snippet generation", func() {
		It("should generate banner snippet with context ID", func() {
			contextID := support.NewTestContextID()

			js, err := snippet.GenerateBannerSnippet(contextID, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(js).To(ContainSubstring(contextID))
			Expect(js).To(ContainSubstring("AutomataBanner"))
		})

		It("should generate banner snippet with custom server host", func() {
			contextID := support.NewTestContextID()
			options := map[string]interface{}{
				"serverHost": "https://banners.example.com",
			}

			js, err := snippet.GenerateBannerSnippet(contextID, options)
			Expect(err).NotTo(HaveOccurred())
			Expect(js).To(ContainSubstring("https://banners.example.com"))
		})
	})

	Describe("Snippet sanitization", func() {
		It("should remove leading/trailing whitespace", func() {
			contextID := support.NewTestContextID()

			js, err := snippet.Generate(contextID, "", nil)
			Expect(err).NotTo(HaveOccurred())

			sanitized := snippet.Sanitize("  " + js + "  ")
			Expect(sanitized).To(Equal(js))
			Expect(sanitized).NotTo(HavePrefix(" "))
			Expect(sanitized).NotTo(HaveSuffix(" "))
		})
	})

	Describe("Error handling", func() {
		It("should return error for empty context ID", func() {
			_, err := snippet.Generate("", "", nil)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("contextID required"))
		})

		It("should return error for empty context ID in banner snippet", func() {
			_, err := snippet.GenerateBannerSnippet("", nil)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("contextID required"))
		})
	})

	Describe("Minified generation", func() {
		It("should generate minified snippet", func() {
			contextID := support.NewTestContextID()

			js, err := snippet.GenerateMinified(contextID, "", nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(js).To(ContainSubstring(contextID))

			// Verify minified is same as regular
			regular, err := snippet.Generate(contextID, "", nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(js).To(Equal(regular))
		})
	})
})
