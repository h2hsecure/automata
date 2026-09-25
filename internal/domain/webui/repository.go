package webui

// SessionRepository defines the interface for session persistence.
type SessionRepository interface {
	// Create persists a session and returns the session ID.
	Create(session *Session) (string, error)

	// GetByToken retrieves a session by its token.
	GetByToken(token string) (*Session, error)

	// Delete removes a session by its token.
	Delete(token string) error

	// Invalidate removes all sessions for a given user.
	InvalidateUser(userID string) error
}
