package email

import "fmt"

var (
	ErrNotFound     = fmt.Errorf("email template not found")
	ErrDuplicateKey = fmt.Errorf("template key already exists for this context")
)

// Repository defines the persistence interface for email templates.
type Repository interface {
	Create(t *EmailTemplate) error
	GetByID(id string) (*EmailTemplate, error)
	GetByKey(contextID, key string) (*EmailTemplate, error)
	List(contextID string) ([]*EmailTemplate, error)
	Update(t *EmailTemplate) error
	Delete(id string) error
	SeedDefaults(contextID string) error
}
