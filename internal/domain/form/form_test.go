package form

import (
	"testing"
)

func TestFormValidation(t *testing.T) {
	tests := []struct {
		name    string
		form    *Form
		wantErr bool
	}{
		{
			name:    "empty slug",
			form:    &Form{},
			wantErr: true,
		},
		{
			name:    "slug but no fields",
			form:    &Form{Slug: "contact"},
			wantErr: true,
		},
		{
			name:    "valid form",
			form:    &Form{Slug: "contact", Fields: []FormField{{Slug: "name", Type: "text"}}},
			wantErr: false,
		},
		{
			name:    "valid form with multiple fields",
			form:    &Form{Slug: "survey", Fields: []FormField{{Slug: "name", Type: "text"}, {Slug: "email", Type: "email"}}},
			wantErr: false,
		},
		{
			name:    "valid form with settings",
			form:    &Form{Slug: "feedback", Fields: []FormField{{Slug: "comment", Type: "textarea"}}, Settings: map[string]interface{}{"webhook_url": "https://example.com"}},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.form.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSubmissionEntity(t *testing.T) {
	s := &Submission{
		FormID:   "550e8400-e29b-41d4-a716-446655440000",
		ContextID: "550e8400-e29b-41d4-a716-446655440001",
		Data:     map[string]interface{}{"name": "John", "email": "john@example.com"},
		Files:    []FileUpload{{Field: "resume", Name: "cv.pdf", Size: 1024, Type: "application/pdf", URL: "/uploads/cv.pdf"}},
	}

	if s.FormID == "" {
		t.Error("FormID should be set")
	}
	if s.Data == nil {
		t.Error("Data should not be nil")
	}
	if len(s.Files) != 1 {
		t.Errorf("expected 1 file, got %d", len(s.Files))
	}
}

func TestFormField(t *testing.T) {
	f := FormField{
		Slug:     "email",
		Type:     "email",
		Label:    "Email Address",
		Required: true,
		Validation: map[string]interface{}{
			"pattern": `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`,
		},
		Order: 1,
	}

	if f.Slug != "email" {
		t.Errorf("expected slug 'email', got '%s'", f.Slug)
	}
	if !f.Required {
		t.Error("expected Required to be true")
	}
	if f.Order != 1 {
		t.Errorf("expected order 1, got %d", f.Order)
	}
}
