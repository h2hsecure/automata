// Package geo implements IP geolocation against the embedded GeoLite2-City
// MaxMind database. The database is compiled into the binary via go:embed so
// it ships with the Docker image and requires no network access at runtime.
package geo

import (
	"embed"
	"net"

	geoip2 "github.com/oschwald/geoip2-golang"

	geodomain "github.com/h2hsecure/automata/internal/domain/geo"
)

//go:embed data/GeoLite2-City.mmdb
var mmdbFS embed.FS

// provider resolves IP addresses from the embedded GeoLite2-City database.
type provider struct {
	db *geoip2.Reader
}

// New opens the embedded GeoLite2-City database and returns a domain
// Lookup. If the embedded database is unreadable, it returns a no-op lookup
// so geolocation fails soft instead of breaking tracking.
func New() geodomain.Lookup {
	b, err := mmdbFS.ReadFile("data/GeoLite2-City.mmdb")
	if err != nil {
		return noop{}
	}
	db, err := geoip2.FromBytes(b)
	if err != nil {
		return noop{}
	}
	return &provider{db: db}
}

// Locate resolves an IP address to a Location. It returns ok=false for
// unparseable addresses and for addresses with no data in the database.
func (p *provider) Locate(ip string) (geodomain.Location, bool) {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return geodomain.Location{}, false
	}

	city, err := p.db.City(parsed)
	if err != nil {
		return geodomain.Location{}, false
	}

	loc := geodomain.Location{
		Country:     city.Country.Names["en"],
		CountryCode: city.Country.IsoCode,
		City:        city.City.Names["en"],
	}
	if len(city.Subdivisions) > 0 {
		loc.Region = city.Subdivisions[0].Names["en"]
	}
	// A hit with no fields at all is a miss.
	if loc.Country == "" && loc.City == "" {
		return geodomain.Location{}, false
	}
	return loc, true
}

// noop is a Lookup that never resolves, used when the embedded database is
// unavailable.
type noop struct{}

func (noop) Locate(string) (geodomain.Location, bool) { return geodomain.Location{}, false }