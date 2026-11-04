package export

import (
	"bytes"
	"encoding/csv"
	"fmt"
)

// sanitizeCSVCell neutralizes CSV formula injection: values that begin with a
// character spreadsheet applications treat as a formula (or a tab/CR control
// char) are prefixed with a single quote so they are stored as plain text.
// Empty and plain values are returned unchanged.
func sanitizeCSVCell(s string) string {
	if s == "" {
		return s
	}
	switch s[0] {
	case '=', '+', '-', '@', '\t', '\r':
		return "'" + s
	}
	return s
}

// Generate creates a CSV byte slice from a list of contacts.
//
// contacts is a slice of maps, each representing a contact with string keys
// and interface{} values.  fields specifies the column order and which keys
// to include.
func Generate(contacts []map[string]interface{}, fields []string) ([]byte, error) {
	if len(contacts) == 0 {
		return nil, fmt.Errorf("no contacts to export")
	}
	if len(fields) == 0 {
		return nil, fmt.Errorf("at least one field is required for export")
	}

	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)

	// Write header row
	if err := writer.Write(fields); err != nil {
		return nil, fmt.Errorf("write CSV header: %w", err)
	}

	// Write data rows
	for _, contact := range contacts {
		row := make([]string, len(fields))
		for i, field := range fields {
			if val, ok := contact[field]; ok {
				row[i] = sanitizeCSVCell(fmt.Sprintf("%v", val))
			} else {
				row[i] = ""
			}
		}
		if err := writer.Write(row); err != nil {
			return nil, fmt.Errorf("write CSV row: %w", err)
		}
	}

	// Flush before reading the buffer: the writer buffers output, so the
	// data is not in buf until Flush has run.
	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, fmt.Errorf("write CSV: %w", err)
	}

	return buf.Bytes(), nil
}
