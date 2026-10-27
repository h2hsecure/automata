// Package staticfile models files linked to a context (website) and served
// publicly. Each record maps a URL-friendly name to a file stored in the
// shared FileStore.
package staticfile

import "time"

// StaticFile is a file attached to a context and exposed at
// /static/<contextID>/<name>.
type StaticFile struct {
	ID        string
	ContextID string
	Name      string // URL-friendly name, e.g. "logo.png"
	Token     string // stored file name in the FileStore
	Mime      string
	Size      int64
	CreatedAt time.Time
}

// Repository persists static file records.
type Repository interface {
	// Create stores a new record. (ContextID, Name) must be unique.
	Create(f *StaticFile) error
	// GetByName looks up a record by context and URL name.
	GetByName(contextID, name string) (*StaticFile, error)
	// List returns all records for a context.
	List(contextID string) ([]*StaticFile, error)
	// Delete removes a record by context and URL name.
	Delete(contextID, name string) error
}