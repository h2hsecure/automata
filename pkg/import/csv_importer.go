package csvimport

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strings"
)

// FieldMapping maps field names to CSV column indices.
type FieldMapping map[string]int

// ImportResult aggregates the outcome of a CSV import.
type ImportResult struct {
	Created int
	Updated int
	Skipped int
	Errors  []ImportError
}

// ImportError records a problem encountered on a specific row.
type ImportError struct {
	Row    int
	Field  string
	Reason string
}

// ParseCSV reads a CSV stream, validates the header against the mapping,
// and returns a list of contact maps (one per data row) along with an
// aggregate ImportResult.
//
// Each returned map uses the field names from the mapping as keys and the
// extracted CSV cell values as string values.  The caller (use case layer)
// is responsible for converting these into domain objects and performing
// any database operations.
func ParseCSV(r io.Reader, mapping FieldMapping, tenantID string) ([]map[string]string, *ImportResult, error) {
	reader := csv.NewReader(r)
	reader.FieldsPerRecord = -1 // allow variable number of fields

	// Read header row
	header, err := reader.Read()
	if err != nil {
		return nil, nil, fmt.Errorf("read CSV header: %w", err)
	}

	// Build header index for validation
	headerIndex := make(map[string]int)
	for i, col := range header {
		headerIndex[strings.TrimSpace(col)] = i
	}

	// Validate that every mapped field exists in the header
	for fieldName, colIdx := range mapping {
		if _, ok := headerIndex[strings.TrimSpace(header[colIdx])]; !ok {
			return nil, nil, fmt.Errorf("field %q mapped to column %d but header does not contain expected column index", fieldName, colIdx)
		}
	}

	var contacts []map[string]string
	result := &ImportResult{}

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			result.Errors = append(result.Errors, ImportError{
				Row:    result.Created + result.Updated + result.Skipped + 1,
				Field:  "",
				Reason: fmt.Sprintf("read row: %v", err),
			})
			continue
		}

		rowNum := result.Created + result.Updated + result.Skipped + 1
		contact := make(map[string]string)

		for fieldName, colIdx := range mapping {
			value := ""
			if colIdx < len(record) {
				value = strings.TrimSpace(record[colIdx])
			}
			contact[fieldName] = value
		}

		// Validate email if present
		if email, ok := contact["email"]; ok && email != "" {
			if !isValidEmail(email) {
				result.Errors = append(result.Errors, ImportError{
					Row:    rowNum,
					Field:  "email",
					Reason: fmt.Sprintf("invalid email format: %s", email),
				})
				result.Skipped++
				continue
			}
		}

		// Require at least email or a name field
		if contact["email"] == "" && contact["first_name"] == "" && contact["last_name"] == "" {
			result.Errors = append(result.Errors, ImportError{
				Row:    rowNum,
				Field:  "email",
				Reason: "at least email, first_name, or last_name is required",
			})
			result.Skipped++
			continue
		}

		contacts = append(contacts, contact)
	}

	return contacts, result, nil
}

// isValidEmail checks basic email format validity.
func isValidEmail(email string) bool {
	if len(email) == 0 || len(email) > 254 {
		return false
	}
	atIdx := -1
	for i, ch := range email {
		if ch == '@' {
			atIdx = i
		}
	}
	if atIdx <= 0 || atIdx >= len(email)-1 {
		return false
	}
	local := email[:atIdx]
	domain := email[atIdx+1:]
	if len(local) == 0 || len(domain) == 0 {
		return false
	}
	if len(local) > 64 {
		return false
	}
	for _, ch := range local {
		if (ch < 33 || ch > 126) && ch != '.' && ch != '_' && ch != '-' && ch != '+' {
			return false
		}
	}
	if len(domain) > 253 {
		return false
	}
	return true
}

// _errorsSatisfies is a no-op variable to silence unused import if errors is used elsewhere.
var _ = errors.New
