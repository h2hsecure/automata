package context

import (
	"errors"
	"fmt"
	"regexp"
	"time"
	"unicode/utf8"
)

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

// User represents a platform-level user account.
// Context membership is tracked via UserContext records.
type User struct {
	ID              string
	ContextID       string  // deprecated; kept for backward compat with context_users
	Email           string
	PasswordHash    string  // empty for SSO users
	SSOProvider     *string // nil for local auth
	SSOID           *string // nil for local auth
	IsAdmin         bool
	IsOwner         bool   // deprecated; kept for backward compat
	DisplayName     *string // nil means unset
	AvatarURL       *string // nil means no avatar
	PasswordChangedAt time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// UserContext represents a user's membership in a context.
type UserContext struct {
	ID        string
	UserID    string
	ContextID string
	Role      string // "owner", "admin", or "member"
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Validate checks that the User has all required fields set.
func (u *User) Validate() error {
	if u.Email == "" {
		return errors.New("email is required")
	}
	if !emailRegex.MatchString(u.Email) {
		return fmt.Errorf("invalid email format: %s", u.Email)
	}
	// Password required only for local auth users (not SSO)
	if u.PasswordHash == "" && (u.SSOProvider == nil || *u.SSOProvider == "") {
		return errors.New("password hash is required for local auth users")
	}
	if utf8.RuneCountInString(u.Email) > 254 {
		return errors.New("email must be 254 characters or less")
	}
	return nil
}
