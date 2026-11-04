package contact

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	ct "github.com/h2hsecure/automata/internal/domain/contact"
)

func TestCreateContact_EmptyStringEmailBecomesNil(t *testing.T) {
	repo := &fakeRepo{}
	empty := ""

	got, err := CreateContact(repo, "ctx-1", CreateInput{Email: &empty, FirstName: "Ann"})
	if err != nil {
		t.Fatalf("CreateContact() error: %v", err)
	}
	if got.Email != nil {
		t.Errorf("Email = %v, want nil (empty string must normalize to nil)", got.Email)
	}
}

func TestCreateContact_UniqueViolationReturnsExisting(t *testing.T) {
	repo := &fakeRepo{
		createErr: errors.New("duplicate key value violates unique constraint (23505)"),
		findByEmailFunc: func(contextID, email string) (*ct.Contact, error) {
			if email == "dup@example.com" {
				return &ct.Contact{ID: "c-existing", ContextID: contextID}, nil
			}
			return nil, errors.New("contact not found")
		},
	}
	email := "dup@example.com"

	got, err := CreateContact(repo, "ctx-1", CreateInput{Email: &email, FirstName: "Ann"})
	if err != nil {
		t.Fatalf("CreateContact() should re-read on 23505, got error: %v", err)
	}
	if got.ID != "c-existing" {
		t.Errorf("returned contact ID = %q, want c-existing (re-read existing)", got.ID)
	}
}

func TestCreateContact_ActivityFailureIsLoggedNotFatal(t *testing.T) {
	var buf bytes.Buffer
	orig := log.Logger
	log.Logger = zerolog.New(&buf)
	defer func() { log.Logger = orig }()

	repo := &fakeRepo{
		createActivityErr: errors.New("activity insert blew up"),
	}
	email := "act@example.com"

	got, err := CreateContact(repo, "ctx-1", CreateInput{Email: &email, FirstName: "Ann"})
	if err != nil {
		t.Fatalf("CreateContact() should not fail when activity creation fails, got: %v", err)
	}
	if got.ID == "" {
		t.Fatal("expected a created contact")
	}
	out := buf.String()
	if !strings.Contains(out, "activity insert blew up") {
		t.Errorf("log output = %q, want it to contain the activity error", out)
	}
}