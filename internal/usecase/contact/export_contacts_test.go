package contact

import (
	"strings"
	"testing"

	ct "github.com/h2hsecure/automata/internal/domain/contact"
)

// fakePaginatingRepo simulates offset/limit pagination over a fixed set of
// contacts, so export pagination is exercised even in unit tests.
type fakePaginatingRepo struct {
	*fakeRepo
	all   []*ct.Contact
	lists int
}

var _ Repository = (*fakePaginatingRepo)(nil)

func (f *fakePaginatingRepo) List(offset, limit int, filters ct.FilterOptions) ([]*ct.Contact, error) {
	f.lists++
	if limit <= 0 {
		return nil, nil
	}
	if offset >= len(f.all) {
		return nil, nil
	}
	end := offset + limit
	if end > len(f.all) {
		end = len(f.all)
	}
	return f.all[offset:end], nil
}

// TestExportContacts_PaginatesToExhaustion verifies that exports do not
// silently cap at one page: 5 contacts with a page size of 2 must all
// appear in the output, requiring multiple List calls.
func TestExportContacts_PaginatesToExhaustion(t *testing.T) {
	var all []*ct.Contact
	for i := 0; i < 5; i++ {
		all = append(all, &ct.Contact{ID: idOf(i), ContextID: "ctx-1"})
	}

	repo := &fakePaginatingRepo{all: all}

	csv, err := exportContactsPages(repo, "ctx-1", ct.FilterOptions{ContextID: "ctx-1"}, []string{"id"}, 2)
	if err != nil {
		t.Fatalf("exportContactsPages() error: %v", err)
	}

	out := string(csv)
	for i := 0; i < 5; i++ {
		if !strings.Contains(out, idOf(i)) {
			t.Errorf("export missing contact %s; got:\n%s", idOf(i), out)
		}
	}
	if repo.lists < 3 {
		t.Errorf("expected at least 3 List calls for 5 rows at page size 2, got %d", repo.lists)
	}
}

func idOf(i int) string {
	return "c-" + string(rune('a'+i))
}