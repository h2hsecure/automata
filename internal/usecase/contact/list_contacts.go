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

// ListContacts retrieves a paginated list of contacts for a tenant with optional filters.
// Returns a ContactWithCount containing the contacts and total count.
func ListContacts(repo Repository, tenantID string, opts ListOptions) (*contact.ContactWithCount, error) {
	if tenantID == "" {
		return nil, fmt.Errorf("tenant_id is required")
	}

	contacts, err := repo.List(opts.Offset, opts.Limit, opts.Filters)
	if err != nil {
		return nil, fmt.Errorf("list contacts: %w", err)
	}

	total, err := repo.CountByTenant(tenantID)
	if err != nil {
		return nil, fmt.Errorf("count contacts: %w", err)
	}

	return &contact.ContactWithCount{
		Contacts: contacts,
		Total:    total,
	}, nil
}
