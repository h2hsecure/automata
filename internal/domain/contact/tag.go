package contact

import (
	"errors"
	"time"
)

// Tag represents a contact tag for categorization.
type Tag struct {
	ID        string
	TenantID  string
	Name      string
	Color     string
	CreatedAt time.Time
}

// Validate checks that the Tag has all required fields set.
func (t *Tag) Validate() error {
	if t.Name == "" {
		return errors.New("tag name required")
	}
	return nil
}
