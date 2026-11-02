package page

// Repository defines the interface for page data persistence.
type Repository interface {
	Create(p *Page) error
	GetByID(id string) (*Page, error)
	GetBySlug(contextID, slug string) (*Page, error)
	List(contextID string) ([]*Page, error)
	Update(p *Page) error
	Delete(id string) error
}