package snippet

import (
	"strings"
	"testing"
)

func TestGenerateForm(t *testing.T) {
	js, err := GenerateForm("ctx-123", "contact")
	if err != nil {
		t.Fatalf("GenerateForm() error = %v", err)
	}
	for _, want := range []string{
		`"ctx-123"`,
		`/form/definition/ctx-123/contact`,
		`/form/submit/contact`,
		"X-Context-ID",
		"data-automata-form",
	} {
		if !strings.Contains(js, want) {
			t.Errorf("GenerateForm() missing %q", want)
		}
	}
}

func TestGenerateFormSendsVisitorID(t *testing.T) {
	js, err := GenerateForm("00000000-0000-4000-8000-000000000000", "test")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`_getCookie("automata_visitor")`, `visitor_id:visitorId`} {
		if !strings.Contains(js, want) {
			t.Errorf("snippet missing %q", want)
		}
	}
}

func TestGenerateFormValidation(t *testing.T) {
	if _, err := GenerateForm("", "contact"); err == nil {
		t.Error("expected error for empty contextID")
	}
	if _, err := GenerateForm("ctx-123", ""); err == nil {
		t.Error("expected error for empty slug")
	}
}