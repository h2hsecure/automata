package tenant

// Repository defines the interface for tenant data persistence.
type Repository interface {
	Create(t *Tenant) error
	GetByID(id string) (*Tenant, error)
	GetBySlug(slug string) (*Tenant, error)
	List(offset, limit int) ([]*Tenant, error)
	Update(t *Tenant) error
	Delete(id string) error
}
