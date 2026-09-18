package contact

import (
	"fmt"
)

// MergeContacts merges one contact into another, keeping the keepID contact and deleting mergeIntoID.
func MergeContacts(repo Repository, tenantID, keepID, mergeIntoID string) error {
	if tenantID == "" {
		return fmt.Errorf("tenant_id is required")
	}

	// Verify both contacts exist and belong to same tenant
	keep, err := repo.GetByID(keepID)
	if err != nil {
		return fmt.Errorf("get keep contact: %w", err)
	}
	if keep.TenantID != tenantID {
		return fmt.Errorf("keep contact does not belong to tenant")
	}

	mergeInto, err := repo.GetByID(mergeIntoID)
	if err != nil {
		return fmt.Errorf("get merge contact: %w", err)
	}
	if mergeInto.TenantID != tenantID {
		return fmt.Errorf("merge contact does not belong to tenant")
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
