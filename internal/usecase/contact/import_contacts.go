package contact

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"io"
	"strings"

	csvimport "github.com/h2hsecure/automata/pkg/import"
)

// ImportCSVInput holds the input parameters for importing contacts from CSV.
type ImportCSVInput struct {
	CSVData  io.Reader
	Mapping  csvimport.FieldMapping
	Source   string
	SourceID string
}

// ImportContacts parses a CSV file and creates or skips contacts based on
// email uniqueness.  Returns an aggregate ImportResult.
func ImportContacts(repo Repository, contextID string, input ImportCSVInput) (*csvimport.ImportResult, error) {
	if contextID == "" {
		return nil, fmt.Errorf("context_id is required")
	}

	// When no explicit mapping is supplied, derive one from the CSV header
	// row so that every column is mapped to its own field name.
	mapping := input.Mapping
	var data io.Reader = input.CSVData
	if len(mapping) == 0 {
		headerLine, rest, err := readFirstLine(input.CSVData)
		if err != nil {
			return nil, fmt.Errorf("read CSV header: %w", err)
		}
		mapping, err = mappingFromHeader(headerLine)
		if err != nil {
			return nil, fmt.Errorf("derive field mapping: %w", err)
		}
		data = io.MultiReader(strings.NewReader(headerLine), rest)
	}

	// Parse CSV
	contacts, result, err := csvimport.ParseCSV(data, mapping, contextID)
	if err != nil {
		return nil, fmt.Errorf("parse CSV: %w", err)
	}

	// Process each parsed contact
	for _, contactMap := range contacts {
		// Extract email
		email := contactMap["email"]

		// Check for existing contact by email
		if email != "" {
			existing, err := repo.FindByEmail(contextID, email)
			if err != nil {
				if !strings.Contains(err.Error(), "contact not found") {
					result.Errors = append(result.Errors, csvimport.ImportError{
						Row:    result.Created + result.Updated + result.Skipped + 1,
						Field:  "email",
						Reason: fmt.Sprintf("check duplicate: %v", err),
					})
					result.Skipped++
					continue
				}
				// Contact not found, proceed with creation
			} else {
				// Duplicate found, skip
				result.Updated++
				_ = existing
				continue
			}
		}

		// Build contact input from CSV fields
		var emailPtr *string
		if email != "" {
			emailPtr = &email
		}

		createInput := CreateInput{
			Email:        emailPtr,
			FirstName:    contactMap["first_name"],
			LastName:     contactMap["last_name"],
			Phone:        contactMap["phone"],
			Company:      contactMap["company"],
			Source:       input.Source,
			SourceID:     input.SourceID,
			CustomFields: extractCustomFields(contactMap),
		}

		// Create the contact
		if _, err := CreateContact(repo, contextID, createInput); err != nil {
			result.Errors = append(result.Errors, csvimport.ImportError{
				Row:    result.Created + result.Updated + result.Skipped + 1,
				Field:  "email",
				Reason: fmt.Sprintf("create contact: %v", err),
			})
			result.Skipped++
			continue
		}

		result.Created++
	}

	return result, nil
}

// readFirstLine reads the first line of a CSV stream, including its
// trailing newline, and returns the line along with a reader positioned
// at the remainder of the stream. The remainder is a buffered reader so
// any data the buffer has already pulled is preserved.
func readFirstLine(r io.Reader) (string, io.Reader, error) {
	br := bufio.NewReader(r)
	line, err := br.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", nil, err
	}
	if line == "" {
		return "", nil, fmt.Errorf("empty CSV data")
	}
	// If the final line has no newline, ReadString returns io.EOF with the
	// content; normalize so the prepended data always ends with a newline.
	if !strings.HasSuffix(line, "\n") {
		line += "\n"
	}
	return line, br, nil
}

// mappingFromHeader parses a CSV header line and maps each non-empty,
// trimmed column name to its index.
func mappingFromHeader(headerLine string) (csvimport.FieldMapping, error) {
	records, err := csv.NewReader(strings.NewReader(headerLine)).ReadAll()
	if err != nil {
		return nil, fmt.Errorf("parse header: %w", err)
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("header has no columns")
	}
	mapping := make(csvimport.FieldMapping)
	for i, col := range records[0] {
		name := strings.TrimSpace(col)
		if name == "" {
			continue
		}
		mapping[name] = i
	}
	if len(mapping) == 0 {
		return nil, fmt.Errorf("header has no usable columns")
	}
	return mapping, nil
}

// extractCustomFields removes known standard fields from the contact map
// and returns the remaining entries as custom fields.
func extractCustomFields(contactMap map[string]string) map[string]interface{} {
	standardFields := map[string]bool{
		"email":      true,
		"first_name": true,
		"last_name":  true,
		"phone":      true,
		"company":    true,
	}

	customFields := make(map[string]interface{})
	for k, v := range contactMap {
		if !standardFields[k] {
			customFields[k] = v
		}
	}
	return customFields
}
