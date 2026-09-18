package form

import (
	"fmt"

	"github.com/hekemen/automata/internal/domain/form"
)

// ListSubmissions returns submissions for a form with optional pagination.
func ListSubmissions(repo form.Repository, formID string, opts form.ListSubmissionsOptions) ([]*form.Submission, int64, error) {
	if formID == "" {
		return nil, 0, fmt.Errorf("formID required")
	}

	submissions, total, err := repo.ListSubmissions(formID, opts)
	if err != nil {
		return nil, 0, fmt.Errorf("list submissions: %w", err)
	}

	return submissions, total, nil
}
