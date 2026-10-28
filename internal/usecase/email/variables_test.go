package email

import (
	"testing"
	"time"

	"github.com/h2hsecure/automata/internal/domain/contact"
	"github.com/h2hsecure/automata/internal/domain/form"
)

func TestBuildContactVariables(t *testing.T) {
	email := "a@b.com"
	c := &contact.Contact{
		FirstName:    "Ada",
		LastName:     "Lovelace",
		Email:        &email,
		Phone:        "+123",
		Company:      "Analytical Engines",
		CustomFields: map[string]interface{}{"plan": "pro"},
	}

	v := BuildContactVariables(c)

	if v["first_name"] != "Ada" {
		t.Errorf("first_name = %v, want Ada", v["first_name"])
	}
	if v["last_name"] != "Lovelace" {
		t.Errorf("last_name = %v, want Lovelace", v["last_name"])
	}
	if v["email"] != "a@b.com" {
		t.Errorf("email = %v, want a@b.com", v["email"])
	}
	if v["phone"] != "+123" {
		t.Errorf("phone = %v, want +123", v["phone"])
	}
	if v["company"] != "Analytical Engines" {
		t.Errorf("company = %v, want Analytical Engines", v["company"])
	}
	custom, ok := v["custom"].(map[string]interface{})
	if !ok {
		t.Fatalf("custom is not a map: %T", v["custom"])
	}
	if custom["plan"] != "pro" {
		t.Errorf("custom[plan] = %v, want pro", custom["plan"])
	}
}

func TestBuildContactVariablesNil(t *testing.T) {
	v := BuildContactVariables(nil)

	if v["first_name"] != "" {
		t.Errorf("first_name = %v, want empty", v["first_name"])
	}
	if v["email"] != "" {
		t.Errorf("email = %v, want empty", v["email"])
	}
	custom, ok := v["custom"].(map[string]interface{})
	if !ok {
		t.Fatalf("custom is not a map: %T", v["custom"])
	}
	if len(custom) != 0 {
		t.Errorf("custom should be empty, got %v", custom)
	}
}

func TestBuildFormVariables(t *testing.T) {
	submittedAt := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
	f := &form.Form{
		ID:   "f1",
		Name: "Contact Us",
	}
	s := &form.Submission{
		ID:        "s1",
		FormID:    "f1",
		Data:      map[string]interface{}{"name": "Ada", "message": "hi"},
		CreatedAt: submittedAt,
	}
	email := "a@b.com"
	c := &contact.Contact{FirstName: "Ada", Email: &email}

	v := BuildFormVariables(f, s, c)

	fd, ok := v["form"].(map[string]interface{})
	if !ok {
		t.Fatalf("form is not a map: %T", v["form"])
	}
	if fd["name"] != "Ada" {
		t.Errorf("form[name] = %v, want Ada", fd["name"])
	}
	if fd["message"] != "hi" {
		t.Errorf("form[message] = %v, want hi", fd["message"])
	}
	if v["form_name"] != "Contact Us" {
		t.Errorf("form_name = %v, want Contact Us", v["form_name"])
	}
	if v["submit_date"] != submittedAt.Format(time.RFC1123) {
		t.Errorf("submit_date = %v, want %v", v["submit_date"], submittedAt.Format(time.RFC1123))
	}
	// Contact variables are merged in.
	if v["first_name"] != "Ada" {
		t.Errorf("first_name = %v, want Ada", v["first_name"])
	}
}

func TestBuildFormVariablesNilContact(t *testing.T) {
	f := &form.Form{Name: "F"}
	s := &form.Submission{Data: map[string]interface{}{}}

	v := BuildFormVariables(f, s, nil)

	if v["first_name"] != "" {
		t.Errorf("first_name = %v, want empty", v["first_name"])
	}
	if v["form_name"] != "F" {
		t.Errorf("form_name = %v, want F", v["form_name"])
	}
}