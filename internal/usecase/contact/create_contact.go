package contact

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	ct "github.com/hekemen/automata/internal/domain/contact"
)

// CreateInput holds the input parameters for creating a contact.
type CreateInput struct {
	Email        *string
	FirstName    string
	LastName     string
	Phone        string
	Company      string
	CustomFields map[string]interface{}
	Source       string
	SourceID     string
	Tags         []string
}

// CreateContact creates a new contact or returns the existing one if a duplicate email is found.
func CreateContact(repo Repository, contextID string, input CreateInput) (*ct.Contact, error) {
	if contextID == "" {
		return nil, fmt.Errorf("context_id is required")
	}

	// Validate email format if provided
	if input.Email != nil && *input.Email != "" {
		if !isValidEmail(*input.Email) {
			return nil, fmt.Errorf("invalid email format: %s", *input.Email)
		}
	}

	// Check for duplicate email
	if input.Email != nil && *input.Email != "" {
		existing, err := repo.FindByEmail(contextID, *input.Email)
		if err != nil {
			// If not found, proceed with creation
			if !strings.Contains(err.Error(), "contact not found") {
				return nil, fmt.Errorf("check duplicate email: %w", err)
			}
		} else {
			// Duplicate found, return existing contact
			return existing, nil
		}
	}

	// Generate UUID for new contact
	id := uuid.New().String()

	c := &ct.Contact{
		ID:           id,
		ContextID:    contextID,
		Email:        input.Email,
		FirstName:    input.FirstName,
		LastName:     input.LastName,
		Phone:        input.Phone,
		Company:      input.Company,
		CustomFields: input.CustomFields,
		Source:       input.Source,
		SourceID:     input.SourceID,
	}

	if err := c.Validate(); err != nil {
		return nil, fmt.Errorf("validate contact: %w", err)
	}

	if err := repo.Create(c); err != nil {
		return nil, fmt.Errorf("create contact: %w", err)
	}

	// Create activity for contact creation
	now := time.Now().Format(time.RFC3339)
	if err := repo.CreateActivity(&ct.Activity{
		ID:        uuid.New().String(),
		ContactID: id,
		ContextID: contextID,
		Type:      ct.ActivityContactCreated,
		Data: map[string]interface{}{
			"source":    input.Source,
			"created_at": now,
		},
	}); err != nil {
		// Non-fatal: log but don't fail the operation
		_ = fmt.Errorf("create activity: %w", err)
	}

	// Apply tags if provided
	if len(input.Tags) > 0 {
		if err := applyTags(repo, contextID, id, input.Tags); err != nil {
			return nil, fmt.Errorf("apply tags: %w", err)
		}
	}

	return c, nil
}

// isValidEmail checks basic email format validity.
func isValidEmail(email string) bool {
	if len(email) == 0 || len(email) > 254 {
		return false
	}
	atIdx := -1
	for i, ch := range email {
		if ch == '@' {
			atIdx = i
		}
	}
	if atIdx <= 0 || atIdx >= len(email)-1 {
		return false
	}
	local := email[:atIdx]
	domain := email[atIdx+1:]
	if len(local) == 0 || len(domain) == 0 {
		return false
	}
	if len(local) > 64 {
		return false
	}
	for _, ch := range local {
		if (ch < 33 || ch > 126) && ch != '.' && ch != '_' && ch != '-' && ch != '+' {
			return false
		}
	}
	if len(domain) > 253 {
		return false
	}
	return true
}
