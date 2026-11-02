package page

import (
	"fmt"

	"github.com/h2hsecure/automata/internal/domain/page"
)

// DeletePage deletes a page, verifying context ownership.
func DeletePage(repo page.Repository, contextID, id string) error {
	if contextID == "" {
		return fmt.Errorf("contextID required")
	}
	if id == "" {
		return fmt.Errorf("page ID required")
	}

	p, err := repo.GetByID(id)
	if err != nil {
		return fmt.Errorf("get page: %w", err)
	}

	if p.ContextID != contextID {
		return fmt.Errorf("page does not belong to context")
	}

	if err := repo.Delete(id); err != nil {
		return fmt.Errorf("delete page: %w", err)
	}

	return nil
}
