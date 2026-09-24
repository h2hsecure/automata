package contact

import (
	"fmt"
)

// DeleteContact removes a contact by ID after verifying it belongs to the context.
func DeleteContact(repo Repository, id, contextID string) error {
	if contextID == "" {
		return fmt.Errorf("context_id is required")
	}

	// Get existing contact to verify it belongs to context
	existing, err := repo.GetByID(id)
	if err != nil {
		return fmt.Errorf("get contact: %w", err)
	}

	if existing.ContextID != contextID {
		return fmt.Errorf("contact does not belong to context")
	}

	// Delete contact (tags cascade via FK)
	if err := repo.Delete(id); err != nil {
		return fmt.Errorf("delete contact: %w", err)
	}

	return nil
}
