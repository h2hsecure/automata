package contact

import (
	"fmt"

	"github.com/hekemen/automata/internal/domain/contact"
)

// GetContact retrieves a contact by ID after verifying it belongs to the tenant.
func GetContact(repo Repository, id, tenantID string) (*contact.Contact, error) {
	if tenantID == "" {
		return nil, fmt.Errorf("tenant_id is required")
	}

	c, err := repo.GetByID(id)
	if err != nil {
		return nil, fmt.Errorf("get contact: %w", err)
	}

	if c.TenantID != tenantID {
		return nil, fmt.Errorf("contact does not belong to tenant")
	}

	return c, nil
}
