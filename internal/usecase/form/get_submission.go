package form

import (
	"fmt"

	"github.com/h2hsecure/automata/internal/domain/form"
)

// GetSubmission retrieves a submission by ID, scoped to the context.
func GetSubmission(repo form.Repository, contextID, id string) (*form.Submission, error) {
	if id == "" {
		return nil, fmt.Errorf("submission ID required")
	}

	s, err := repo.GetSubmission(id)
	if err != nil {
		return nil, fmt.Errorf("submission not found")
	}
	if s.ContextID != contextID {
		// Not-found (not "not yours"): foreign submissions are
		// indistinguishable from nonexistent ones.
		return nil, fmt.Errorf("submission not found")
	}

	return s, nil
}
