package page

import (
	"fmt"

	"github.com/h2hsecure/automata/internal/domain/page"
)

// UpdatePage updates an existing page, verifying context ownership.
func UpdatePage(repo page.Repository, contextID, id string, input CreatePageInput) (*page.Page, error) {
	if contextID == "" {
		return nil, fmt.Errorf("contextID required")
	}
	if id == "" {
		return nil, fmt.Errorf("page ID required")
	}

	p, err := repo.GetByID(id)
	if err != nil {
		return nil, fmt.Errorf("get page: %w", err)
	}

	if p.ContextID != contextID {
		return nil, fmt.Errorf("page does not belong to context")
	}

	p.Slug = input.Slug
	p.Title = input.Title
	p.Content = input.Content
	if input.IsActive != nil {
		p.IsActive = *input.IsActive
	}

	if err := p.Validate(); err != nil {
		return nil, fmt.Errorf("invalid page: %w", err)
	}

	if err := repo.Update(p); err != nil {
		return nil, fmt.Errorf("update page: %w", err)
	}

	return p, nil
}
