package form

import (
	"context"
	"strings"

	ct "github.com/h2hsecure/automata/internal/domain/contact"
	tracking "github.com/h2hsecure/automata/internal/domain/tracking"
	contact "github.com/h2hsecure/automata/internal/usecase/contact"
	"github.com/jackc/pgx/v5"
)

// txProvider is an optional narrow interface implemented by concrete
// repositories that wrap a pool (contact and tracking both do). It lets
// IdentifyVisitor run multi-repository steps atomically when the repos
// support it.
type txProvider interface {
	WithTx(ctx context.Context, fn func(tx pgx.Tx) error) error
}

type contactMergeTxRepo interface {
	txProvider
	MergeTx(tx pgx.Tx, keepID, mergeIntoID string) error
}

type trackingReassignTxRepo interface {
	ReassignVisitorContactIDTx(tx pgx.Tx, contextID, oldContactID, newContactID string) error
}

// IdentifyVisitor creates a contact from form data (email dedupe built in)
// and links it to the tracking visitor. It only identifies when an email
// is present in the submission.
//
// When the visitor already has a hidden contact (created by page tracking),
// that contact is "uplifted": identity fields are filled, hidden is cleared
// and the source is set to the form. On an email collision the hidden
// contact is merged into the existing contact and the visitor link is
// repointed to it.
func IdentifyVisitor(contactRepo contact.Repository, trackingRepo tracking.Repository,
	contextID, slug, visitorID string, data map[string]interface{}) (*ct.Contact, error) {

	emailVal, ok := data["email"].(string)
	if !ok || emailVal == "" {
		return nil, nil
	}
	email := emailVal

	hidden, err := contactRepo.FindBySourceID(contextID, "visitor", visitorID)
	if err != nil {
		return nil, err
	}

	if hidden != nil {
		// Collision check first: the unique (context_id, email) constraint
		// makes this ordering mandatory (the hidden contact's email is only
		// updated after we know it does not collide).
		existing, err := contactRepo.FindByEmail(contextID, email)
		if err != nil && !strings.Contains(err.Error(), "contact not found") {
			return nil, err
		}
		if existing != nil && existing.ID != hidden.ID {
			// Keep the real contact (its email stays); the hidden contact is
			// deleted by the merge, its page_visit activities move with it.
			//
			// When both repos expose a tx seam (the concrete PostgreSQL
			// repos do), run merge + reassign in one transaction so a
			// failure mid-merge cannot leave the contact merged while the
			// visitor link still points at the deleted contact.
			if m, ok := contactRepo.(contactMergeTxRepo); ok {
				if r, ok := trackingRepo.(trackingReassignTxRepo); ok {
					ctx := context.Background()
					if err := m.WithTx(ctx, func(tx pgx.Tx) error {
						if err := m.MergeTx(tx, existing.ID, hidden.ID); err != nil {
							return err
						}
						return r.ReassignVisitorContactIDTx(tx, contextID, hidden.ID, existing.ID)
					}); err != nil {
						return nil, err
					}
					return existing, nil
				}
			}

			if err := contactRepo.Merge(existing.ID, hidden.ID); err != nil {
				return nil, err
			}
			if err := trackingRepo.ReassignVisitorContactID(contextID, hidden.ID, existing.ID); err != nil {
				return nil, err
			}
			return existing, nil
		}

		hidden.Email = &email
		hidden.FirstName = str(data["first_name"])
		hidden.LastName = str(data["last_name"])
		hidden.Phone = str(data["phone"])
		hidden.Company = str(data["company"])
		hidden.Hidden = false
		hidden.Source = "form:" + slug
		if err := contactRepo.Update(hidden); err != nil {
			return nil, err
		}
		return hidden, nil
	}

	// Legacy path: no hidden contact for this visitor.
	c, err := contact.CreateContact(contactRepo, contextID, contact.CreateInput{
		Email:     &email,
		FirstName: str(data["first_name"]),
		LastName:  str(data["last_name"]),
		Phone:     str(data["phone"]),
		Company:   str(data["company"]),
		Source:    "form:" + slug,
		SourceID:  visitorID,
	})
	if err != nil {
		return nil, err
	}

	if err := trackingRepo.SetVisitorContactID(context.Background(), contextID, visitorID, c.ID); err != nil {
		return nil, err
	}

	return c, nil
}

func str(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}