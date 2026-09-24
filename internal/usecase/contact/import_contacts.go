package contact

import (
	"fmt"
	"io"
	"strings"

	csvimport "github.com/hekemen/automata/pkg/import"
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

	// Parse CSV
	contacts, result, err := csvimport.ParseCSV(input.CSVData, input.Mapping, contextID)
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
