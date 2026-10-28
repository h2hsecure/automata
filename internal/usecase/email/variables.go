package email

import (
	"time"

	"github.com/h2hsecure/automata/internal/domain/contact"
	"github.com/h2hsecure/automata/internal/domain/form"
)

// BuildContactVariables builds template variables from a contact for
// standalone email templates.
func BuildContactVariables(c *contact.Contact) map[string]interface{} {
	v := map[string]interface{}{
		"first_name": "",
		"last_name":  "",
		"email":      "",
		"phone":      "",
		"company":    "",
		"custom":     map[string]interface{}{},
	}
	if c == nil {
		return v
	}
	v["first_name"] = c.FirstName
	v["last_name"] = c.LastName
	if c.Email != nil {
		v["email"] = *c.Email
	}
	v["phone"] = c.Phone
	v["company"] = c.Company
	if c.CustomFields != nil {
		v["custom"] = c.CustomFields
	}
	return v
}

// BuildFormVariables builds template variables for form-integrated email
// templates: submission data under "form", plus contact variables when a
// matching contact is provided.
func BuildFormVariables(f *form.Form, s *form.Submission, c *contact.Contact) map[string]interface{} {
	v := BuildContactVariables(c)

	fieldData := map[string]interface{}{}
	if s != nil && s.Data != nil {
		for k, val := range s.Data {
			fieldData[k] = val
		}
	}
	v["form"] = fieldData

	formName := ""
	submitDate := ""
	if f != nil {
		formName = f.Name
	}
	if s != nil && !s.CreatedAt.IsZero() {
		submitDate = s.CreatedAt.Format(time.RFC1123)
	}
	v["form_name"] = formName
	v["submit_date"] = submitDate

	return v
}