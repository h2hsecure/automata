package form

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/hekemen/automata/internal/domain/form"
	"github.com/hekemen/automata/pkg/validation"
)

// SubmitFormInput holds the input for submitting a form.
type SubmitFormInput struct {
	Slug      string
	Data      map[string]interface{}
	Files     []form.FileUpload
	VisitorID string
}

// FileResolver resolves an upload token (the stored file name) into a
// FileUpload record for a submission.
type FileResolver interface {
	Resolve(contextID, token string) (form.FileUpload, error)
}

// SubmitForm submits a form by slug, validating data against form fields.
// When resolver is non-nil, tokens found in file-typed field values are
// resolved into Submission.Files.
func SubmitForm(repo form.Repository, contextID string, input SubmitFormInput, resolver FileResolver) (*form.Submission, error) {
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

	files := input.Files
	if resolver != nil {
		for _, field := range f.Fields {
			if field.Type != "file" {
				continue
			}
			token, _ := input.Data[field.Slug].(string)
			if token == "" {
				continue
			}
			fu, err := resolver.Resolve(contextID, token)
			if err != nil {
				return nil, fmt.Errorf("resolve file upload: %w", err)
			}
			fu.Field = field.Slug
			files = append(files, fu)
		}
	}

	s := &form.Submission{
		ID:       uuid.New().String(),
		FormID:   f.ID,
		ContextID: contextID,
		Data:     input.Data,
		Files:    files,
	}

	if err := repo.CreateSubmission(s); err != nil {
		return nil, fmt.Errorf("create submission: %w", err)
	}

	return s, nil
}
