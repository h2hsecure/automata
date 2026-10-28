package form

import (
	"fmt"

	"github.com/hekemen/automata/internal/domain/form"
)

// DeleteSubmission deletes a submission by ID, scoped to the context.
// Foreign-context and missing submissions are both reported as not found,
// so they are indistinguishable from nonexistent ones.
func DeleteSubmission(repo form.Repository, contextID, id string) error {
	if contextID == "" {
		return fmt.Errorf("contextID required")
	}
	if id == "" {
		return fmt.Errorf("submission ID required")
	}

	s, err := repo.GetSubmission(id)
	if err != nil {
		return fmt.Errorf("submission not found")
	}
	if s.ContextID != contextID {
		return fmt.Errorf("submission not found")
	}

	if err := repo.DeleteSubmission(id); err != nil {
		return fmt.Errorf("delete submission: %w", err)
	}

	return nil
}
