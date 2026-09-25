package contact

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
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
func UpdateContact(repo Repository, id, contextID string, input UpdateInput) (*contact.Contact, error) {
	if contextID == "" {
		return nil, fmt.Errorf("context_id is required")
	}

	// Get existing contact
	existing, err := repo.GetByID(id)
	if err != nil {
		return nil, fmt.Errorf("get contact: %w", err)
	}

	// Verify contact belongs to context
	if existing.ContextID != contextID {
		return nil, fmt.Errorf("contact does not belong to context")
	}

	// Validate new email if provided
	if input.Email != nil && *input.Email != "" {
		if !isValidEmail(*input.Email) {
			return nil, fmt.Errorf("invalid email format: %s", *input.Email)
		}

		// Check for email conflicts with other contacts
		if existing.Email == nil || *existing.Email != *input.Email {
			existingContact, err := repo.FindByEmail(contextID, *input.Email)
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

	// Compute changed fields before update
	var changedFields []string
	if input.Email != nil && (existing.Email == nil || *existing.Email != *input.Email) {
		changedFields = append(changedFields, "email")
	}
	if input.FirstName != "" && existing.FirstName != input.FirstName {
		changedFields = append(changedFields, "first_name")
	}
	if input.LastName != "" && existing.LastName != input.LastName {
		changedFields = append(changedFields, "last_name")
	}
	if input.Phone != "" && existing.Phone != input.Phone {
		changedFields = append(changedFields, "phone")
	}
	if input.Company != "" && existing.Company != input.Company {
		changedFields = append(changedFields, "company")
	}
	if input.CustomFields != nil && !mapsEqual(existing.CustomFields, input.CustomFields) {
		changedFields = append(changedFields, "custom_fields")
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

	// Create activity if fields changed
	if len(changedFields) > 0 {
		now := time.Now().Format(time.RFC3339)
		prevValues := map[string]interface{}{
			"email":         existing.Email,
			"first_name":    input.FirstName,
			"last_name":     input.LastName,
			"phone":         input.Phone,
			"company":       input.Company,
		}
		if err := repo.CreateActivity(&contact.Activity{
			ID:        uuid.New().String(),
			ContactID: id,
			ContextID: contextID,
			Type:      contact.ActivityContactUpdated,
			Data: map[string]interface{}{
				"changed_fields": changedFields,
				"updated_at":     now,
				"previous_values": prevValues,
			},
		}); err != nil {
			_ = fmt.Errorf("create activity: %w", err)
		}
	}

	// Update tags (delete old memberships, create new ones)
	if len(input.Tags) > 0 {
		if err := applyTags(repo, contextID, id, input.Tags); err != nil {
			return nil, fmt.Errorf("apply tags: %w", err)
		}
	}

	return existing, nil
}

// mapsEqual checks if two maps are equal (same keys and values).
func mapsEqual(a, b map[string]interface{}) bool {
	if len(a) != len(b) {
		return false
	}
	for k, va := range a {
		if vb, ok := b[k]; !ok || va != vb {
			return false
		}
	}
	return true
}
