package form

import (
	"errors"
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

// Validate checks that the Form has all required fields set.
func (f *Form) Validate() error {
	if f.Slug == "" {
		return errors.New("slug required")
	}
	if len(f.Fields) == 0 {
		return errors.New("at least one field required")
	}
	return nil
}
