package contact

import "time"

// FilterOptions holds optional filters for contact list queries.
type FilterOptions struct {
	Tags           []string
	Source         string
	MinSubmissions int
	Company        string
	Search         string
	// Hidden filters on the hidden flag: nil = no filter, true = hidden
	// only, false = exclude hidden contacts.
	Hidden *bool
	// ContextID scopes the query to a single context; empty = no scope.
	ContextID string
}

// Repository defines the interface for contact data persistence.
type Repository interface {
	Create(c *Contact) error
	GetByID(id string) (*Contact, error)
	List(offset, limit int, filters FilterOptions) ([]*Contact, error)
	Update(c *Contact) error
	Delete(id string) error
	FindByEmail(contextID, email string) (*Contact, error)
	Merge(keepID, mergeIntoID string) error
	CreateActivity(a *Activity) error
	GetActivity(contactID, contextID string, offset, limit int, activityType *string) ([]Activity, int64, error)
	GetActivityCount(contactID string) (int64, error)
	CountByContext(contextID string) (int64, error)
	CountDistinctContactsByActivity(contextID, activityType string, since *time.Time) (int, error)
	// FindBySourceID returns the contact for (source, source_id) in a
	// context, or (nil, nil) when absent.
	FindBySourceID(contextID, source, sourceID string) (*Contact, error)
	// Count returns the number of contacts matching the filters.
	Count(contextID string, filters FilterOptions) (int64, error)
}
