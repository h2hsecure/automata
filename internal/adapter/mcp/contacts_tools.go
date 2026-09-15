package mcp

import "context"

// registerContactTools adds contact-related MCP tools.
// These are stubbed for now since the contact domain is not yet implemented.
func (s *Server) registerContactTools() {
	s.AddTool("contacts.list", "List contacts for a tenant", func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		return []map[string]interface{}{}, nil
	})

	s.AddTool("contacts.get", "Get a contact by ID", func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		return nil, nil
	})

	s.AddTool("contacts.create", "Create a new contact", func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		return nil, nil
	})

	s.AddTool("contacts.update", "Update a contact", func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		return nil, nil
	})

	s.AddTool("contacts.delete", "Delete a contact", func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		return nil, nil
	})

	s.AddTool("contacts.merge", "Merge two contacts", func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		return nil, nil
	})

	s.AddTool("contacts.import", "Import contacts from CSV", func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		return nil, nil
	})

	s.AddTool("contacts.export", "Export contacts to CSV", func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		return nil, nil
	})
}
