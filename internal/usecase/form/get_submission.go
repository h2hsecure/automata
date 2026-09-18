package form

import (
	"fmt"

	"github.com/hekemen/automata/internal/domain/form"
)

// GetSubmission retrieves a submission by ID.
func GetSubmission(repo form.Repository, id string) (*form.Submission, error) {
	if id == "" {
		return nil, fmt.Errorf("submission ID required")
	}

	s, err := repo.GetSubmission(id)
	if err != nil {
		return nil, fmt.Errorf("get submission: %w", err)
	}

	return s, nil
}
