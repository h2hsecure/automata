package contact

import (
	"fmt"

	"github.com/hekemen/automata/internal/domain/contact"
)

// ListOptions holds the options for listing contacts.
type ListOptions struct {
	Offset int
	Limit  int
	Filters contact.FilterOptions
}

// ListContacts retrieves a paginated list of contacts for a context with optional filters.
// Returns a ContactWithCount containing the contacts and total count.
func ListContacts(repo Repository, contextID string, opts ListOptions) (*contact.ContactWithCount, error) {
	if contextID == "" {
		return nil, fmt.Errorf("context_id is required")
	}

	contacts, err := repo.List(opts.Offset, opts.Limit, opts.Filters)
	if err != nil {
		return nil, fmt.Errorf("list contacts: %w", err)
	}

	total, err := repo.CountByContext(contextID)
	if err != nil {
		return nil, fmt.Errorf("count contacts: %w", err)
	}

	return &contact.ContactWithCount{
		Contacts: contacts,
		Total:    total,
	}, nil
}
