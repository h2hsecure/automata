package page

import (
	"fmt"

	"github.com/h2hsecure/automata/internal/domain/page"
)

// ListPages returns all pages for the given context.
func ListPages(repo page.Repository, contextID string) ([]*page.Page, error) {
	if contextID == "" {
		return nil, fmt.Errorf("contextID required")
	}

	pages, err := repo.List(contextID)
	if err != nil {
		return nil, fmt.Errorf("list pages: %w", err)
	}

	return pages, nil
}
