package webui

import "time"

// Session represents an authenticated user session for the Web UI.
type Session struct {
	UserID      string
	Email       string
	IsAdmin     bool
	ContextID   string
	ContextName string
	Token       string
	CreatedAt   time.Time
	ExpiresAt   time.Time
}
