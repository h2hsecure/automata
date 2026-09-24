package form

import (
	"fmt"

	"github.com/hekemen/automata/internal/domain/form"
)

// UpdateForm updates an existing form, verifying context ownership.
func UpdateForm(repo form.Repository, contextID, id string, input CreateFormInput) (*form.Form, error) {
	if contextID == "" {
		return nil, fmt.Errorf("contextID required")
	}
	if id == "" {
		return nil, fmt.Errorf("form ID required")
	}

	f, err := repo.GetByID(id)
	if err != nil {
		return nil, fmt.Errorf("get form: %w", err)
	}

	if f.ContextID != contextID {
		return nil, fmt.Errorf("form does not belong to context")
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
