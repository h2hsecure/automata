package mcp

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/hekemen/automata/internal/domain/contact"
	contactuc "github.com/hekemen/automata/internal/usecase/contact"
	csvimport "github.com/hekemen/automata/pkg/import"
)

// registerContactTools adds contact-related MCP tools.
func (s *Server) registerContactTools() {
	s.AddTool("contacts.list", "List contacts for a context", func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		contextID, _ := args["context_id"].(string)
		if contextID == "" {
			return nil, fmt.Errorf("context_id is required")
		}

		var filters contact.FilterOptions
		if tags, ok := args["tags"].([]interface{}); ok {
			filters.Tags = interfaceSliceToStringSlice(tags)
		}
		if source, ok := args["source"].(string); ok && source != "" {
			filters.Source = source
		}
		if company, ok := args["company"].(string); ok && company != "" {
			filters.Company = company
		}
		if search, ok := args["search"].(string); ok && search != "" {
			filters.Search = search
		}

		page := 1
		if pageArg, ok := args["page"].(float64); ok && pageArg > 0 {
			page = int(pageArg)
		}
		limit := 20
		if limitArg, ok := args["limit"].(float64); ok && limitArg > 0 {
			limit = int(limitArg)
		}
		offset := (page - 1) * limit

		result, err := contactuc.ListContacts(s.contactRepo, contextID, contactuc.ListOptions{
			Offset: offset,
			Limit:  limit,
			Filters: filters,
		})
		if err != nil {
			return nil, fmt.Errorf("list contacts: %w", err)
		}

		contacts := make([]map[string]interface{}, 0, len(result.Contacts))
		for _, c := range result.Contacts {
			contacts = append(contacts, contactToMap(c))
		}

		return map[string]interface{}{
			"contacts": contacts,
			"total":    result.Total,
			"page":     page,
			"limit":    limit,
		}, nil
	})

	s.AddTool("contacts.get", "Get a contact by ID", func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		contextID, _ := args["context_id"].(string)
		id, ok := args["id"].(string)
		if !ok || id == "" {
			return nil, fmt.Errorf("id is required")
		}
		if contextID == "" {
			return nil, fmt.Errorf("context_id is required")
		}

		contact, err := contactuc.GetContact(s.contactRepo, id, contextID)
		if err != nil {
			return nil, fmt.Errorf("get contact: %w", err)
		}

		return contactToMap(contact), nil
	})

	s.AddTool("contacts.create", "Create a new contact", func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		contextID, _ := args["context_id"].(string)
		if contextID == "" {
			return nil, fmt.Errorf("context_id is required")
		}

		var email *string
		if emailVal, ok := args["email"].(string); ok && emailVal != "" {
			email = &emailVal
		}

		var customFields map[string]interface{}
		if cf, ok := args["custom_fields"].(map[string]interface{}); ok {
			customFields = cf
		}

		var tags []string
		if tagsArg, ok := args["tags"].([]interface{}); ok {
			tags = interfaceSliceToStringSlice(tagsArg)
		}

		input := contactuc.CreateInput{
			Email:        email,
			FirstName:    getStringArg(args, "first_name"),
			LastName:     getStringArg(args, "last_name"),
			Phone:        getStringArg(args, "phone"),
			Company:      getStringArg(args, "company"),
			CustomFields: customFields,
			Source:       getStringArg(args, "source"),
			SourceID:     getStringArg(args, "source_id"),
			Tags:         tags,
		}

		contact, err := contactuc.CreateContact(s.contactRepo, contextID, input)
		if err != nil {
			return nil, fmt.Errorf("create contact: %w", err)
		}

		return contactToMap(contact), nil
	})

	s.AddTool("contacts.update", "Update a contact", func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		contextID, _ := args["context_id"].(string)
		id, ok := args["id"].(string)
		if !ok || id == "" {
			return nil, fmt.Errorf("id is required")
		}
		if contextID == "" {
			return nil, fmt.Errorf("context_id is required")
		}

		var email *string
		if emailVal, ok := args["email"].(string); ok && emailVal != "" {
			email = &emailVal
		}

		var customFields map[string]interface{}
		if cf, ok := args["custom_fields"].(map[string]interface{}); ok {
			customFields = cf
		}

		var tags []string
		if tagsArg, ok := args["tags"].([]interface{}); ok {
			tags = interfaceSliceToStringSlice(tagsArg)
		}

		input := contactuc.UpdateInput{
			Email:        email,
			FirstName:    getStringArg(args, "first_name"),
			LastName:     getStringArg(args, "last_name"),
			Phone:        getStringArg(args, "phone"),
			Company:      getStringArg(args, "company"),
			CustomFields: customFields,
			Tags:         tags,
		}

		contact, err := contactuc.UpdateContact(s.contactRepo, id, contextID, input)
		if err != nil {
			return nil, fmt.Errorf("update contact: %w", err)
		}

		return contactToMap(contact), nil
	})

	s.AddTool("contacts.delete", "Delete a contact", func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		contextID, _ := args["context_id"].(string)
		id, ok := args["id"].(string)
		if !ok || id == "" {
			return nil, fmt.Errorf("id is required")
		}
		if contextID == "" {
			return nil, fmt.Errorf("context_id is required")
		}

		err := contactuc.DeleteContact(s.contactRepo, id, contextID)
		if err != nil {
			return nil, fmt.Errorf("delete contact: %w", err)
		}

		return map[string]interface{}{"success": true}, nil
	})

	s.AddTool("contacts.merge", "Merge two contacts", func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		contextID, _ := args["context_id"].(string)
		keepID, ok1 := args["keep_id"].(string)
		mergeIntoID, ok2 := args["merge_into_id"].(string)
		if !ok1 || keepID == "" || !ok2 || mergeIntoID == "" {
			return nil, fmt.Errorf("keep_id and merge_into_id are required")
		}
		if contextID == "" {
			return nil, fmt.Errorf("context_id is required")
		}

		err := contactuc.MergeContacts(s.contactRepo, contextID, keepID, mergeIntoID)
		if err != nil {
			return nil, fmt.Errorf("merge contacts: %w", err)
		}

		return map[string]interface{}{"success": true}, nil
	})

	s.AddTool("contacts.import", "Import contacts from CSV", func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		contextID, _ := args["context_id"].(string)
		if contextID == "" {
			return nil, fmt.Errorf("context_id is required")
		}

		csvData, ok := args["csv_data"].(string)
		if !ok || csvData == "" {
			return nil, fmt.Errorf("csv_data is required")
		}

		var mapping csvimport.FieldMapping
		if mappingArg, ok := args["mapping"].(map[string]interface{}); ok {
			mapping = interfaceMapToIntMap(mappingArg)
		} else {
			mapping = csvimport.FieldMapping{
				"email":      0,
				"first_name": 1,
				"last_name":  2,
				"phone":      3,
				"company":    4,
			}
		}

		reader := strings.NewReader(csvData)
		input := contactuc.ImportCSVInput{
			CSVData:  reader,
			Mapping:  mapping,
			Source:   getStringArg(args, "source"),
			SourceID: getStringArg(args, "source_id"),
		}

		result, err := contactuc.ImportContacts(s.contactRepo, contextID, input)
		if err != nil {
			return nil, fmt.Errorf("import contacts: %w", err)
		}

		errors := make([]map[string]interface{}, 0, len(result.Errors))
		for _, e := range result.Errors {
			errors = append(errors, map[string]interface{}{
				"row":    e.Row,
				"field":  e.Field,
				"reason": e.Reason,
			})
		}

		return map[string]interface{}{
			"created": result.Created,
			"updated": result.Updated,
			"skipped": result.Skipped,
			"errors":  errors,
		}, nil
	})

	s.AddTool("contacts.export", "Export contacts to CSV", func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		contextID, _ := args["context_id"].(string)
		if contextID == "" {
			return nil, fmt.Errorf("context_id is required")
		}

		var filters contact.FilterOptions
		if tags, ok := args["tags"].([]interface{}); ok {
			filters.Tags = interfaceSliceToStringSlice(tags)
		}

		var fields []string
		if fieldsArg, ok := args["fields"].([]interface{}); ok {
			fields = interfaceSliceToStringSlice(fieldsArg)
		} else {
			fields = []string{"id", "email", "first_name", "last_name", "phone", "company", "source", "created_at"}
		}

		csvBytes, err := contactuc.ExportContacts(s.contactRepo, contextID, filters, fields)
		if err != nil {
			return nil, fmt.Errorf("export contacts: %w", err)
		}

		return map[string]interface{}{
			"csv": base64.StdEncoding.EncodeToString(csvBytes),
		}, nil
	})

	s.AddTool("contacts.get_activity", "Get contact activity timeline", func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		contextID, _ := args["context_id"].(string)
		contactID, ok := args["contact_id"].(string)
		if !ok || contactID == "" {
			return nil, fmt.Errorf("contact_id is required")
		}
		if contextID == "" {
			return nil, fmt.Errorf("context_id is required")
		}

		offset := 0
		if offsetArg, ok := args["offset"].(float64); ok {
			offset = int(offsetArg)
		}
		limit := 50
		if limitArg, ok := args["limit"].(float64); ok && limitArg > 0 {
			limit = int(limitArg)
		}

		var activityType *string
		if at, ok := args["activity_type"].(string); ok && at != "" {
			activityType = &at
		}

		result, err := contactuc.GetActivity(s.contactRepo, contactID, contextID, offset, limit, activityType)
		if err != nil {
			return nil, fmt.Errorf("get activity: %w", err)
		}

		total := result.Total
		activityMaps := make([]map[string]interface{}, 0, len(result.Activities))
		for _, a := range result.Activities {
			activityMaps = append(activityMaps, map[string]interface{}{
				"id":          a.ID,
				"type":        string(a.Type),
				"title":       a.Title,
				"description": a.Description,
				"data":        a.Data,
				"created_at":  a.CreatedAt.Format("2006-01-02T15:04:05Z"),
			})
		}

		return map[string]interface{}{
			"activities": activityMaps,
			"total":      total,
		}, nil
	})
}

