package auth

import (
	"fmt"
	"os"

	"github.com/hekemen/automata/internal/infrastructure/config"
	"github.com/rs/zerolog/log"
)

// Secrets holds the application's cryptographic secrets, loaded and validated
// at startup. All callers should access secrets through GetJWTSecret() and
// GetCookieSecret() rather than reading config directly.
type Secrets struct {
	jwtSecret    string
	cookieSecret string
}

var globalSecrets *Secrets

// Init loads all secrets from configuration and validates that they are set.
// It panics on startup if any required secret is missing — this ensures
// deployments without secrets fail fast rather than falling back to weak defaults.
func Init() {
	jwtSecret := config.Get("auth.secret_key")
	if jwtSecret == "" {
		jwtSecret = os.Getenv("AUTOMATA_AUTH_SECRET_KEY")
	}
	if jwtSecret == "" {
		log.Fatal().Msg("FATAL: auth.secret_key is not set. Set config.auth.secret_key in config.yaml or the AUTOMATA_AUTH_SECRET_KEY environment variable. Without this, JWT tokens are forgeable.")
	}

	cookieSecret := config.Get("auth.cookie_secret")
	if cookieSecret == "" {
		cookieSecret = os.Getenv("AUTOMATA_COOKIE_SECRET")
	}
	if cookieSecret == "" {
		log.Fatal().Msg("FATAL: auth.cookie_secret is not set. Set config.auth.cookie_secret in config.yaml or the AUTOMATA_COOKIE_SECRET environment variable. Without this, session cookies can be forged.")
	}

	// Validate minimum lengths
	if len(jwtSecret) < 16 {
		log.Fatal().Msg("FATAL: auth.secret_key must be at least 16 characters long for HMAC signing security.")
	}
	if len(cookieSecret) < 16 {
		log.Fatal().Msg("FATAL: auth.cookie_secret must be at least 16 characters long for AES-GCM encryption security.")
	}

	globalSecrets = &Secrets{
		jwtSecret:    jwtSecret,
		cookieSecret: cookieSecret,
	}

	log.Info().Msg("Secret keys loaded successfully")
}

// GetJWTSecret returns the JWT signing secret. Must be called after Init().
func GetJWTSecret() string {
	if globalSecrets == nil {
		panic("secrets: GetJWTSecret() called before Init()")
	}
	return globalSecrets.jwtSecret
}

// GetCookieSecret returns the cookie encryption secret. Must be called after Init().
func GetCookieSecret() string {
	if globalSecrets == nil {
		panic("secrets: GetCookieSecret() called before Init()")
	}
	return globalSecrets.cookieSecret
}

// InitForTesting initializes secrets with known test values.
// This is for testing purposes only.
func InitForTesting(jwtSecret, cookieSecret string) {
	globalSecrets = &Secrets{
		jwtSecret:    jwtSecret,
		cookieSecret: cookieSecret,
	}
}

// Reset clears the global secrets. Used for testing isolation.
func Reset() {
	globalSecrets = nil
}

// VerifySecretsLoaded returns an error if Init() has not been called.
func VerifySecretsLoaded() error {
	if globalSecrets == nil {
		return fmt.Errorf("secrets not initialized — call Init() before use")
	}
	return nil
}
