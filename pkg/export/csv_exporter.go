package export

import (
	"bytes"
	"encoding/csv"
	"fmt"
)

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
	defer writer.Flush()

	// Write header row
	if err := writer.Write(fields); err != nil {
		return nil, fmt.Errorf("write CSV header: %w", err)
	}

	// Write data rows
	for _, contact := range contacts {
		row := make([]string, len(fields))
		for i, field := range fields {
			if val, ok := contact[field]; ok {
				row[i] = fmt.Sprintf("%v", val)
			} else {
				row[i] = ""
			}
		}
		if err := writer.Write(row); err != nil {
			return nil, fmt.Errorf("write CSV row: %w", err)
		}
	}

	return buf.Bytes(), nil
}
