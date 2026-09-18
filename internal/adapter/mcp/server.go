package mcp

import (
	"context"

	"github.com/hekemen/automata/internal/domain/contact"
	"github.com/hekemen/automata/internal/domain/tenant"
)

// Tool represents an MCP tool.
type Tool struct {
	Name        string
	Description string
	Handler     func(ctx context.Context, args map[string]interface{}) (interface{}, error)
}

// Server is the MCP server that exposes tools for AI integration.
type Server struct {
	tenantRepo   tenant.Repository
	contactRepo  contact.Repository
	tools        []*Tool
}

// NewServer creates a new MCP server.
func NewServer(tenantRepo tenant.Repository, contactRepo contact.Repository) *Server {
	s := &Server{tenantRepo: tenantRepo, contactRepo: contactRepo}
	s.registerBaseTools()
	s.registerContactTools()
	return s
}

// AddTool registers a new tool.
func (s *Server) AddTool(name, description string, handler func(ctx context.Context, args map[string]interface{}) (interface{}, error)) {
	s.tools = append(s.tools, &Tool{
		Name:        name,
		Description: description,
		Handler:     handler,
	})
}

// GetTools returns all registered tools.
func (s *Server) GetTools() []*Tool {
	return s.tools
}

func (s *Server) registerBaseTools() {
	s.AddTool("tenants.list", "List all tenants", func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		tenants, err := s.tenantRepo.List(0, 100)
		if err != nil {
			return nil, err
		}
		result := make([]map[string]interface{}, 0, len(tenants))
		for _, t := range tenants {
			domain := ""
			if t.Domain != nil {
				domain = *t.Domain
			}
			result = append(result, map[string]interface{}{
				"id":        t.ID,
				"slug":      t.Slug,
				"name":      t.Name,
				"domain":    domain,
				"is_active": t.IsActive,
			})
		}
		return result, nil
	})
}
