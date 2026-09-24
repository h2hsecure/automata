package form

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/hekemen/automata/internal/domain/form"
	"github.com/hekemen/automata/pkg/validation"
)

// SubmitFormInput holds the input for submitting a form.
type SubmitFormInput struct {
	Slug    string
	Data    map[string]interface{}
	Files   []form.FileUpload
}

// SubmitForm submits a form by slug, validating data against form fields.
func SubmitForm(repo form.Repository, contextID string, input SubmitFormInput) (*form.Submission, error) {
	if contextID == "" {
		return nil, fmt.Errorf("contextID required")
	}

	f, err := repo.GetBySlug(contextID, input.Slug)
	if err != nil {
		return nil, fmt.Errorf("get form: %w", err)
	}

	if err := validation.Validate(input.Data, f.Fields); err != nil {
		return nil, fmt.Errorf("validate submission: %w", err)
	}

	if honeypot, ok := input.Data["_honeypot"].(string); ok && honeypot != "" {
		return nil, fmt.Errorf("spam detected")
	}

	s := &form.Submission{
		ID:       uuid.New().String(),
		FormID:   f.ID,
		ContextID: contextID,
		Data:     input.Data,
		Files:    input.Files,
	}

	if err := repo.CreateSubmission(s); err != nil {
		return nil, fmt.Errorf("create submission: %w", err)
	}

	return s, nil
}
