package contact

import (
	"errors"
	"fmt"
	"time"
)

// Contact represents a marketing contact.
type Contact struct {
	ID           string
	TenantID     string
	Email        *string
	FirstName    string
	LastName     string
	Phone        string
	Company      string
	CustomFields map[string]interface{}
	Source       string
	SourceID     string
	Tags         []Tag
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Validate checks that the Contact has all required fields set.
func (c *Contact) Validate() error {
	if c.TenantID == "" {
		return errors.New("tenant_id required")
	}
	if c.Email != nil && *c.Email != "" {
		if !isValidEmail(*c.Email) {
			return errors.New("invalid email format")
		}
	}
	return nil
}

// IsVerified returns true if the contact has a non-empty email.
func (c *Contact) IsVerified() bool {
	return c.Email != nil && *c.Email != ""
}

func isValidEmail(email string) bool {
	if len(email) == 0 || len(email) > 254 {
		return false
	}
	atIdx := -1
	for i, ch := range email {
		if ch == '@' {
			atIdx = i
		}
	}
	if atIdx <= 0 || atIdx >= len(email)-1 {
		return false
	}
	local := email[:atIdx]
	domain := email[atIdx+1:]
	if len(local) == 0 || len(domain) == 0 {
		return false
	}
	if len(local) > 64 {
		return false
	}
	for _, ch := range local {
		if (ch < 33 || ch > 126) && ch != '.' && ch != '_' && ch != '-' && ch != '+' {
			return false
		}
	}
	if len(domain) > 253 {
		return false
	}
	return true
}

// ContactWithCount wraps a list of contacts with a total count.
type ContactWithCount struct {
	Contacts []*Contact
	Total    int64
}

// ErrDuplicateEmail is returned when a contact with the same email already exists.
var ErrDuplicateEmail = fmt.Errorf("contact with this email already exists")
