package export

import (
	"encoding/csv"
	"strings"
	"testing"
)

// TestCSVInjection_Neutralized ensures cell values that would be interpreted
// as formulas by spreadsheet applications (CSV injection) are prefixed with a
// single quote, while plain values pass through untouched.
func TestCSVInjection_Neutralized(t *testing.T) {
	dangerous := []string{"=SUM(A1)", "+cmd", "-1", "@x", "\tEVIL"}
	safe := []string{"hello", "123", "normal value"}

	contacts := []map[string]interface{}{
		{"name": dangerous[0], "email": safe[0]},
		{"name": dangerous[1], "email": safe[1]},
		{"name": dangerous[2], "email": safe[2]},
		{"name": dangerous[3], "email": "a@b.com"},
		{"name": dangerous[4], "email": "plain"},
	}

	out, err := Generate(contacts, []string{"name", "email"})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	records, err := csv.NewReader(strings.NewReader(string(out))).ReadAll()
	if err != nil {
		t.Fatalf("parse CSV: %v", err)
	}

	// Header at index 0, data rows follow.
	for i, d := range dangerous {
		got := records[i+1][0]
		if !strings.HasPrefix(got, "'") {
			t.Errorf("dangerous cell %q not neutralized, got %q", d, got)
		}
	}

	// Plain values must be untouched.
	if got := records[1][1]; got != "hello" {
		t.Errorf("plain value modified: got %q", got)
	}
	if got := records[2][1]; got != "123" {
		t.Errorf("plain value modified: got %q", got)
	}
	if got := records[3][1]; got != "normal value" {
		t.Errorf("plain value modified: got %q", got)
	}
}

func TestCSVInjection_EmptyCellsUntouched(t *testing.T) {
	contacts := []map[string]interface{}{
		{"name": "", "email": ""},
	}
	out, err := Generate(contacts, []string{"name", "email"})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	records, err := csv.NewReader(strings.NewReader(string(out))).ReadAll()
	if err != nil {
		t.Fatalf("parse CSV: %v", err)
	}
	if records[1][0] != "" || records[1][1] != "" {
		t.Errorf("empty cells should stay empty, got %q / %q", records[1][0], records[1][1])
	}
}