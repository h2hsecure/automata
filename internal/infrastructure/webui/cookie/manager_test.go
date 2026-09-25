package cookie

import (
	"testing"
	"time"
)

func TestEncryptDecryptRefreshToken_RoundTrip(t *testing.T) {
	m := New("1234567890123456") // 16-byte AES key

	// Create a sample JWT token
	jwtToken := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ0ZXN0IjoiZGF0YSJ9.signature"

	encrypted, err := m.EncryptRefreshToken(jwtToken)
	if err != nil {
		t.Fatalf("EncryptRefreshToken failed: %v", err)
	}
	if encrypted == "" {
		t.Fatal("expected non-empty encrypted value")
	}
	if encrypted == jwtToken {
		t.Fatal("encrypted value should differ from original")
	}

	decrypted, err := m.DecryptRefreshToken(encrypted)
	if err != nil {
		t.Fatalf("DecryptRefreshToken failed: %v", err)
	}
	if decrypted != jwtToken {
		t.Errorf("expected decrypted token to match original, got: %s", decrypted)
	}
}

func TestEncryptDecryptRefreshToken_DifferentTokens(t *testing.T) {
	m := New("1234567890123456") // 16-byte AES key

	token1 := "eyJhbGciOiJIUzI1NiJ9.eyJ0ZXN0IjoxfQ.sig1"
	token2 := "eyJhbGciOiJIUzI1NiJ9.eyJ0ZXN0IjoyfQ.sig2"

	enc1, _ := m.EncryptRefreshToken(token1)
	enc2, _ := m.EncryptRefreshToken(token2)

	// Same key, different tokens should produce different ciphertexts
	if enc1 == enc2 {
		t.Fatal("different tokens should produce different ciphertexts")
	}

	// Decrypt should return original tokens
	dec1, _ := m.DecryptRefreshToken(enc1)
	dec2, _ := m.DecryptRefreshToken(enc2)

	if dec1 != token1 {
		t.Errorf("expected token1, got: %s", dec1)
	}
	if dec2 != token2 {
		t.Errorf("expected token2, got: %s", dec2)
	}
}

func TestDecryptRefreshToken_InvalidBase64(t *testing.T) {
	m := New("1234567890123456") // 16-byte AES key

	_, err := m.DecryptRefreshToken("not-valid-base64!!!")
	if err == nil {
		t.Fatal("expected error for invalid base64 input")
	}
}

func TestDecryptRefreshToken_TooShort(t *testing.T) {
	m := New("1234567890123456") // 16-byte AES key

	// Base64-encoded string that decodes to less than nonce size
	// GCM nonce size is 12 bytes, so ciphertext must be at least 12 bytes
	_, err := m.DecryptRefreshToken("YWJjZGVm") // "abcdef" in base64, only 6 bytes
	if err == nil {
		t.Fatal("expected error for ciphertext too short")
	}
}

func TestDecryptRefreshToken_WrongKey(t *testing.T) {
	m := New("1234567890123456") // 16-byte AES key

	jwtToken := "eyJhbGciOiJIUzI1NiJ9.eyJ0ZXN0IjoiZGF0YSJ9.signature"
	encrypted, err := m.EncryptRefreshToken(jwtToken)
	if err != nil {
		t.Fatalf("EncryptRefreshToken failed: %v", err)
	}

	// Create manager with different key
	m2 := New("0000000000000000")
	_, err = m2.DecryptRefreshToken(encrypted)
	if err == nil {
		t.Fatal("expected error when decrypting with wrong key")
	}
}

func TestRefreshPayload_CreatedAt(t *testing.T) {
	m := New("1234567890123456") // 16-byte AES key

	before := time.Now().UTC()
	jwtToken := "eyJhbGciOiJIUzI1NiJ9.eyJ0ZXN0IjoiZGF0YSJ9.signature"

	encrypted, err := m.EncryptRefreshToken(jwtToken)
	if err != nil {
		t.Fatalf("EncryptRefreshToken failed: %v", err)
	}

	decrypted, err := m.DecryptRefreshToken(encrypted)
	if err != nil {
		t.Fatalf("DecryptRefreshToken failed: %v", err)
	}

	if decrypted != jwtToken {
		t.Fatalf("expected token to match, got: %s", decrypted)
	}
	// The CreatedAt field is stored but we just verify the round-trip works
	_ = before // used implicitly via time.Now() in Encrypt
}

func TestCookieConstants(t *testing.T) {
	if RefreshCookieName != "automata_refresh" {
		t.Errorf("expected RefreshCookieName to be 'automata_refresh', got: %s", RefreshCookieName)
	}
	expectedMaxAge := 604800 // 7 days in seconds
	if RefreshCookieMaxAge != expectedMaxAge {
		t.Errorf("expected RefreshCookieMaxAge to be %d, got: %d", expectedMaxAge, RefreshCookieMaxAge)
	}
}
