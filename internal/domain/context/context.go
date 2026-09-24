package context

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

var (
	slugRegex = regexp.MustCompile(`^[a-z0-9][a-z0-9\-]{0,62}$`)
	domainRegex = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9\-\.]*\.[a-zA-Z]{2,}$`)
)

// Context represents a tenant/website context in the platform.
// Each context has its own contacts, forms, tracking data, banners, and configuration.
type Context struct {
	ID        string
	Slug      string
	Name      string
	Domain    *string
	IsActive  bool
	Settings  map[string]interface{}
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Validate checks that the Context has all required fields set.
func (c *Context) Validate() error {
	if c.Slug == "" {
		return errors.New("slug is required")
	}
	if c.Name == "" {
		return errors.New("name is required")
	}

	// Validate slug format: lowercase alphanumeric + hyphens, max 64 chars
	if !slugRegex.MatchString(c.Slug) {
		return fmt.Errorf("invalid slug format: must be lowercase alphanumeric with hyphens, max 64 characters")
	}
	if len(c.Slug) > 64 {
		return fmt.Errorf("slug must be 64 characters or less")
	}

	// Validate domain format if provided
	if c.Domain != nil && *c.Domain != "" {
		if !domainRegex.MatchString(*c.Domain) {
			return fmt.Errorf("invalid domain format: %s", *c.Domain)
		}
		if len(*c.Domain) > 256 {
			return fmt.Errorf("domain must be 256 characters or less")
		}
	}

	return nil
}

// ValidateSettings checks the context settings for common issues.
func (c *Context) ValidateSettings() error {
	if c.Settings == nil {
		return nil
	}

	for k, v := range c.Settings {
		if !isValidJSONKey(k) {
			return fmt.Errorf("invalid settings key: %s", k)
		}
		if !isValidJSONValue(v) {
			return fmt.Errorf("invalid settings value for key %s", k)
		}
	}

	return nil
}

// Clone creates a deep copy of the context.
func (c *Context) Clone() *Context {
	clone := &Context{
		ID:        c.ID,
		Slug:      c.Slug,
		Name:      c.Name,
		IsActive:  c.IsActive,
		CreatedAt: c.CreatedAt,
		UpdatedAt: c.UpdatedAt,
	}
	if c.Domain != nil {
		d := *c.Domain
		clone.Domain = &d
	}
	if c.Settings != nil {
		clone.Settings = make(map[string]interface{})
		for k, v := range c.Settings {
			clone.Settings[k] = v
		}
	}
	return clone
}

// MarshalJSON implements custom JSON marshaling for context settings.
func (c *Context) MarshalJSON() ([]byte, error) {
	type Alias Context
	aux := &struct {
		*Alias
		Settings json.RawMessage `json:"settings,omitempty"`
	}{
		Alias: (*Alias)(c),
	}

	if c.Settings != nil {
		settingsBytes, err := json.Marshal(c.Settings)
		if err != nil {
			return nil, fmt.Errorf("marshal settings: %w", err)
		}
		aux.Settings = settingsBytes
	} else {
		aux.Settings = json.RawMessage("{}")
	}

	return json.Marshal(aux)
}

// UnmarshalJSON implements custom JSON unmarshaling for context settings.
func (c *Context) UnmarshalJSON(data []byte) error {
	type Alias Context
	aux := &struct {
		*Alias
		Settings json.RawMessage `json:"settings,omitempty"`
	}{
		Alias: (*Alias)(c),
	}

	if err := json.Unmarshal(data, aux); err != nil {
		return fmt.Errorf("unmarshal context: %w", err)
	}

	c.ID = aux.ID
	c.Slug = aux.Slug
	c.Name = aux.Name
	c.Domain = aux.Domain
	c.IsActive = aux.IsActive
	c.CreatedAt = aux.CreatedAt
	c.UpdatedAt = aux.UpdatedAt

	if len(aux.Settings) > 0 {
		c.Settings = make(map[string]interface{})
		if err := json.Unmarshal(aux.Settings, &c.Settings); err != nil {
			return fmt.Errorf("unmarshal settings: %w", err)
		}
	}

	return nil
}

// IsActiveContext returns true if the context is active and has a valid domain or slug.
func (c *Context) IsActiveContext() bool {
	return c.IsActive && (c.Domain != nil && *c.Domain != "" || c.Slug != "")
}

// GetDomain returns the context domain, falling back to the slug if no domain is set.
func (c *Context) GetDomain() string {
	if c.Domain != nil && *c.Domain != "" {
		return *c.Domain
	}
	return c.Slug
}

// GetSetting retrieves a setting value by key.
func (c *Context) GetSetting(key string) interface{} {
	if c.Settings == nil {
		return nil
	}
	return c.Settings[key]
}

// SetSetting sets a setting value by key.
func (c *Context) SetSetting(key string, value interface{}) {
	if c.Settings == nil {
		c.Settings = make(map[string]interface{})
	}
	c.Settings[key] = value
}

// RemoveSetting removes a setting by key.
func (c *Context) RemoveSetting(key string) {
	if c.Settings != nil {
		delete(c.Settings, key)
	}
}

// HasSetting checks if a setting key exists.
func (c *Context) HasSetting(key string) bool {
	if c.Settings == nil {
		return false
	}
	_, ok := c.Settings[key]
	return ok
}

// ToMap converts the context to a map representation.
func (c *Context) ToMap() map[string]interface{} {
	result := map[string]interface{}{
		"id":         c.ID,
		"slug":       c.Slug,
		"name":       c.Name,
		"is_active":  c.IsActive,
		"created_at": c.CreatedAt,
		"updated_at": c.UpdatedAt,
	}

	if c.Domain != nil {
		result["domain"] = *c.Domain
	} else {
		result["domain"] = nil
	}

	if c.Settings != nil {
		result["settings"] = c.Settings
	} else {
		result["settings"] = map[string]interface{}{}
	}

	return result
}

// FromMap populates the context from a map representation.
func (c *Context) FromMap(m map[string]interface{}) error {
	if id, ok := m["id"].(string); ok {
		c.ID = id
	}
	if slug, ok := m["slug"].(string); ok {
		c.Slug = slug
	}
	if name, ok := m["name"].(string); ok {
		c.Name = name
	}
	if isActive, ok := m["is_active"].(bool); ok {
		c.IsActive = isActive
	}
	if createdAt, ok := m["created_at"].(time.Time); ok {
		c.CreatedAt = createdAt
	}
	if updatedAt, ok := m["updated_at"].(time.Time); ok {
		c.UpdatedAt = updatedAt
	}
	if domain, ok := m["domain"].(string); ok && domain != "" {
		c.Domain = &domain
	}
	if settings, ok := m["settings"].(map[string]interface{}); ok {
		c.Settings = settings
	}

	return c.Validate()
}

// DisplayName returns the context name for display purposes.
func (c *Context) DisplayName() string {
	if c.Name != "" {
		return c.Name
	}
	return c.Slug
}

// DisplaySlug returns the context slug for display purposes.
func (c *Context) DisplaySlug() string {
	// Replace hyphens with spaces for display
	return strings.ReplaceAll(c.Slug, "-", " ")
}

// FormatCreatedAt returns a formatted creation timestamp.
func (c *Context) FormatCreatedAt() string {
	return c.CreatedAt.Format("2006-01-02 15:04:05")
}

// FormatUpdatedAt returns a formatted update timestamp.
func (c *Context) FormatUpdatedAt() string {
	return c.UpdatedAt.Format("2006-01-02 15:04:05")
}

// IsNew checks if the context has not been persisted yet.
func (c *Context) IsNew() bool {
	return c.ID == ""
}

// IsValid checks if the context passes all validations.
func (c *Context) IsValid() bool {
	return c.Validate() == nil
}

// Equal checks if two contexts are equal.
func (c *Context) Equal(other *Context) bool {
	if c == nil || other == nil {
		return c == other
	}
	return c.ID == other.ID &&
		c.Slug == other.Slug &&
		c.Name == other.Name &&
		c.IsActive == other.IsActive &&
		c.CreatedAt.Equal(other.CreatedAt) &&
		c.UpdatedAt.Equal(other.UpdatedAt)
}

// String returns a string representation of the context.
func (c *Context) String() string {
	return fmt.Sprintf("Context{ID: %s, Slug: %s, Name: %s, IsActive: %v}",
		c.ID, c.Slug, c.Name, c.IsActive)
}

// isValidJSONKey checks if a settings key is valid.
func isValidJSONKey(key string) bool {
	if key == "" {
		return false
	}
	if len(key) > 128 {
		return false
	}
	return true
}

// isValidJSONValue checks if a settings value is a valid JSON value.
func isValidJSONValue(v interface{}) bool {
	switch v.(type) {
	case string, int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64,
		float32, float64, bool, nil:
		return true
	case map[string]interface{}, []interface{}:
		return true
	default:
		return false
	}
}
