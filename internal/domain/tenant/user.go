package tenant

import (
	"errors"
	"fmt"
	"regexp"
	"time"
	"unicode/utf8"
)

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

// User represents a tenant user.
type User struct {
	ID           string
	TenantID     string
	Email        string
	PasswordHash string
	IsOwner      bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Validate checks that the User has all required fields set.
func (u *User) Validate() error {
	if u.Email == "" {
		return errors.New("email is required")
	}
	if !emailRegex.MatchString(u.Email) {
		return fmt.Errorf("invalid email format: %s", u.Email)
	}
	if u.PasswordHash == "" {
		return errors.New("password hash is required")
	}
	if utf8.RuneCountInString(u.Email) > 254 {
		return errors.New("email must be 254 characters or less")
	}
	return nil
}
