package context

import (
	"testing"
	"time"
)

func strPtr(s string) *string { return &s }

func TestContextValidation(t *testing.T) {
	tests := []struct {
		name    string
		ctx     Context
		wantErr bool
	}{
		{
			name:    "valid context",
			ctx:     Context{Slug: "myapp", Name: "My App"},
			wantErr: false,
		},
		{
			name:    "empty slug",
			ctx:     Context{Name: "My App"},
			wantErr: true,
		},
		{
			name:    "empty name",
			ctx:     Context{Slug: "myapp"},
			wantErr: true,
		},
		{
			name:    "empty slug and name",
			ctx:     Context{},
			wantErr: true,
		},
		{
			name: "context with optional fields",
			ctx: Context{
				ID:       "550e8400-e29b-41d4-a716-446655440000",
				Slug:     "myapp",
				Name:     "My App",
				Domain:   strPtr("myapp.example.com"),
				IsActive: true,
				Settings: map[string]interface{}{"theme": "dark"},
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.ctx.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
