package tenant

import (
	"errors"
	"fmt"
	"time"
	"unicode/utf8"
)

// Tenant represents a multi-tenant workspace.
type Tenant struct {
	ID        string
	Slug      string
	Name      string
	Domain    *string
	IsActive  bool
	Settings  map[string]interface{}
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Validate checks that the Tenant has all required fields set.
func (t *Tenant) Validate() error {
	var errs []string

	if t.Slug == "" {
		errs = append(errs, "slug is required")
	} else if utf8.RuneCountInString(t.Slug) > 63 {
		errs = append(errs, "slug must be 63 characters or less")
	}

	if t.Name == "" {
		errs = append(errs, "name is required")
	} else if utf8.RuneCountInString(t.Name) > 255 {
		errs = append(errs, "name must be 255 characters or less")
	}

	if len(errs) > 0 {
		return errors.New(fmt.Sprintf("invalid tenant: %s", errs[0]))
	}

	return nil
}
