package form

import (
	"fmt"

	"github.com/hekemen/automata/internal/domain/form"
)

// UpdateForm updates an existing form, verifying tenant ownership.
func UpdateForm(repo form.Repository, tenantID, id string, input CreateFormInput) (*form.Form, error) {
	if tenantID == "" {
		return nil, fmt.Errorf("tenantID required")
	}
	if id == "" {
		return nil, fmt.Errorf("form ID required")
	}

	f, err := repo.GetByID(id)
	if err != nil {
		return nil, fmt.Errorf("get form: %w", err)
	}

	if f.TenantID != tenantID {
		return nil, fmt.Errorf("form does not belong to tenant")
	}

	f.Slug = input.Slug
	f.Name = input.Name
	f.Description = input.Description
	f.Fields = input.Fields
	f.Settings = input.Settings

	if err := f.Validate(); err != nil {
		return nil, fmt.Errorf("invalid form: %w", err)
	}

	if err := repo.Update(f); err != nil {
		return nil, fmt.Errorf("update form: %w", err)
	}

	return f, nil
}
