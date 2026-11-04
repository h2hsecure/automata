package contact

import (
	"errors"
	"strings"
	"testing"

	ct "github.com/h2hsecure/automata/internal/domain/contact"
)

// fakeRepo implements Repository for usecase tests.
type fakeRepo struct {
	existing        *ct.Contact
	contacts        map[string]*ct.Contact
	updated         *ct.Contact
	activities      []*ct.Activity
	createErr       error
	updateErr       error
	createActivityErr error
	findByEmailFunc func(contextID, email string) (*ct.Contact, error)
	listFunc        func(offset, limit int, filters ct.FilterOptions) ([]*ct.Contact, error)
	tagRepo         TagRepository
}

var _ Repository = (*fakeRepo)(nil)

func (f *fakeRepo) Create(c *ct.Contact) error { return f.createErr }

func (f *fakeRepo) GetByID(id string) (*ct.Contact, error) {
	if f.contacts != nil {
		if c, ok := f.contacts[id]; ok {
			return c, nil
		}
		return nil, errors.New("contact not found")
	}
	if f.existing == nil || f.existing.ID != id {
		return nil, errors.New("contact not found")
	}
	return f.existing, nil
}

func (f *fakeRepo) List(offset, limit int, filters ct.FilterOptions) ([]*ct.Contact, error) {
	if f.listFunc != nil {
		return f.listFunc(offset, limit, filters)
	}
	return nil, nil
}

func (f *fakeRepo) Update(c *ct.Contact) error {
	f.updated = c
	return f.updateErr
}

func (f *fakeRepo) Delete(id string) error { return nil }

func (f *fakeRepo) FindByEmail(contextID, email string) (*ct.Contact, error) {
	if f.findByEmailFunc != nil {
		return f.findByEmailFunc(contextID, email)
	}
	return nil, errors.New("contact not found")
}

func (f *fakeRepo) Merge(keepID, mergeIntoID string) error { return nil }

func (f *fakeRepo) CreateActivity(a *ct.Activity) error {
	if f.createActivityErr != nil {
		return f.createActivityErr
	}
	f.activities = append(f.activities, a)
	return nil
}

// TagRepository exposes the fake tag repository, letting applyTags's
// tagOpener type-assertion succeed in tests.
func (f *fakeRepo) TagRepository() TagRepository { return f.tagRepo }

// fakeTagRepo records the tag names each ApplyTags call replaces with.
// applied stays nil when ApplyTags was never called.
type fakeTagRepo struct {
	applied []string
}

var _ TagRepository = (*fakeTagRepo)(nil)

func (f *fakeTagRepo) ApplyTags(contextID, contactID string, tagNames []string, ac ActivityCloser) error {
	f.applied = tagNames
	return nil
}
func (f *fakeTagRepo) ListTags(contextID string) ([]*ct.Tag, error)              { return nil, nil }
func (f *fakeTagRepo) CreateTag(contextID, name, color string) (*ct.Tag, error)   { return nil, nil }
func (f *fakeTagRepo) DeleteTag(contextID, tagID string) error                    { return nil }

func (f *fakeRepo) GetActivity(contactID, contextID string, offset, limit int, activityType *string) ([]ct.Activity, int64, error) {
	return nil, 0, nil
}

func (f *fakeRepo) GetActivityCount(contactID string) (int64, error) { return 0, nil }

func (f *fakeRepo) CountByContext(contextID string) (int64, error) { return 0, nil }

func (f *fakeRepo) FindBySourceID(contextID, source, sourceID string) (*ct.Contact, error) {
	return nil, nil
}

func (f *fakeRepo) Count(contextID string, filters ct.FilterOptions) (int64, error) { return 0, nil }

func newTestContact(id string) *ct.Contact {
	email := "old@example.com"
	return &ct.Contact{
		ID:           id,
		ContextID:    "ctx-1",
		Email:        &email,
		FirstName:    "Old",
		LastName:     "OldLast",
		Phone:        "+15550001",
		Company:      "OldCo",
		CustomFields: map[string]interface{}{"plan": "pro"},
	}
}