// contactToMap converts a domain Contact to a map for JSON serialization.
func contactToMap(c *contact.Contact) map[string]interface{} {
	email := ""
	if c.Email != nil {
		email = *c.Email
	}

	tagNames := make([]string, 0, len(c.Tags))
	for _, tag := range c.Tags {
		tagNames = append(tagNames, tag.Name)
	}

	return map[string]interface{}{
		"id":           c.ID,
		"context_id":    c.ContextID,
		"email":        email,
		"first_name":   c.FirstName,
		"last_name":    c.LastName,
		"phone":        c.Phone,
		"company":      c.Company,
		"custom_fields": c.CustomFields,
		"source":       c.Source,
		"source_id":    c.SourceID,
		"tags":         tagNames,
		"created_at":   c.CreatedAt.Format("2006-01-02T15:04:05Z"),
		"updated_at":   c.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
}

// getStringArg extracts a string value from args, returning empty string if not present.
func getStringArg(args map[string]interface{}, key string) string {
	if val, ok := args[key].(string); ok {
		return val
	}
	return ""
}

// interfaceSliceToStringSlice converts []interface{} to []string.
func interfaceSliceToStringSlice(items []interface{}) []string {
	result := make([]string, 0, len(items))
	for _, item := range items {
		if s, ok := item.(string); ok {
			result = append(result, s)
		}
	}
	return result
}

// interfaceMapToIntMap converts map[string]interface{} to csvimport.FieldMapping (map[string]int).
func interfaceMapToIntMap(m map[string]interface{}) csvimport.FieldMapping {
	result := make(csvimport.FieldMapping)
	for k, v := range m {
		if idx, ok := v.(float64); ok {
			result[k] = int(idx)
		}
	}
	return result
}
