package form

// Repository defines the interface for form data persistence.
type Repository interface {
	Create(f *Form) error
	GetByID(id string) (*Form, error)
	GetBySlug(contextID, slug string) (*Form, error)
	List(contextID string) ([]*Form, error)
	Update(f *Form) error
	Delete(id string) error
	CreateSubmission(s *Submission) error
	GetSubmission(id string) (*Submission, error)
	ListSubmissions(formID string, opts ListSubmissionsOptions) ([]*Submission, int64, error)
}

// ListSubmissionsOptions holds pagination options for listing submissions.
type ListSubmissionsOptions struct {
	Offset int
	Limit  int
}
