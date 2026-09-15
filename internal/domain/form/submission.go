package form

import "time"

// FileUpload represents a file attached to a form submission.
type FileUpload struct {
	Field string `json:"field"`
	Name  string `json:"name"`
	Size  int64  `json:"size"`
	Type  string `json:"type"`
	URL   string `json:"url"`
}

// Submission represents a completed form submission.
type Submission struct {
	ID        string
	FormID    string
	TenantID  string
	Data      map[string]interface{}
	Files     []FileUpload
	CreatedAt time.Time
}
