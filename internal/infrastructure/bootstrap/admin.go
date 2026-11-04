package bootstrap

import (
	"context"
	"crypto/rand"
	"fmt"
	"os"

	"github.com/google/uuid"
	ctxdomain "github.com/h2hsecure/automata/internal/domain/context"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/bcrypt"
)

// secretFileEnv selects where generated credentials are written;
// secretFileDefault is used when the env var is unset.
const (
	secretFileEnv     = "AUTOMATA_ADMIN_SECRET_FILE"
	secretFileDefault = ".admin_credentials"
)

// Run creates an admin user on first startup if no users exist. A
// generated password is written to a 0600 secret file (path from
// AUTOMATA_ADMIN_SECRET_FILE, default ".admin_credentials"); credentials
// are never printed to stdout or logs. It does not create contexts: the
// first context is created by the user through the onboarding flow, which
// grants the creator an owner membership. Returns nil if users already
// exist.
func Run(pool *pgxpool.Pool, userRepo ctxdomain.UserRepository) error {
	// Check if any users exist in the users table
	exists, err := userRepo.ExistsAnyUser()
	if err != nil {
		return fmt.Errorf("check users existence: %w", err)
	}
	if exists {
		return nil // Already initialized
	}

	// Also check context_users for backward compat
	var ctxUserCount int
	if err := pool.QueryRow(context.Background(), "SELECT COUNT(*) FROM context_users").Scan(&ctxUserCount); err == nil && ctxUserCount > 0 {
		return nil // context_users has data, migration handles it
	}

	// Read config
	adminEmail := os.Getenv("AUTOMATA_ADMIN_EMAIL")
	if adminEmail == "" {
		adminEmail = "admin@automata.local"
	}

	// Use provided password or generate random one. Track whether we
	// generated it so we know whether a secret file is required.
	password := os.Getenv("AUTOMATA_ADMIN_PASSWORD")
	generated := false
	if password == "" {
		password, err = generatePassword(16)
		generated = true
	}
	if err != nil {
		return fmt.Errorf("generate password: %w", err)
	}

	// Hash password
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	// Create admin user
	adminUser := &ctxdomain.User{
		ID:           uuid.New().String(),
		Email:        adminEmail,
		PasswordHash: string(hash),
		IsAdmin:      true,
	}
	if err := userRepo.Create(adminUser); err != nil {
		return fmt.Errorf("create admin user: %w", err)
	}

	// Write marker file
	markerPath := ".admin_initialized"
	if err := os.WriteFile(markerPath, []byte("initialized"), 0644); err != nil {
		log.Warn().Err(err).Msg("failed to write admin initialized marker")
	}

	// A generated password must reach the operator without passing through
	// stdout/logs. Write it to a 0600 secret file and log only the path.
	// When the operator supplied the password explicitly there is nothing to
	// disclose, so no file is written.
	if generated {
		secretPath := os.Getenv(secretFileEnv)
		if secretPath == "" {
			secretPath = secretFileDefault
		}
		if err := writeCredentialsFile(secretPath, adminEmail, password); err != nil {
			return fmt.Errorf("write credentials file: %w", err)
		}
		log.Info().Str("email", adminEmail).Str("credentials_file", secretPath).
			Msg("admin user created via bootstrap; generated password written to secret file")
	} else {
		log.Info().Str("email", adminEmail).Msg("admin user created via bootstrap")
	}
	return nil
}

// writeCredentialsFile writes the generated credentials to path with
// owner-only permissions. It never returns the password through any other
// channel.
func writeCredentialsFile(path, email, password string) error {
	content := fmt.Sprintf("email=%s\npassword=%s\n", email, password)
	return os.WriteFile(path, []byte(content), 0600)
}

func generatePassword(length int) (string, error) {
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*"
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	for i := range bytes {
		bytes[i] = chars[int(bytes[i])%len(chars)]
	}
	return string(bytes), nil
}
