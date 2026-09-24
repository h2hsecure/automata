package form

import (
	"fmt"

	"github.com/hekemen/automata/internal/domain/form"
)

// DeleteForm deletes a form, verifying context ownership.
func DeleteForm(repo form.Repository, contextID, id string) error {
	if contextID == "" {
		return fmt.Errorf("contextID required")
	}
	if id == "" {
		return fmt.Errorf("form ID required")
	}

	f, err := repo.GetByID(id)
	if err != nil {
		return fmt.Errorf("get form: %w", err)
	}

	if f.ContextID != contextID {
		return fmt.Errorf("form does not belong to context")
	}

	if err := repo.Delete(id); err != nil {
		return fmt.Errorf("delete form: %w", err)
	}

	return nil
}
