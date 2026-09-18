package contact

import (
	"fmt"

	"github.com/hekemen/automata/internal/domain/contact"
)

// GetActivity retrieves activities for a contact.
func GetActivity(repo Repository, contactID string, offset, limit int) ([]contact.Activity, error) {
	// Verify contact exists
	_, err := repo.GetByID(contactID)
	if err != nil {
		return nil, fmt.Errorf("get contact: %w", err)
	}

	activities, err := repo.GetActivity(contactID, offset, limit)
	if err != nil {
		return nil, fmt.Errorf("get activity: %w", err)
	}

	return activities, nil
}
