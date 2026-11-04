package contact

import (
	"bytes"
	"errors"
	"testing"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	ct "github.com/h2hsecure/automata/internal/domain/contact"
)

var errActivityBoom = errors.New("activity insert blew up")

func TestMergeContacts_ActivityFailureIsLoggedNotFatal(t *testing.T) {
	var buf bytes.Buffer
	orig := log.Logger
	log.Logger = zerolog.New(&buf)
	defer func() { log.Logger = orig }()

	keepEmail := "keep@example.com"
	mergeEmail := "merge@example.com"
	repo := &fakeRepo{
		contacts: map[string]*ct.Contact{
			"c-keep":  {ID: "c-keep", ContextID: "ctx-1", Email: &keepEmail},
			"c-merge": {ID: "c-merge", ContextID: "ctx-1", Email: &mergeEmail},
		},
		createActivityErr: errActivityBoom,
	}

	if err := MergeContacts(repo, "ctx-1", "c-keep", "c-merge"); err != nil {
		t.Fatalf("MergeContacts() should not fail when activity creation fails, got: %v", err)
	}
	if buf.String() == "" {
		t.Error("expected the activity creation error to be logged")
	}
}