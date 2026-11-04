package csvimport

import (
	"strings"
	"testing"
)

func TestParseCSV_RejectsOversizedField(t *testing.T) {
	// A single field exceeding the 1 MB line limit must abort the import.
	var sb strings.Builder
	sb.WriteString("email,first_name\n")
	sb.WriteString("a@b.com,")
	sb.WriteString(strings.Repeat("x", (1<<20)+1))

	_, _, err := ParseCSV(strings.NewReader(sb.String()),
		FieldMapping{"email": 0, "first_name": 1}, "t1")
	if err == nil {
		t.Fatal("expected error for field over 1 MB, got nil")
	}
}

func TestParseCSV_RejectsOversizedRowCount(t *testing.T) {
	// More than 100,000 data rows must abort the import.
	var sb strings.Builder
	sb.WriteString("email,first_name\n")
	for i := 0; i < 100001; i++ {
		sb.WriteString("row@example.com,Row\n")
	}

	_, _, err := ParseCSV(strings.NewReader(sb.String()),
		FieldMapping{"email": 0, "first_name": 1}, "t1")
	if err == nil {
		t.Fatal("expected error for more than 100,000 rows, got nil")
	}
}