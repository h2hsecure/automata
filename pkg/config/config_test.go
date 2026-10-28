package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/h2hsecure/automata/internal/infrastructure/config"
)

func TestConfigGetDefault(t *testing.T) {
	config.Reset()
	got := config.Get("nonexistent.key")
	if got != "" {
		t.Errorf("Get(\"nonexistent.key\") = %q, want empty string", got)
	}
}

func TestConfigSetAndGet(t *testing.T) {
	config.Reset()
	config.Set("server.host", "localhost")
	config.Set("server.port", "8080")

	if got := config.Get("server.host"); got != "localhost" {
		t.Errorf("Get(\"server.host\") = %q, want %q", got, "localhost")
	}
	if got := config.Get("server.port"); got != "8080" {
		t.Errorf("Get(\"server.port\") = %q, want %q", got, "8080")
	}
}

func TestConfigLoadYAML(t *testing.T) {
	config.Reset()

	yamlContent := `
server:
  host: localhost
  port: "8080"
database:
  host: db.example.com
  port: "5432"
  name: automata
`
	tmpDir := t.TempDir()
	yamlFile := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(yamlFile, []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}

	if err := config.Load(yamlFile); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		key    string
		expect string
	}{
		{"server.host", "localhost"},
		{"server.port", "8080"},
		{"database.host", "db.example.com"},
		{"database.port", "5432"},
		{"database.name", "automata"},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			got := config.Get(tt.key)
			if got != tt.expect {
				t.Errorf("Get(%q) = %q, want %q", tt.key, got, tt.expect)
			}
		})
	}
}

func TestConfigEnvOverride(t *testing.T) {
	config.Reset()

	yamlContent := `
server:
  host: localhost
  port: "8080"
`
	tmpDir := t.TempDir()
	yamlFile := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(yamlFile, []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}

	os.Setenv("AUTOMATA_SERVER_HOST", "prod.example.com")
	defer os.Unsetenv("AUTOMATA_SERVER_HOST")

	if err := config.Load(yamlFile); err != nil {
		t.Fatal(err)
	}

	if got := config.Get("server.host"); got != "prod.example.com" {
		t.Errorf("Get(\"server.host\") = %q, want %q (env override)", got, "prod.example.com")
	}

	if got := config.Get("server.port"); got != "8080" {
		t.Errorf("Get(\"server.port\") = %q, want %q (unchanged from YAML)", got, "8080")
	}
}
