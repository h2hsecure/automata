package form

import (
	"fmt"

	"github.com/hekemen/automata/internal/domain/form"
)

// DeleteForm deletes a form, verifying tenant ownership.
func DeleteForm(repo form.Repository, tenantID, id string) error {
	if tenantID == "" {
		return fmt.Errorf("tenantID required")
	}
	if id == "" {
		return fmt.Errorf("form ID required")
	}

	f, err := repo.GetByID(id)
	if err != nil {
		return fmt.Errorf("get form: %w", err)
	}

	if f.TenantID != tenantID {
		return fmt.Errorf("form does not belong to tenant")
	}

	if err := repo.Delete(id); err != nil {
		return fmt.Errorf("delete form: %w", err)
	}

	return nil
}
