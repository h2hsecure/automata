package cicd_test

import (
	"encoding/base64"
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/h2hsecure/automata/cicd/support"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("MCP contacts import/export tools", func() {
	var (
		engine    *gin.Engine
		apiKey    string
		contextID string
	)

	BeforeEach(func() {
		engine, apiKey, contextID, _ = setupMCPEngine()
	})

	AfterEach(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM contact_tag_memberships")
		_, _ = pool.Exec(ctx, "DELETE FROM contact_tags")
		_, _ = pool.Exec(ctx, "DELETE FROM contacts")
		_, _ = pool.Exec(ctx, "DELETE FROM api_keys")
		_, _ = pool.Exec(ctx, `DELETE FROM contexts WHERE id = $1`, contextID)
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE email LIKE 'swagger-admin-%@example.com'`)
	})

	It("imports contacts from CSV via contacts_import", func() {
		csv := "email,first_name,last_name\nimport-a@example.com,Alice,Imports\nimport-b@example.com,Bob,Imports\n"
		w := mcpTool(engine, apiKey, 1, "contacts_import", map[string]any{"csv_data": csv})
		Expect(w.Code).To(Equal(http.StatusOK), "body: %s", w.Body.String())

		var result struct {
			Created int
			Updated int
			Skipped int
		}
		Expect(json.Unmarshal([]byte(mcpToolText(w)), &result)).To(Succeed())
		Expect(result.Created).To(Equal(2))

		var count int
		err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM contacts WHERE context_id = $1", contextID).Scan(&count)
		Expect(err).NotTo(HaveOccurred())
		Expect(count).To(Equal(2))
	})

	It("reports duplicate emails as updated via contacts_import", func() {
		contactID := support.NewTestUUID()
		customFields, _ := json.Marshal(map[string]interface{}{})
		_, err := pool.Exec(ctx, `
			INSERT INTO contacts (id, context_id, email, first_name, last_name, phone, company,
			                      custom_fields, source, source_id, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW(), NOW())
		`, contactID, contextID, "dup@example.com", "Dup", "Contact", "", "Corp",
			customFields, "test", "")
		Expect(err).NotTo(HaveOccurred())

		csv := "email,first_name\ndup@example.com,Again\nfresh@example.com,Fresh\n"
		w := mcpTool(engine, apiKey, 1, "contacts_import", map[string]any{"csv_data": csv})
		Expect(w.Code).To(Equal(http.StatusOK), "body: %s", w.Body.String())

		var result struct {
			Created int
			Updated int
		}
		Expect(json.Unmarshal([]byte(mcpToolText(w)), &result)).To(Succeed())
		Expect(result.Created).To(Equal(1))
		Expect(result.Updated).To(Equal(1))
	})

	It("exports contacts as base64 CSV via contacts_export", func() {
		contactID := support.NewTestUUID()
		customFields, _ := json.Marshal(map[string]interface{}{})
		_, err := pool.Exec(ctx, `
			INSERT INTO contacts (id, context_id, email, first_name, last_name, phone, company,
			                      custom_fields, source, source_id, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW(), NOW())
		`, contactID, contextID, "export-a@example.com", "Alice", "Exports", "", "Corp",
			customFields, "test", "")
		Expect(err).NotTo(HaveOccurred())

		w := mcpTool(engine, apiKey, 1, "contacts_export", map[string]any{
			"fields": []string{"email", "first_name"},
		})
		Expect(w.Code).To(Equal(http.StatusOK), "body: %s", w.Body.String())

		var out struct {
			CSVBase64 string `json:"csv_base64"`
		}
		text := mcpToolText(w)
		Expect(json.Unmarshal([]byte(text), &out)).To(Succeed(), "text: %s", text)
		Expect(out.CSVBase64).NotTo(BeEmpty())

		decoded, err := base64.StdEncoding.DecodeString(out.CSVBase64)
		Expect(err).NotTo(HaveOccurred())
		csv := string(decoded)
		Expect(csv).To(ContainSubstring("email,first_name"))
		Expect(csv).To(ContainSubstring("export-a@example.com,Alice"))
	})

	It("exports with default fields when fields is omitted", func() {
		contactID := support.NewTestUUID()
		customFields, _ := json.Marshal(map[string]interface{}{})
		_, err := pool.Exec(ctx, `
			INSERT INTO contacts (id, context_id, email, first_name, last_name, phone, company,
			                      custom_fields, source, source_id, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW(), NOW())
		`, contactID, contextID, "default-fields@example.com", "Alice", "Exports", "", "Corp",
			customFields, "test", "")
		Expect(err).NotTo(HaveOccurred())

		w := mcpTool(engine, apiKey, 1, "contacts_export", map[string]any{})
		Expect(w.Code).To(Equal(http.StatusOK), "body: %s", w.Body.String())

		var out struct {
			CSVBase64 string `json:"csv_base64"`
		}
		text := mcpToolText(w)
		Expect(json.Unmarshal([]byte(text), &out)).To(Succeed(), "text: %s", text)

		decoded, err := base64.StdEncoding.DecodeString(out.CSVBase64)
		Expect(err).NotTo(HaveOccurred())
		header := decodedHeaderLine(decoded)
		Expect(header).To(Equal("id,email,first_name,last_name,phone,company,source,tags,created_at"))
	})
})

// decodedHeaderLine returns the first line of decoded CSV output.
func decodedHeaderLine(decoded []byte) string {
	for i, b := range decoded {
		if b == '\n' {
			return string(decoded[:i])
		}
	}
	return string(decoded)
}