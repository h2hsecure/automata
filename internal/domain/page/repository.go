package page

// Repository defines the interface for page data persistence.
type Repository interface {
	Create(p *Page) error
	GetByID(id string) (*Page, error)
	GetBySlug(contextID, slug string) (*Page, error)
	List(contextID string) ([]*Page, error)
	ListActive(contextID string) ([]*Page, error)
	Update(p *Page) error
	Delete(id string) error
	RecordImpression(contextID, pageID, visitorID string) error
	RecordDecision(d *Decision) error
	ListDecisions(pageID string, offset, limit int) ([]*Decision, int64, error)
	DecisionCounts(pageID string) (map[string]int64, error)
}
