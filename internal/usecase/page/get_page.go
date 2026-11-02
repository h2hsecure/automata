package page

import (
	"fmt"

	"github.com/h2hsecure/automata/internal/domain/page"
)

// GetPage retrieves a page by ID.
func GetPage(repo page.Repository, id string) (*page.Page, error) {
	if id == "" {
		return nil, fmt.Errorf("page ID required")
	}

	p, err := repo.GetByID(id)
	if err != nil {
		return nil, fmt.Errorf("get page: %w", err)
	}

	return p, nil
}

// GetPageBySlug retrieves a page by slug for the given context.
func GetPageBySlug(repo page.Repository, contextID, slug string) (*page.Page, error) {
	if contextID == "" {
		return nil, fmt.Errorf("contextID required")
	}

	p, err := repo.GetBySlug(contextID, slug)
	if err != nil {
		return nil, fmt.Errorf("get page: %w", err)
	}

	return p, nil
}
