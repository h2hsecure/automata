package geo

import (
	"testing"

	geodomain "github.com/h2hsecure/automata/internal/domain/geo"
)

// TestNew returns a non-nil lookup backed by the embedded GeoLite2-City
// database.
func TestNew(t *testing.T) {
	lookup := New()
	if lookup == nil {
		t.Fatal("New() returned nil lookup")
	}
}

// TestLocate asserts stable GeoLite2-City values for well-known IPs so the
// tests stay valid across database refreshes (these addresses are not
// re-allocated in practice).
func TestLocate(t *testing.T) {
	lookup := New()

	tests := []struct {
		name        string
		ip          string
		wantCountry string
		wantCode    string
		wantCity    string
		wantRegion  string
		wantOK      bool
	}{
		{"city and region", "208.65.44.17", "United States", "US", "Naples", "Florida", true},
		{"german city", "140.82.121.4", "Germany", "DE", "Frankfurt am Main", "Hesse", true},
		{"city without region", "156.146.56.186", "Singapore", "SG", "Singapore", "", true},
		{"country only", "8.8.8.8", "United States", "US", "", "", true},
		{"no data", "203.0.113.7", "", "", "", "", false},
		{"invalid ip", "not-an-ip", "", "", "", "", false},
		{"empty ip", "", "", "", "", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loc, ok := lookup.Locate(tt.ip)
			if ok != tt.wantOK {
				t.Fatalf("Locate(%q) ok = %v, want %v (loc=%+v)", tt.ip, ok, tt.wantOK, loc)
			}
			if tt.wantOK {
				if loc.Country != tt.wantCountry || loc.CountryCode != tt.wantCode ||
					loc.City != tt.wantCity || loc.Region != tt.wantRegion {
					t.Errorf("Locate(%q) = %+v, want %+v", tt.ip, loc,
						geodomain.Location{Country: tt.wantCountry, CountryCode: tt.wantCode, City: tt.wantCity, Region: tt.wantRegion})
				}
			}
		})
	}
}

// TestLocateImplementsDomainInterface guards the hexagonal contract: the
// provider must satisfy the domain Lookup interface.
func TestLocateImplementsDomainInterface(t *testing.T) {
	var _ geodomain.Lookup = New()
}