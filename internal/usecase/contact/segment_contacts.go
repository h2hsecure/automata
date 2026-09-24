package contact

import (
	"fmt"

	"github.com/hekemen/automata/internal/domain/contact"
)

// SegmentContacts retrieves contacts matching the given filters.
func SegmentContacts(repo Repository, contextID string, filters contact.FilterOptions) ([]*contact.Contact, error) {
	if contextID == "" {
		return nil, fmt.Errorf("context_id is required")
	}

	contacts, err := repo.List(0, 0, filters)
	if err != nil {
		return nil, fmt.Errorf("segment contacts: %w", err)
	}

	return contacts, nil
}
