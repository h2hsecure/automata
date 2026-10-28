package repo

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/h2hsecure/automata/internal/domain/email"
	"github.com/jackc/pgx/v5/pgxpool"
)

// inboxRepo implements the email.InboxRepository interface using PostgreSQL.
type inboxRepo struct {
	pool *pgxpool.Pool
}

// NewInboxRepo creates a new PostgreSQL inbox repository.
func NewInboxRepo(pool *pgxpool.Pool) email.InboxRepository {
	return &inboxRepo{pool: pool}
}

const inboxColumns = `id, context_id, uid, sender, recipient, subject, body_text, message_date, is_read, created_at`

func (r *inboxRepo) List(contextID string, limit int) ([]*email.InboxMessage, error) {
	if limit <= 0 {
		limit = 50
	}
	query := fmt.Sprintf(
		`SELECT %s FROM email_inbox WHERE context_id = $1 ORDER BY message_date DESC, uid DESC LIMIT $2`,
		inboxColumns,
	)
	rows, err := r.pool.Query(context.Background(), query, contextID, limit)
	if err != nil {
		return nil, fmt.Errorf("list inbox: %w", err)
	}
	defer rows.Close()

	msgs := make([]*email.InboxMessage, 0)
	for rows.Next() {
		m := &email.InboxMessage{}
		if err := rows.Scan(&m.ID, &m.ContextID, &m.UID, &m.From, &m.To, &m.Subject,
			&m.BodyText, &m.MessageDate, &m.IsRead, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan inbox message: %w", err)
		}
		msgs = append(msgs, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate inbox: %w", err)
	}
	return msgs, nil
}

// MarkRead flags a message as read. Returns sql.ErrNoRows when no message is updated.
func (r *inboxRepo) MarkRead(id string) error {
	res, err := r.pool.Exec(context.Background(),
		`UPDATE email_inbox SET is_read = true WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("mark inbox read: %w", err)
	}
	if res.RowsAffected() == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// Upsert inserts a message, updating the existing row on (context_id, uid) conflict.
func (r *inboxRepo) Upsert(m *email.InboxMessage) error {
	if m.ID == "" {
		m.ID = uuid.New().String()
	}
	_, err := r.pool.Exec(context.Background(),
		`INSERT INTO email_inbox (id, context_id, uid, sender, recipient, subject, body_text, message_date)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 ON CONFLICT (context_id, uid) DO UPDATE SET
		   sender = EXCLUDED.sender,
		   recipient = EXCLUDED.recipient,
		   subject = EXCLUDED.subject,
		   body_text = EXCLUDED.body_text,
		   message_date = EXCLUDED.message_date`,
		m.ID, m.ContextID, m.UID, m.From, m.To, m.Subject, m.BodyText, m.MessageDate,
	)
	if err != nil {
		return fmt.Errorf("upsert inbox message: %w", err)
	}
	return nil
}

// MaxUID returns the highest stored UID for the context, 0 when empty.
func (r *inboxRepo) MaxUID(contextID string) (int64, error) {
	var uid sql.NullInt64
	err := r.pool.QueryRow(context.Background(),
		`SELECT max(uid) FROM email_inbox WHERE context_id = $1`, contextID,
	).Scan(&uid)
	if err != nil {
		return 0, fmt.Errorf("max inbox uid: %w", err)
	}
	return uid.Int64, nil
}

// TrimTo keeps only the newest max messages for the context.
func (r *inboxRepo) TrimTo(contextID string, max int) error {
	if max <= 0 {
		return nil
	}
	_, err := r.pool.Exec(context.Background(),
		`DELETE FROM email_inbox WHERE context_id = $1 AND id NOT IN (
			SELECT id FROM email_inbox WHERE context_id = $1 ORDER BY message_date DESC, uid DESC LIMIT $2
		 )`,
		contextID, max,
	)
	if err != nil {
		return fmt.Errorf("trim inbox: %w", err)
	}
	return nil
}
