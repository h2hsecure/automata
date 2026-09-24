package contact

import (
	"testing"
)

func TestContactValidation(t *testing.T) {
	tests := []struct {
		name    string
		contact Contact
		wantErr bool
	}{
		{
			name: "valid contact with email",
			contact: Contact{
				ContextID: "550e8400-e29b-41d4-a716-446655440000",
				Email:    strPtr("user@example.com"),
			},
			wantErr: false,
		},
		{
			name: "valid contact without email (unverified)",
			contact: Contact{
				ContextID: "550e8400-e29b-41d4-a716-446655440000",
				Email:    nil,
			},
			wantErr: false,
		},
		{
			name: "missing tenant_id",
			contact: Contact{
				Email: strPtr("user@example.com"),
			},
			wantErr: true,
		},
		{
			name: "invalid email format",
			contact: Contact{
				ContextID: "550e8400-e29b-41d4-a716-446655440000",
				Email:    strPtr("not-an-email"),
			},
			wantErr: true,
		},
		{
			name: "email with @ but no domain",
			contact: Contact{
				ContextID: "550e8400-e29b-41d4-a716-446655440000",
				Email:    strPtr("user@"),
			},
			wantErr: true,
		},
		{
			name: "email with @ but no local part",
			contact: Contact{
				ContextID: "550e8400-e29b-41d4-a716-446655440000",
				Email:    strPtr("@domain.com"),
			},
			wantErr: true,
		},
		{
			name: "empty email string",
			contact: Contact{
				ContextID: "550e8400-e29b-41d4-a716-446655440000",
				Email:    strPtr(""),
			},
			wantErr: false,
		},
		{
			name: "email too long",
			contact: Contact{
				ContextID: "550e8400-e29b-41d4-a716-446655440000",
				Email:    strPtr("a" + string(make([]byte, 255)) + "@example.com"),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.contact.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestContactIsVerified(t *testing.T) {
	tests := []struct {
		name     string
		contact  Contact
		expected bool
	}{
		{
			name:     "email set",
			contact:  Contact{Email: strPtr("user@example.com")},
			expected: true,
		},
		{
			name:     "email nil",
			contact:  Contact{Email: nil},
			expected: false,
		},
		{
			name:     "email empty string",
			contact:  Contact{Email: strPtr("")},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.contact.IsVerified(); got != tt.expected {
				t.Errorf("IsVerified() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestTagValidation(t *testing.T) {
	tests := []struct {
		name    string
		tag     Tag
		wantErr bool
	}{
		{
			name: "valid tag",
			tag:  Tag{Name: "lead", Color: "#6366f1"},
		},
		{
			name:    "empty tag name",
			tag:     Tag{Color: "#6366f1"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.tag.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func strPtr(s string) *string {
	return &s
}
