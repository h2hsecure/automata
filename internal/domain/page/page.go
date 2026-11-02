package page

import (
	"errors"
	"time"
)

// Page is a per-context editable HTML page. It is served publicly at
// /page/:contextID/:slug and editable in the dashboard.
type Page struct {
	ID        string
	ContextID string
	Slug      string
	Title     string
	Content   string
	IsActive  bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Validate checks that the Page has the required fields set.
func (p *Page) Validate() error {
	if p.Slug == "" {
		return errors.New("slug required")
	}
	if p.Title == "" {
		return errors.New("title required")
	}
	return nil
}