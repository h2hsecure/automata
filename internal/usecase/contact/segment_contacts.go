package contact

import (
	"fmt"

	"github.com/hekemen/automata/internal/domain/contact"
)

// SegmentContacts retrieves contacts matching the given filters.
func SegmentContacts(repo Repository, tenantID string, filters contact.FilterOptions) ([]*contact.Contact, error) {
	if tenantID == "" {
		return nil, fmt.Errorf("tenant_id is required")
	}

	contacts, err := repo.List(0, 0, filters)
	if err != nil {
		return nil, fmt.Errorf("segment contacts: %w", err)
	}

	return contacts, nil
}
