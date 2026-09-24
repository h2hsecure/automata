package contact

import (
	"fmt"
)

// MergeContacts merges one contact into another, keeping the keepID contact and deleting mergeIntoID.
func MergeContacts(repo Repository, contextID, keepID, mergeIntoID string) error {
	if contextID == "" {
		return fmt.Errorf("context_id is required")
	}

	// Verify both contacts exist and belong to same context
	keep, err := repo.GetByID(keepID)
	if err != nil {
		return fmt.Errorf("get keep contact: %w", err)
	}
	if keep.ContextID != contextID {
		return fmt.Errorf("keep contact does not belong to context")
	}

	mergeInto, err := repo.GetByID(mergeIntoID)
	if err != nil {
		return fmt.Errorf("get merge contact: %w", err)
	}
	if mergeInto.ContextID != contextID {
		return fmt.Errorf("merge contact does not belong to context")
	}

	// Verify they are different contacts
	if keepID == mergeIntoID {
		return fmt.Errorf("cannot merge a contact into itself")
	}

	// Perform the merge
	if err := repo.Merge(keepID, mergeIntoID); err != nil {
		return fmt.Errorf("merge contacts: %w", err)
	}

	return nil
}
