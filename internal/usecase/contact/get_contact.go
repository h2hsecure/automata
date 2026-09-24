package contact

import (
	"fmt"

	"github.com/hekemen/automata/internal/domain/contact"
)

// GetContact retrieves a contact by ID after verifying it belongs to the context.
func GetContact(repo Repository, id, contextID string) (*contact.Contact, error) {
	if contextID == "" {
		return nil, fmt.Errorf("context_id is required")
	}

	c, err := repo.GetByID(id)
	if err != nil {
		return nil, fmt.Errorf("get contact: %w", err)
	}

	if c.ContextID != contextID {
		return nil, fmt.Errorf("contact does not belong to context")
	}

	return c, nil
}
