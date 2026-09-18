package form

import (
	"fmt"

	"github.com/hekemen/automata/internal/domain/form"
)

// ListForms returns all forms for the given tenant.
func ListForms(repo form.Repository, tenantID string) ([]*form.Form, error) {
	if tenantID == "" {
		return nil, fmt.Errorf("tenantID required")
	}

	forms, err := repo.List(tenantID)
	if err != nil {
		return nil, fmt.Errorf("list forms: %w", err)
	}

	return forms, nil
}
