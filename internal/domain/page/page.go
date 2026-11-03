package page

import (
	"errors"
	"time"
)

// Position is where a page is rendered as an overlay/banner on the
// customer site. Empty means the page is a plain content page.
type Position string

const (
	PositionNone   Position = ""
	PositionTop    Position = "top"
	PositionBottom Position = "bottom"
	PositionLeft   Position = "left"
	PositionRight  Position = "right"
	PositionFull   Position = "full"
)

// Valid reports whether the Position is one of the supported values.
func (p Position) Valid() bool {
	switch p {
	case PositionNone, PositionTop, PositionBottom, PositionLeft, PositionRight, PositionFull:
		return true
	}
	return false
}

// Button is a call-to-action rendered inside a positioned page.
type Button struct {
	Label   string `json:"label"`
	Href    string `json:"href"`
	Action  string `json:"action"`  // "link" | "dismiss_accept" | "dismiss_decline"
	Variant string `json:"variant"` // "primary" | "secondary" | ...
}

// Page is a per-context editable HTML page. It is served publicly at
// /page/:contextID/:slug and editable in the dashboard. A page may also
// carry a Position and Buttons, in which case it is rendered as a
// banner-style overlay on the customer site.
type Page struct {
	ID        string
	ContextID string
	Slug      string
	Title     string
	Content   string
	IsActive  bool
	Position  Position
	Buttons   []Button
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
	if !p.Position.Valid() {
		return errors.New("invalid position")
	}
	return nil
}

// Decision records a visitor's choice (accept/decline) for a positioned page.
type Decision struct {
	ID        string
	ContextID string
	PageID    string
	VisitorID string
	Action    string
	IPHash    string
	UserAgent string
	CreatedAt time.Time
}

// Impression records that a positioned page was shown to a visitor.
type Impression struct {
	ID        string
	ContextID string
	PageID    string
	VisitorID string
	CreatedAt time.Time
}