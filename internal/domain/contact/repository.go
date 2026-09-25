package contact

// FilterOptions holds optional filters for contact list queries.
type FilterOptions struct {
	Tags         []string
	Source       string
	MinSubmissions int
	Company      string
	Search       string
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
}
