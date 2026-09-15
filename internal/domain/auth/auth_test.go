package auth_test

import (
	"testing"

	"github.com/hekemen/automata/internal/infrastructure/auth"
)

func TestHashPasswordRoundTrip(t *testing.T) {
	svc := auth.NewService(nil)
	password := "mySecurePassword123"

	hash, err := svc.HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}

	err = svc.ComparePassword(hash, password)
	if err != nil {
		t.Fatalf("ComparePassword() error = %v", err)
	}
}

func TestDifferentPasswordsProduceDifferentHashes(t *testing.T) {
	svc := auth.NewService(nil)

	hash1, _ := svc.HashPassword("password1")
	hash2, _ := svc.HashPassword("password2")

	if hash1 == hash2 {
		t.Fatal("different passwords should produce different hashes")
	}
}

func TestWrongPasswordFails(t *testing.T) {
	svc := auth.NewService(nil)

	hash, _ := svc.HashPassword("correctPassword")
	err := svc.ComparePassword(hash, "wrongPassword")
	if err == nil {
		t.Fatal("ComparePassword should fail for wrong password")
	}
}
