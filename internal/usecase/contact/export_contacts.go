package contact

import (
	"fmt"

	"github.com/hekemen/automata/internal/domain/contact"
	"github.com/hekemen/automata/pkg/export"
)

// ExportContacts retrieves contacts for a tenant, converts them to CSV,
// and returns the CSV bytes.
func ExportContacts(repo Repository, tenantID string, filters contact.FilterOptions, fields []string) ([]byte, error) {
	if tenantID == "" {
		return nil, fmt.Errorf("tenant_id is required")
	}
	if len(fields) == 0 {
		return nil, fmt.Errorf("at least one field is required for export")
	}

	// Fetch all contacts (no pagination — export everything)
	contacts, err := repo.List(0, 100000, filters)
	if err != nil {
		return nil, fmt.Errorf("list contacts: %w", err)
	}

	if len(contacts) == 0 {
		return nil, fmt.Errorf("no contacts to export")
	}

	// Convert contacts to map[string]interface{}
	contactMaps := make([]map[string]interface{}, 0, len(contacts))
	for _, c := range contacts {
		contactMaps = append(contactMaps, contactToMap(c, fields))
	}

	// Generate CSV
	return export.Generate(contactMaps, fields)
}

// contactToMap converts a Contact to a map with only the requested fields.
func contactToMap(c *contact.Contact, fields []string) map[string]interface{} {
	m := make(map[string]interface{})
	for _, field := range fields {
		switch field {
		case "id":
			m["id"] = c.ID
		case "email":
			if c.Email != nil {
				m["email"] = *c.Email
			} else {
				m["email"] = ""
			}
		case "first_name":
			m["first_name"] = c.FirstName
		case "last_name":
			m["last_name"] = c.LastName
		case "phone":
			m["phone"] = c.Phone
		case "company":
			m["company"] = c.Company
		case "source":
			m["source"] = c.Source
		case "source_id":
			m["source_id"] = c.SourceID
		case "tags":
			tagNames := make([]string, 0, len(c.Tags))
			for _, tag := range c.Tags {
				tagNames = append(tagNames, tag.Name)
			}
			m["tags"] = tagNames
		case "custom_fields":
			m["custom_fields"] = c.CustomFields
		case "created_at":
			m["created_at"] = c.CreatedAt.Format("2006-01-02T15:04:05Z")
		case "updated_at":
			m["updated_at"] = c.UpdatedAt.Format("2006-01-02T15:04:05Z")
		default:
			// Check custom fields
			if val, ok := c.CustomFields[field]; ok {
				m[field] = val
			}
		}
	}
	return m
}
