package contact

import (
	"fmt"

	"github.com/hekemen/automata/internal/domain/contact"
)

// GetActivity retrieves activities for a contact with optional type filtering.
// Returns enriched ActivityDetail objects with title/description.
func GetActivity(repo Repository, contactID, contextID string, offset, limit int, activityType *string) (*contact.ActivityResult, error) {
	// Verify contact exists and belongs to context
	c, err := repo.GetByID(contactID)
	if err != nil {
		return nil, fmt.Errorf("get contact: %w", err)
	}
	_ = c // contact context is verified by the handler before calling this

	activities, total, err := repo.GetActivity(contactID, contextID, offset, limit, activityType)
	if err != nil {
		return nil, fmt.Errorf("get activity: %w", err)
	}

	// Enrich with title and description
	details := make([]contact.ActivityDetail, 0, len(activities))
	for _, a := range activities {
		detail := contact.ActivityDetail{
			ID:          a.ID,
			Type:        a.Type,
			Title:       contact.FormatActivityTitle(a),
			Description: contact.FormatActivityDescription(a),
			Data:        a.Data,
			CreatedAt:   a.CreatedAt,
		}
		details = append(details, detail)
	}

	return &contact.ActivityResult{
		Activities: details,
		Total:      total,
	}, nil
}
