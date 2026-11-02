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
