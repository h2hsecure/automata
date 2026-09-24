package form

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/hekemen/automata/internal/domain/form"
)

// CreateFormInput holds the input for creating a new form.
type CreateFormInput struct {
	Slug        string
	Name        string
	Description string
	Fields      []form.FormField
	Settings    map[string]interface{}
}

// CreateForm creates a new form for the given context.
func CreateForm(repo form.Repository, contextID string, input CreateFormInput) (*form.Form, error) {
	if contextID == "" {
		return nil, fmt.Errorf("contextID required")
	}

	f := &form.Form{
		ID:          uuid.New().String(),
		ContextID:    contextID,
		Slug:        input.Slug,
		Name:        input.Name,
		Description: input.Description,
		Fields:      input.Fields,
		Settings:    input.Settings,
	}

	if err := f.Validate(); err != nil {
		return nil, fmt.Errorf("invalid form: %w", err)
	}

	if err := repo.Create(f); err != nil {
		return nil, fmt.Errorf("create form: %w", err)
	}

	return f, nil
}
