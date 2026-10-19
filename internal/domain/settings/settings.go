package settings

import "errors"

var ErrNotFound = errors.New("setting not found")

// Repository is the persistence port for global key-value settings.
type Repository interface {
	List() (map[string]string, error)
	Get(key string) (string, error)
	Upsert(key, value string) error
	Delete(key string) error
}

// ContextRepository is the persistence port for per-context key-value settings.
type ContextRepository interface {
	List(contextID string) (map[string]string, error)
	Get(contextID, key string) (string, error)
	Upsert(contextID, key, value string) error
	Delete(contextID, key string) error
}
