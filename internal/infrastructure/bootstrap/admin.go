package bootstrap

import (
	"context"
	"crypto/rand"
	"fmt"
	"os"

	"github.com/google/uuid"
	ctxdomain "github.com/hekemen/automata/internal/domain/context"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/bcrypt"
)

// Run creates an admin user on first startup if no users exist and writes
// credentials to stdout. It does not create contexts: the first context is
// created by the user through the onboarding flow, which grants the creator
// an owner membership. Returns nil if users already exist.
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

	// Use provided password or generate random one
	password := os.Getenv("AUTOMATA_ADMIN_PASSWORD")
	if password == "" {
		password, err = generatePassword(16)
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

	// Log credentials to stdout
	fmt.Println("")
	fmt.Println("╔══════════════════════════════════════════════════╗")
	fmt.Println("║  ADMIN CREDENTIALS (first startup only)          ║")
	fmt.Printf("║  Email:    %s                          ║\n", adminEmail)
	fmt.Printf("║  Password: %s                      ║\n", password)
	fmt.Println("╚══════════════════════════════════════════════════╝")
	fmt.Println("")

	log.Info().Str("email", adminEmail).Msg("admin user created via bootstrap")
	return nil
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
