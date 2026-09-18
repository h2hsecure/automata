package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hekemen/automata/internal/domain/form"
	formusecase "github.com/hekemen/automata/internal/usecase/form"
)

// registerFormTools adds form-related MCP tools.
func (s *Server) registerFormTools(formRepo form.Repository) {
	s.AddTool("forms.list", "List all forms for a tenant", func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		tenantID, _ := args["tenant_id"].(string)
		if tenantID == "" {
			return nil, fmt.Errorf("tenant_id is required")
		}

		forms, err := formusecase.ListForms(formRepo, tenantID)
		if err != nil {
			return nil, err
		}

		result := make([]map[string]interface{}, 0, len(forms))
		for _, f := range forms {
			result = append(result, map[string]interface{}{
				"id":          f.ID,
				"tenant_id":   f.TenantID,
				"slug":        f.Slug,
				"name":        f.Name,
				"description": f.Description,
				"fields":      f.Fields,
				"settings":    f.Settings,
				"created_at":  f.CreatedAt,
				"updated_at":  f.UpdatedAt,
			})
		}
		return result, nil
	})

	s.AddTool("forms.get", "Get a form by slug", func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		tenantID, _ := args["tenant_id"].(string)
		slug, _ := args["slug"].(string)
		if tenantID == "" {
			return nil, fmt.Errorf("tenant_id is required")
		}
		if slug == "" {
			return nil, fmt.Errorf("slug is required")
		}

		f, err := formusecase.GetForm(formRepo, tenantID, slug)
		if err != nil {
			return nil, err
		}

		return map[string]interface{}{
			"id":          f.ID,
			"tenant_id":   f.TenantID,
			"slug":        f.Slug,
			"name":        f.Name,
			"description": f.Description,
			"fields":      f.Fields,
			"settings":    f.Settings,
			"created_at":  f.CreatedAt,
			"updated_at":  f.UpdatedAt,
		}, nil
	})

	s.AddTool("forms.create", "Create a new form", func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		tenantID, _ := args["tenant_id"].(string)
		if tenantID == "" {
			return nil, fmt.Errorf("tenant_id is required")
		}

		slug, _ := args["slug"].(string)
		name, _ := args["name"].(string)
		description, _ := args["description"].(string)

		var fields []form.FormField
		if fieldsRaw, ok := args["fields"]; ok && fieldsRaw != nil {
			fieldsBytes, _ := json.Marshal(fieldsRaw)
			_ = json.Unmarshal(fieldsBytes, &fields)
		}

		var settings map[string]interface{}
		if settingsRaw, ok := args["settings"]; ok && settingsRaw != nil {
			settingsBytes, _ := json.Marshal(settingsRaw)
			_ = json.Unmarshal(settingsBytes, &settings)
		}

		input := formusecase.CreateFormInput{
			Slug:        slug,
			Name:        name,
			Description: description,
			Fields:      fields,
			Settings:    settings,
		}

		f, err := formusecase.CreateForm(formRepo, tenantID, input)
		if err != nil {
			return nil, err
		}

		return map[string]interface{}{
			"id":          f.ID,
			"tenant_id":   f.TenantID,
			"slug":        f.Slug,
			"name":        f.Name,
			"description": f.Description,
			"fields":      f.Fields,
			"settings":    f.Settings,
			"created_at":  f.CreatedAt,
			"updated_at":  f.UpdatedAt,
		}, nil
	})

	s.AddTool("forms.update", "Update a form", func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		tenantID, _ := args["tenant_id"].(string)
		if tenantID == "" {
			return nil, fmt.Errorf("tenant_id is required")
		}

		id, _ := args["id"].(string)
		if id == "" {
			return nil, fmt.Errorf("id is required")
		}

		name, _ := args["name"].(string)
		description, _ := args["description"].(string)
		slug, _ := args["slug"].(string)

		var fields []form.FormField
		if fieldsRaw, ok := args["fields"]; ok && fieldsRaw != nil {
			fieldsBytes, _ := json.Marshal(fieldsRaw)
			_ = json.Unmarshal(fieldsBytes, &fields)
		}

		var settings map[string]interface{}
		if settingsRaw, ok := args["settings"]; ok && settingsRaw != nil {
			settingsBytes, _ := json.Marshal(settingsRaw)
			_ = json.Unmarshal(settingsBytes, &settings)
		}

		input := formusecase.CreateFormInput{
			Slug:        slug,
			Name:        name,
			Description: description,
			Fields:      fields,
			Settings:    settings,
		}

		f, err := formusecase.UpdateForm(formRepo, tenantID, id, input)
		if err != nil {
			return nil, err
		}

		return map[string]interface{}{
			"id":          f.ID,
			"tenant_id":   f.TenantID,
			"slug":        f.Slug,
			"name":        f.Name,
			"description": f.Description,
			"fields":      f.Fields,
			"settings":    f.Settings,
			"created_at":  f.CreatedAt,
			"updated_at":  f.UpdatedAt,
		}, nil
	})

	s.AddTool("forms.delete", "Delete a form", func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		tenantID, _ := args["tenant_id"].(string)
		if tenantID == "" {
			return nil, fmt.Errorf("tenant_id is required")
		}

		id, _ := args["id"].(string)
		if id == "" {
			return nil, fmt.Errorf("id is required")
		}

		err := formusecase.DeleteForm(formRepo, tenantID, id)
		if err != nil {
			return nil, err
		}

		return map[string]interface{}{
			"success": true,
			"id":      id,
		}, nil
	})

	s.AddTool("forms.submit", "Submit a form response", func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		tenantID, _ := args["tenant_id"].(string)
		if tenantID == "" {
			return nil, fmt.Errorf("tenant_id is required")
		}

		slug, _ := args["slug"].(string)
		if slug == "" {
			return nil, fmt.Errorf("slug is required")
		}

		var data map[string]interface{}
		if dataRaw, ok := args["data"]; ok && dataRaw != nil {
			dataBytes, _ := json.Marshal(dataRaw)
			_ = json.Unmarshal(dataBytes, &data)
		}

		var files []form.FileUpload
		if filesRaw, ok := args["files"]; ok && filesRaw != nil {
			filesBytes, _ := json.Marshal(filesRaw)
			_ = json.Unmarshal(filesBytes, &files)
		}

		input := formusecase.SubmitFormInput{
			Slug:  slug,
			Data:  data,
			Files: files,
		}

		s, err := formusecase.SubmitForm(formRepo, tenantID, input)
		if err != nil {
			return nil, err
		}

		return map[string]interface{}{
			"id":         s.ID,
			"form_id":    s.FormID,
			"tenant_id":  s.TenantID,
			"data":       s.Data,
			"files":      s.Files,
			"created_at": s.CreatedAt,
		}, nil
	})

	s.AddTool("forms.submissions.list", "List submissions for a form", func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		formID, _ := args["form_id"].(string)
		if formID == "" {
			return nil, fmt.Errorf("form_id is required")
		}

		page, _ := args["page"].(float64)
		limit, _ := args["limit"].(float64)

		offset := int(page) * int(limit)
		limitInt := int(limit)
		if limitInt <= 0 {
			limitInt = 20
		}

		opts := form.ListSubmissionsOptions{
			Offset: offset,
			Limit:  limitInt,
		}

		submissions, total, err := formusecase.ListSubmissions(formRepo, formID, opts)
		if err != nil {
			return nil, err
		}

		result := make([]map[string]interface{}, 0, len(submissions))
		for _, s := range submissions {
			result = append(result, map[string]interface{}{
				"id":         s.ID,
				"form_id":    s.FormID,
				"tenant_id":  s.TenantID,
				"data":       s.Data,
				"files":      s.Files,
				"created_at": s.CreatedAt,
			})
		}

		return map[string]interface{}{
			"submissions": result,
			"total":       total,
			"page":        int(page),
			"limit":       limitInt,
		}, nil
	})

	s.AddTool("forms.submissions.get", "Get a form submission", func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		submissionID, _ := args["submission_id"].(string)
		if submissionID == "" {
			return nil, fmt.Errorf("submission_id is required")
		}

		s, err := formusecase.GetSubmission(formRepo, submissionID)
		if err != nil {
			return nil, err
		}

		return map[string]interface{}{
			"id":         s.ID,
			"form_id":    s.FormID,
			"tenant_id":  s.TenantID,
			"data":       s.Data,
			"files":      s.Files,
			"created_at": s.CreatedAt,
		}, nil
	})
}
