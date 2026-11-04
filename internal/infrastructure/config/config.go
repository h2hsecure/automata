package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

// Config is a thread-safe key-value configuration store.
type Config struct {
	mu     sync.RWMutex
	values map[string]string
}

var global = &Config{values: make(map[string]string)}

// Get returns the value for the given key, or empty string if not set.
func Get(key string) string {
	global.mu.RLock()
	defer global.mu.RUnlock()
	return global.values[key]
}

// GetInt returns the integer value for the given key, or def if the key is
// unset or the value is not a valid integer.
func GetInt(key string, def int) int {
	v := Get(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

// Set stores a key-value pair.
func Set(key, value string) {
	global.mu.Lock()
	defer global.mu.Unlock()
	global.values[key] = value
}

// Load reads configuration from a YAML file, flattens nested keys to dot-notation,
// then overrides with environment variables (format: AUTOMATA_<KEY> where <KEY>
// is the dot-notation key uppercased with dots replaced by underscores).
func Load(path string) error {
	return global.loadFromFile(path)
}

// Reset clears all configuration values. Used for testing.
func Reset() {
	global.mu.Lock()
	defer global.mu.Unlock()
	global.values = make(map[string]string)
}

// loadFromFile reads a YAML file, flattens nested keys to dot-notation,
// stores them in the config, then overrides with environment variables.
func (c *Config) loadFromFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	flat, err := flattenYAML(data)
	if err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.values = flat

	applyEnvOverrides(c.values)
	return nil
}

// flattenYAML parses YAML bytes into a flat map with dot-notation keys.
func flattenYAML(data []byte) (map[string]string, error) {
	var raw interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	flat := make(map[string]string)
	walk(nil, raw, flat)
	return flat, nil
}

// walk recursively flattens nested structures into dot-notation keys.
func walk(path []string, v interface{}, flat map[string]string) {
	switch val := v.(type) {
	case map[string]interface{}:
		for k, child := range val {
			walk(append(path, k), child, flat)
		}
	case map[interface{}]interface{}:
		for k, child := range val {
			walk(append(path, fmt.Sprintf("%v", k)), child, flat)
		}
	case []interface{}:
		for i, item := range val {
			walk(append(path, fmt.Sprintf("%d", i)), item, flat)
		}
	default:
		key := strings.Join(path, ".")
		if key != "" {
			flat[key] = formatValue(val)
		}
	}
}

// formatValue converts a value to its string representation.
func formatValue(v interface{}) string {
	if v == nil {
		return ""
	}
	return fmt.Sprintf("%v", v)
}

// applyEnvOverrides overrides config values with environment variables.
// Environment variable format: AUTOMATA_<KEY> where <KEY> is the dot-notation
// key uppercased with dots replaced by underscores (e.g., server.host -> AUTOMATA_SERVER_HOST).
func applyEnvOverrides(values map[string]string) {
	prefix := "AUTOMATA_"
	for key := range values {
		envKey := prefix + strings.ToUpper(strings.ReplaceAll(key, ".", "_"))
		if envVal, ok := os.LookupEnv(envKey); ok {
			values[key] = envVal
		}
	}
}
