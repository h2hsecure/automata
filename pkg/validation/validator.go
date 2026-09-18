package validation

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/hekemen/automata/internal/domain/form"
)

const emailRegex = `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`

var emailPattern = regexp.MustCompile(emailRegex)

// Validate validates form data against the defined form fields.
// It checks required fields, email format, number range, regex patterns,
// and string length constraints, returning all validation errors combined.
func Validate(data map[string]interface{}, fields []form.FormField) error {
	var errors []string

	for _, field := range fields {
		value, exists := data[field.Slug]

		// Check required
		if field.Required && (!exists || isEmpty(value)) {
			errors = append(errors, fmt.Sprintf("%s is required", field.Label))
			continue
		}

		// Skip further validation if value is empty or not present
		if !exists || isEmpty(value) {
			continue
		}

		strValue, ok := value.(string)
		if !ok {
			strValue = fmt.Sprintf("%v", value)
		}

		// Email validation
		if field.Type == "email" && !emailPattern.MatchString(strValue) {
			errors = append(errors, fmt.Sprintf("%s must be a valid email", field.Label))
			continue
		}

		// Number range validation
		if field.Type == "number" && field.Validation != nil {
			numValue, err := toFloat64(value)
			if err == nil {
				if min, ok := field.Validation["min"].(float64); ok && numValue < min {
					errors = append(errors, fmt.Sprintf("%s must be at least %v", field.Label, min))
				}
				if max, ok := field.Validation["max"].(float64); ok && numValue > max {
					errors = append(errors, fmt.Sprintf("%s must be at most %v", field.Label, max))
				}
			}
		}

		// Regex pattern validation
		if field.Validation != nil {
			if pattern, ok := field.Validation["pattern"].(string); ok && pattern != "" {
				re, err := regexp.Compile(pattern)
				if err == nil && !re.MatchString(strValue) {
					errors = append(errors, fmt.Sprintf("%s format is invalid", field.Label))
				}
			}
		}

		// String length validation
		if field.Validation != nil {
			if minLength, ok := field.Validation["min_length"].(float64); ok {
				if float64(len(strValue)) < minLength {
					errors = append(errors, fmt.Sprintf("%s must be at least %v characters", field.Label, int(minLength)))
				}
			}
			if maxLength, ok := field.Validation["max_length"].(float64); ok {
				if float64(len(strValue)) > maxLength {
					errors = append(errors, fmt.Sprintf("%s must be at most %v characters", field.Label, int(maxLength)))
				}
			}
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("validation failed: %s", strings.Join(errors, "; "))
	}

	return nil
}

func isEmpty(value interface{}) bool {
	switch v := value.(type) {
	case string:
		return v == ""
	case nil:
		return true
	case float64:
		return v == 0
	case bool:
		return false
	default:
		return false
	}
}

func toFloat64(value interface{}) (float64, error) {
	switch v := value.(type) {
	case float64:
		return v, nil
	case int:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case string:
		return 0, fmt.Errorf("cannot convert string to number")
	default:
		return 0, fmt.Errorf("unsupported type for number validation")
	}
}
