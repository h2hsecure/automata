package page

import (
	"testing"
)

func TestPageValidation(t *testing.T) {
	tests := []struct {
		name    string
		page    *Page
		wantErr bool
	}{
		{
			name:    "empty slug",
			page:    &Page{Title: "Cookie Policy"},
			wantErr: true,
		},
		{
			name:    "empty title",
			page:    &Page{Slug: "cookie-policy"},
			wantErr: true,
		},
		{
			name:    "valid page",
			page:    &Page{Slug: "cookie-policy", Title: "Cookie Policy", Content: "<p>Text</p>", IsActive: true},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.page.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDecisionValidation(t *testing.T) {
	tests := []struct {
		name    string
		action  string
		wantErr bool
	}{
		{name: "link action", action: "link", wantErr: false},
		{name: "dismiss accept action", action: "dismiss_accept", wantErr: false},
		{name: "dismiss decline action", action: "dismiss_decline", wantErr: false},
		{name: "empty action", action: "", wantErr: true},
		{name: "unknown action", action: "explode", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &Decision{ContextID: "ctx", PageID: "p", Action: tt.action}
			err := d.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
