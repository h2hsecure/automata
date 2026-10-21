package form

import (
	"fmt"

	"github.com/hekemen/automata/internal/domain/form"
)

// ListSubmissions returns submissions for a form with optional pagination,
// scoped to the context (the form must belong to it).
func ListSubmissions(repo form.Repository, contextID, formID string, opts form.ListSubmissionsOptions) ([]*form.Submission, int64, error) {
	if formID == "" {
		return nil, 0, fmt.Errorf("formID required")
	}

	f, err := repo.GetByID(formID)
	if err != nil || f.ContextID != contextID {
		return nil, 0, fmt.Errorf("form not found")
	}

	submissions, total, err := repo.ListSubmissions(formID, opts)
	if err != nil {
		return nil, 0, fmt.Errorf("list submissions: %w", err)
	}

	return submissions, total, nil
}
