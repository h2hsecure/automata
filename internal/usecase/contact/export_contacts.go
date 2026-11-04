package contact

import (
	"fmt"

	"github.com/h2hsecure/automata/internal/domain/contact"
	"github.com/h2hsecure/automata/pkg/export"
)

// exportPageLimit bounds the memory of a single export page; pagination
// continues until the repository returns a short page.
const exportPageLimit = 1000

// ExportContacts retrieves contacts for a context, converts them to CSV,
// and returns the CSV bytes.
func ExportContacts(repo Repository, contextID string, filters contact.FilterOptions, fields []string) ([]byte, error) {
	if contextID == "" {
		return nil, fmt.Errorf("context_id is required")
	}
	if len(fields) == 0 {
		return nil, fmt.Errorf("at least one field is required for export")
	}
	return exportContactsPages(repo, contextID, filters, fields, exportPageLimit)
}

// exportContactsPages lists contacts in pages of pageSize and exports
// every row. Pagination continues until a short page is returned, so
// exports never silently cap at the first page.
func exportContactsPages(repo Repository, contextID string, filters contact.FilterOptions, fields []string, pageSize int) ([]byte, error) {
	if pageSize <= 0 {
		pageSize = exportPageLimit
	}

	filters.ContextID = contextID
	var contacts []*contact.Contact
	offset := 0
	for {
		page, err := repo.List(offset, pageSize, filters)
		if err != nil {
			return nil, fmt.Errorf("list contacts: %w", err)
		}
		contacts = append(contacts, page...)
		if len(page) < pageSize {
			break
		}
		offset += len(page)
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
