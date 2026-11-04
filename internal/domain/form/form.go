package form

import (
	"errors"
	"fmt"
	"time"
)

// FormField represents a single field in a form.
type FormField struct {
	Slug       string                 `json:"slug"`
	Type       string                 `json:"type"`
	Label      string                 `json:"label"`
	Required   bool                   `json:"required"`
	Validation map[string]interface{} `json:"validation"`
	Order      int                    `json:"order"`
}

// Form represents a user-defined form with fields and settings.
type Form struct {
	ID          string
	ContextID   string
	Slug        string
	Name        string
	Description string
	Fields      []FormField
	Settings    map[string]interface{}
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Validate checks that the Form has all required fields set and that no
// field slug is duplicated (duplicate slugs make submission values
// ambiguous and corrupt stored data).
func (f *Form) Validate() error {
	if f.Slug == "" {
		return errors.New("slug required")
	}
	if len(f.Fields) == 0 {
		return errors.New("at least one field required")
	}
	seen := make(map[string]struct{}, len(f.Fields))
	for _, field := range f.Fields {
		if _, ok := seen[field.Slug]; ok {
			return fmt.Errorf("duplicate field slug %q", field.Slug)
		}
		seen[field.Slug] = struct{}{}
	}
	return nil
}
