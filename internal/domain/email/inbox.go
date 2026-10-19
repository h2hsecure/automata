package email

import "time"

// InboxMessage is a single email fetched from a context's IMAP INBOX.
type InboxMessage struct {
	ID          string
	ContextID   string
	UID         int64
	From        string
	To          string
	Subject     string
	BodyText    string
	MessageDate time.Time
	IsRead      bool
	CreatedAt   time.Time
}

// InboxRepository defines persistence for the per-context IMAP inbox.
type InboxRepository interface {
	// List returns messages newest-first, capped at limit.
	List(contextID string, limit int) ([]*InboxMessage, error)
	// MarkRead flags a message as read; returns sql.ErrNoRows when not found.
	MarkRead(id string) error
	// Upsert inserts a message, deduping on (context_id, uid).
	Upsert(m *InboxMessage) error
	// MaxUID returns the highest stored UID for the context, 0 when empty.
	MaxUID(contextID string) (int64, error)
	// TrimTo keeps only the newest max messages for the context.
	TrimTo(contextID string, max int) error
}
