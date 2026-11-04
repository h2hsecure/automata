package contact

import (
	"errors"
	"strings"
	"testing"

	csvimport "github.com/h2hsecure/automata/pkg/import"

	ct "github.com/h2hsecure/automata/internal/domain/contact"
)

var errContactNotFound = errors.New("contact not found")

// TestImportContacts_DuplicatesCountedAsSkipped verifies that rows whose
// email already exists are counted as Skipped, not Updated (the import
// never updates existing contacts).
func TestImportContacts_DuplicatesCountedAsSkipped(t *testing.T) {
	repo := &fakeRepo{
		findByEmailFunc: func(contextID, email string) (*ct.Contact, error) {
			if email == "dup@example.com" {
				return &ct.Contact{ID: "c-existing", ContextID: contextID}, nil
			}
			return nil, errContactNotFound
		},
	}

	csvData := "email,first_name\ndup@example.com,Ann\nfresh@example.com,Bob\n"

	result, err := ImportContacts(repo, "ctx-1", ImportCSVInput{
		CSVData: strings.NewReader(csvData),
		Mapping: csvimport.FieldMapping{"email": 0, "first_name": 1},
	})
	if err != nil {
		t.Fatalf("ImportContacts() error: %v", err)
	}

	if result.Created != 1 {
		t.Errorf("Created = %d, want 1", result.Created)
	}
	if result.Updated != 0 {
		t.Errorf("Updated = %d, want 0 (duplicates must not be counted as updated)", result.Updated)
	}
	if result.Skipped != 1 {
		t.Errorf("Skipped = %d, want 1 (the duplicate row)", result.Skipped)
	}
}