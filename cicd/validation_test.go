package cicd_test

import (
	"regexp"

	"github.com/hekemen/automata/cicd/support"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Validation Tests", func() {
	Describe("Email validation", func() {
		It("should validate valid email format", func() {
			validEmails := []string{
				"user@example.com",
				"john.doe@company.org",
				"user+tag@example.com",
				"user_name@example.com",
				"user-name@example.com",
			}

			emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

			for _, email := range validEmails {
				Expect(emailRegex.MatchString(email)).To(BeTrue(), "Email should be valid: "+email)
			}
		})

		It("should reject invalid email format", func() {
			invalidEmails := []string{
				"not-an-email",
				"user@",
				"@domain.com",
				"",
				"user@.com",
				"user@domain",
			}

			emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

			for _, email := range invalidEmails {
				Expect(emailRegex.MatchString(email)).To(BeFalse(), "Email should be invalid: "+email)
			}
		})
	})

	Describe("Phone number validation", func() {
		It("should validate phone number format", func() {
			validPhones := []string{
				"+1234567890",
				"+1 (234) 567-8900",
				"123-456-7890",
				"123.456.7890",
			}

			phoneRegex := regexp.MustCompile(`^\+?[\d\s\-\(\.)]+$`)

			for _, phone := range validPhones {
				Expect(phoneRegex.MatchString(phone)).To(BeTrue(), "Phone should be valid: "+phone)
			}
		})

		It("should reject invalid phone number format", func() {
			invalidPhones := []string{
				"",
				"abc",
			}

			phoneRegex := regexp.MustCompile(`^\+?[\d\s\-\(\.)]+$`)

			for _, phone := range invalidPhones {
				Expect(phoneRegex.MatchString(phone)).To(BeFalse(), "Phone should be invalid: "+phone)
			}
		})
	})

	Describe("Form field validation", func() {
		It("should validate required form fields", func() {
			formData := support.NewTestForm("test-tenant")

			// Verify form has required fields
			Expect(formData["slug"]).NotTo(BeEmpty())
			Expect(formData["name"]).NotTo(BeEmpty())
			Expect(formData["fields"]).NotTo(BeEmpty())
		})

		It("should validate custom field types", func() {
			validTypes := []string{
				"text",
				"email",
				"tel",
				"number",
				"textarea",
				"checkbox",
				"radio",
				"select",
			}

			validTypeRegex := regexp.MustCompile(`^(text|email|tel|number|textarea|checkbox|radio|select)$`)

			for _, fieldType := range validTypes {
				Expect(validTypeRegex.MatchString(fieldType)).To(BeTrue(), "Field type should be valid: "+fieldType)
			}
		})
	})

	Describe("UTM parameter validation", func() {
		It("should validate UTM parameter format", func() {
			validUTMs := map[string]string{
				"utm_source":    "google",
				"utm_medium":    "cpc",
				"utm_campaign":  "summer_sale",
				"utm_content":   "banner",
				"utm_term":      "keyword",
			}

			utmRegex := regexp.MustCompile(`^[a-zA-Z0-9_\-%]+$`)

			for param, value := range validUTMs {
				Expect(utmRegex.MatchString(value)).To(BeTrue(), "UTM parameter should be valid: "+param+"="+value)
			}
		})

		It("should reject invalid UTM parameter format", func() {
			invalidUTMs := []string{
				"",
				"campaign with spaces",
				"campaign/slash",
			}

			utmRegex := regexp.MustCompile(`^[a-zA-Z0-9_\-%]+$`)

			for _, utm := range invalidUTMs {
				Expect(utmRegex.MatchString(utm)).To(BeFalse(), "UTM should be invalid: "+utm)
			}
		})
	})

	Describe("Tenant slug validation", func() {
		It("should validate tenant slug format", func() {
			validSlugs := []string{
				"test-tenant",
				"my-company",
				"acme-corp",
				"test123",
			}

			slugRegex := regexp.MustCompile(`^[a-z0-9\-]+$`)

			for _, slug := range validSlugs {
				Expect(slugRegex.MatchString(slug)).To(BeTrue(), "Slug should be valid: "+slug)
			}
		})

		It("should reject invalid tenant slug format", func() {
			invalidSlugs := []string{
				"",
				"Tenant With Spaces",
				"tenant_with_underscores",
				"Tenant123",
				"tenant..double",
			}

			slugRegex := regexp.MustCompile(`^[a-z0-9\-]+$`)

			for _, slug := range invalidSlugs {
				Expect(slugRegex.MatchString(slug)).To(BeFalse(), "Slug should be invalid: "+slug)
			}
		})
	})
})