func TestUpdateContact_PartialMergePreservesUnprovidedFields(t *testing.T) {
	repo := &fakeRepo{existing: newTestContact("c-1")}

	got, err := UpdateContact(repo, "c-1", "ctx-1", UpdateInput{FirstName: "New"})
	if err != nil {
		t.Fatalf("UpdateContact() error: %v", err)
	}

	if got.FirstName != "New" {
		t.Errorf("FirstName = %q, want %q", got.FirstName, "New")
	}
	if got.Email == nil || *got.Email != "old@example.com" {
		t.Errorf("Email = %v, want old@example.com", got.Email)
	}
	if got.LastName != "OldLast" {
		t.Errorf("LastName = %q, want OldLast", got.LastName)
	}
	if got.Phone != "+15550001" {
		t.Errorf("Phone = %q, want +15550001", got.Phone)
	}
	if got.Company != "OldCo" {
		t.Errorf("Company = %q, want OldCo", got.Company)
	}
	if got.CustomFields["plan"] != "pro" {
		t.Errorf("CustomFields = %v, want plan=pro", got.CustomFields)
	}
	if repo.updated == nil {
		t.Fatal("repo.Update was not called")
	}
}

func TestUpdateContact_ActivityRecordsPreviousValues(t *testing.T) {
	repo := &fakeRepo{existing: newTestContact("c-1")}
	newEmail := "new@example.com"

	_, err := UpdateContact(repo, "c-1", "ctx-1", UpdateInput{
		Email:     &newEmail,
		FirstName: "New",
	})
	if err != nil {
		t.Fatalf("UpdateContact() error: %v", err)
	}

	if len(repo.activities) != 1 {
		t.Fatalf("expected 1 activity, got %d", len(repo.activities))
	}
	activity := repo.activities[0]
	if activity.Type != ct.ActivityContactUpdated {
		t.Errorf("activity.Type = %q, want %q", activity.Type, ct.ActivityContactUpdated)
	}

	prev, ok := activity.Data["previous_values"].(map[string]interface{})
	if !ok {
		t.Fatalf("previous_values missing or wrong type: %v", activity.Data["previous_values"])
	}
	if prev["email"] != "old@example.com" {
		t.Errorf("previous_values.email = %v, want old@example.com", prev["email"])
	}
	if prev["first_name"] != "Old" {
		t.Errorf("previous_values.first_name = %v, want Old", prev["first_name"])
	}
	if prev["phone"] != "+15550001" {
		t.Errorf("previous_values.phone = %v, want +15550001", prev["phone"])
	}

	changed, ok := activity.Data["changed_fields"].([]string)
	if !ok {
		t.Fatalf("changed_fields missing or wrong type: %v", activity.Data["changed_fields"])
	}
	wantChanged := map[string]bool{"email": false, "first_name": false}
	for _, f := range changed {
		if _, ok := wantChanged[f]; ok {
			wantChanged[f] = true
		}
	}
	for f, seen := range wantChanged {
		if !seen {
			t.Errorf("changed_fields = %v, want it to contain %q", changed, f)
		}
	}
}

func TestUpdateContact_NoopUpdateCreatesNoActivity(t *testing.T) {
	repo := &fakeRepo{existing: newTestContact("c-1")}

	if _, err := UpdateContact(repo, "c-1", "ctx-1", UpdateInput{}); err != nil {
		t.Fatalf("UpdateContact() error: %v", err)
	}
	if len(repo.activities) != 0 {
		t.Errorf("expected no activities, got %d", len(repo.activities))
	}
}

func TestUpdateContact_ContactNotFound(t *testing.T) {
	repo := &fakeRepo{}

	_, err := UpdateContact(repo, "missing", "ctx-1", UpdateInput{FirstName: "X"})
	if err == nil || !strings.Contains(err.Error(), "contact not found") {
		t.Errorf("expected contact not found error, got %v", err)
	}
}

func TestUpdateContact_WrongContext(t *testing.T) {
	repo := &fakeRepo{existing: newTestContact("c-1")}

	_, err := UpdateContact(repo, "c-1", "ctx-2", UpdateInput{FirstName: "X"})
	if err == nil || !strings.Contains(err.Error(), "does not belong to context") {
		t.Errorf("expected context error, got %v", err)
	}
}

