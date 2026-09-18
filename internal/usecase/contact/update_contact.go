package contact

import (
	"fmt"
	"strings"

	"github.com/hekemen/automata/internal/domain/contact"
)

// UpdateInput holds the input parameters for updating a contact.
type UpdateInput struct {
	Email        *string
	FirstName    string
	LastName     string
	Phone        string
	Company      string
	CustomFields map[string]interface{}
	Tags         []string // replaces all existing tags
}

// UpdateContact updates an existing contact and replaces its tags.
func UpdateContact(repo Repository, id, tenantID string, input UpdateInput) (*contact.Contact, error) {
	if tenantID == "" {
		return nil, fmt.Errorf("tenant_id is required")
	}

	// Get existing contact
	existing, err := repo.GetByID(id)
	if err != nil {
		return nil, fmt.Errorf("get contact: %w", err)
	}

	// Verify contact belongs to tenant
	if existing.TenantID != tenantID {
		return nil, fmt.Errorf("contact does not belong to tenant")
	}

	// Validate new email if provided
	if input.Email != nil && *input.Email != "" {
		if !isValidEmail(*input.Email) {
			return nil, fmt.Errorf("invalid email format: %s", *input.Email)
		}

		// Check for email conflicts with other contacts
		if existing.Email == nil || *existing.Email != *input.Email {
			existingContact, err := repo.FindByEmail(tenantID, *input.Email)
			if err != nil {
				if !strings.Contains(err.Error(), "contact not found") {
					return nil, fmt.Errorf("check email conflict: %w", err)
				}
			} else {
				// Email belongs to a different contact
				if existingContact.ID != id {
					return nil, fmt.Errorf("contact with this email already exists")
				}
			}
		}
	}

	// Update contact fields
	existing.Email = input.Email
	existing.FirstName = input.FirstName
	existing.LastName = input.LastName
	existing.Phone = input.Phone
	existing.Company = input.Company
	existing.CustomFields = input.CustomFields

	if err := existing.Validate(); err != nil {
		return nil, fmt.Errorf("validate contact: %w", err)
	}

	if err := repo.Update(existing); err != nil {
		return nil, fmt.Errorf("update contact: %w", err)
	}

	// Update tags (delete old memberships, create new ones)
	if len(input.Tags) > 0 {
		if err := applyTags(repo, tenantID, id, input.Tags); err != nil {
			return nil, fmt.Errorf("apply tags: %w", err)
		}
	}

	return existing, nil
}
