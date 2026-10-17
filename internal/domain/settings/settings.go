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