func TestUpdateContact_EmailConflictWithOtherContact(t *testing.T) {
	other := newTestContact("c-2")
	repo := &fakeRepo{
		existing: newTestContact("c-1"),
		findByEmailFunc: func(contextID, email string) (*ct.Contact, error) {
			return other, nil
		},
	}
	conflictEmail := "conflict@example.com"

	_, err := UpdateContact(repo, "c-1", "ctx-1", UpdateInput{Email: &conflictEmail})
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Errorf("expected email conflict error, got %v", err)
	}
}

func TestUpdateContact_InvalidEmailRejected(t *testing.T) {
	repo := &fakeRepo{existing: newTestContact("c-1")}
	badEmail := "not-an-email"

	_, err := UpdateContact(repo, "c-1", "ctx-1", UpdateInput{Email: &badEmail})
	if err == nil || !strings.Contains(err.Error(), "invalid email") {
		t.Errorf("expected invalid email error, got %v", err)
	}
}

func TestUpdateContact_NilTagsLeaveExistingTagsUntouched(t *testing.T) {
	tags := &fakeTagRepo{}
	repo := &fakeRepo{existing: newTestContact("c-1"), tagRepo: tags}

	if _, err := UpdateContact(repo, "c-1", "ctx-1", UpdateInput{FirstName: "New"}); err != nil {
		t.Fatalf("UpdateContact() error: %v", err)
	}
	if tags.applied != nil {
		t.Errorf("ApplyTags was called with %v, want tags untouched (nil input)", tags.applied)
	}
}

func TestUpdateContact_EmptyTagsClearExistingTags(t *testing.T) {
	tags := &fakeTagRepo{}
	repo := &fakeRepo{existing: newTestContact("c-1"), tagRepo: tags}

	empty := []string{}
	if _, err := UpdateContact(repo, "c-1", "ctx-1", UpdateInput{Tags: &empty}); err != nil {
		t.Fatalf("UpdateContact() error: %v", err)
	}
	if tags.applied == nil {
		t.Fatal("ApplyTags was not called, want tag clearing for empty slice")
	}
	if len(tags.applied) != 0 {
		t.Errorf("ApplyTags called with %v, want empty slice", tags.applied)
	}
}

func TestUpdateContact_NonEmptyTagsReplace(t *testing.T) {
	tags := &fakeTagRepo{}
	repo := &fakeRepo{existing: newTestContact("c-1"), tagRepo: tags}

	newTags := []string{"vip"}
	if _, err := UpdateContact(repo, "c-1", "ctx-1", UpdateInput{Tags: &newTags}); err != nil {
		t.Fatalf("UpdateContact() error: %v", err)
	}
	if tags.applied == nil || len(tags.applied) != 1 || tags.applied[0] != "vip" {
		t.Errorf("ApplyTags called with %v, want [vip]", tags.applied)
	}
}

func TestUpdateContact_EmptyStringEmailIgnored(t *testing.T) {
	repo := &fakeRepo{existing: newTestContact("c-1")}
	empty := ""

	got, err := UpdateContact(repo, "c-1", "ctx-1", UpdateInput{Email: &empty, FirstName: "New"})
	if err != nil {
		t.Fatalf("UpdateContact() error: %v", err)
	}
	if got.Email == nil || *got.Email != "old@example.com" {
		t.Errorf("Email = %v, want old@example.com preserved (empty string must not clear it)", got.Email)
	}
}

func TestUpdateContact_UniqueViolationReturnsExisting(t *testing.T) {
	repo := &fakeRepo{
		existing:  newTestContact("c-1"),
		updateErr: errors.New("duplicate key value violates unique constraint (23505)"),
		findByEmailFunc: func(contextID, email string) (*ct.Contact, error) {
			if email == "old@example.com" {
				return newTestContact("c-9"), nil
			}
			return nil, errors.New("contact not found")
		},
	}

	got, err := UpdateContact(repo, "c-1", "ctx-1", UpdateInput{FirstName: "New"})
	if err != nil {
		t.Fatalf("UpdateContact() should re-read on 23505, got error: %v", err)
	}
	if got.ID != "c-9" {
		t.Errorf("returned contact ID = %q, want c-9 (re-read existing)", got.ID)
	}
}
