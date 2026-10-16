package form

import (
	"context"

	ct "github.com/hekemen/automata/internal/domain/contact"
	tracking "github.com/hekemen/automata/internal/domain/tracking"
	contact "github.com/hekemen/automata/internal/usecase/contact"
)

// IdentifyVisitor creates a contact from form data (email dedupe built in)
// and links it to the tracking visitor. It only identifies when an email
// is present in the submission.
func IdentifyVisitor(contactRepo contact.Repository, trackingRepo tracking.Repository,
	contextID, slug, visitorID string, data map[string]interface{}) (*ct.Contact, error) {

	emailVal, ok := data["email"].(string)
	if !ok || emailVal == "" {
		return nil, nil
	}

	email := emailVal
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
