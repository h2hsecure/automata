package form

import (
	"fmt"

	"github.com/hekemen/automata/internal/domain/form"
)

// ListForms returns all forms for the given context.
func ListForms(repo form.Repository, contextID string) ([]*form.Form, error) {
	if contextID == "" {
		return nil, fmt.Errorf("contextID required")
	}

	forms, err := repo.List(contextID)
	if err != nil {
		return nil, fmt.Errorf("list forms: %w", err)
	}

	return forms, nil
}
