package config

import "time"

// ConfigKey represents a configuration key for a tenant.
type ConfigKey string

const (
	ConfigKeyCORS    ConfigKey = "cors"
	ConfigKeyDomain  ConfigKey = "domain"
	ConfigKeyDisplay ConfigKey = "display"
)

// CORSConfig holds CORS settings for a tenant.
type CORSConfig struct {
	Origins []string `json:"origins"`
}

// DomainConfig holds domain settings for a tenant.
type DomainConfig struct {
	Primary string   `json:"primary"`
	Aliases []string `json:"aliases"`
}

// DisplayConfig holds display settings for a tenant.
type DisplayConfig struct {
	Name    string `json:"name"`
	LogoURL string `json:"logo_url"`
}

// AdminConfig represents a stored configuration entry.
type AdminConfig struct {
	ID        string
	ContextID string
	Key       ConfigKey
	Value     map[string]interface{}
	CreatedAt time.Time
	UpdatedAt time.Time
}

// ConfigRepository defines the interface for admin config persistence.
type ConfigRepository interface {
	GetByContext(contextID string) (map[ConfigKey]map[string]interface{}, error)
	GetByKey(contextID string, key ConfigKey) (map[string]interface{}, error)
	Upsert(contextID string, key ConfigKey, value map[string]interface{}) error
	Delete(contextID string, key ConfigKey) error
}
