package tenant

import (
	"testing"
	"time"
)

func strPtr(s string) *string { return &s }

func TestTenantValidation(t *testing.T) {
	tests := []struct {
		name    string
		tenant  Tenant
		wantErr bool
	}{
		{
			name:    "valid tenant",
			tenant:  Tenant{Slug: "myapp", Name: "My App"},
			wantErr: false,
		},
		{
			name:    "empty slug",
			tenant:  Tenant{Name: "My App"},
			wantErr: true,
		},
		{
			name:    "empty name",
			tenant:  Tenant{Slug: "myapp"},
			wantErr: true,
		},
		{
			name:    "empty slug and name",
			tenant:  Tenant{},
			wantErr: true,
		},
		{
			name: "tenant with optional fields",
			tenant: Tenant{
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
			err := tt.tenant.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
