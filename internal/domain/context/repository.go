package context

// Repository defines the interface for context data persistence.
type Repository interface {
	Create(c *Context) error
	GetByID(id string) (*Context, error)
	GetBySlug(slug string) (*Context, error)
	List(offset, limit int) ([]*Context, error)
	Update(c *Context) error
	Delete(id string) error
	GetByUserEmail(email string) ([]*Context, error)
	GetDefaultContext() (*Context, error)
}
