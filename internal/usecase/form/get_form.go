package form

import (
	"fmt"

	"github.com/hekemen/automata/internal/domain/form"
)

// GetForm retrieves a form by slug for the given context.
func GetForm(repo form.Repository, contextID, slug string) (*form.Form, error) {
	if contextID == "" {
		return nil, fmt.Errorf("contextID required")
	}

	f, err := repo.GetBySlug(contextID, slug)
	if err != nil {
		return nil, fmt.Errorf("get form: %w", err)
	}

	return f, nil
}
