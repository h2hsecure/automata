package contact

import (
	"fmt"
)

// DeleteContact removes a contact by ID after verifying it belongs to the tenant.
func DeleteContact(repo Repository, id, tenantID string) error {
	if tenantID == "" {
		return fmt.Errorf("tenant_id is required")
	}

	// Get existing contact to verify it belongs to tenant
	existing, err := repo.GetByID(id)
	if err != nil {
		return fmt.Errorf("get contact: %w", err)
	}

	if existing.TenantID != tenantID {
		return fmt.Errorf("contact does not belong to tenant")
	}

	// Delete contact (tags cascade via FK)
	if err := repo.Delete(id); err != nil {
		return fmt.Errorf("delete contact: %w", err)
	}

	return nil
}
