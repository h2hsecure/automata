package page

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/h2hsecure/automata/internal/domain/page"
)

// CreatePageInput is the request payload for page create and update.
type CreatePageInput struct {
	Slug     string
	Title    string
	Content  string
	IsActive *bool
}

// CreatePage validates and creates a page for the given context.
func CreatePage(repo page.Repository, contextID string, input CreatePageInput) (*page.Page, error) {
	if contextID == "" {
		return nil, fmt.Errorf("contextID required")
	}

	p := &page.Page{
		ID:        uuid.New().String(),
		ContextID: contextID,
		Slug:      input.Slug,
		Title:     input.Title,
		Content:   input.Content,
		IsActive:  true,
	}
	if input.IsActive != nil {
		p.IsActive = *input.IsActive
	}

	if err := p.Validate(); err != nil {
		return nil, fmt.Errorf("invalid page: %w", err)
	}

	if err := repo.Create(p); err != nil {
		return nil, fmt.Errorf("create page: %w", err)
	}

	return p, nil
}
