// Package geo defines the domain contract for IP geolocation.
package geo

// Location is the geographic location resolved from an IP address.
// Fields may be empty when the database has no data at that level.
type Location struct {
	Country     string
	CountryCode string
	City        string
	Region      string
}

// Lookup resolves an IP address to a Location. The bool return reports
// whether the database had any data for the address; implementations must
// never fail tracking because of a miss.
type Lookup interface {
	Locate(ip string) (Location, bool)
}