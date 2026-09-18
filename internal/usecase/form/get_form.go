package form

import (
	"fmt"

	"github.com/hekemen/automata/internal/domain/form"
)

// GetForm retrieves a form by slug for the given tenant.
func GetForm(repo form.Repository, tenantID, slug string) (*form.Form, error) {
	if tenantID == "" {
		return nil, fmt.Errorf("tenantID required")
	}

	f, err := repo.GetBySlug(tenantID, slug)
	if err != nil {
		return nil, fmt.Errorf("get form: %w", err)
	}

	return f, nil
}
