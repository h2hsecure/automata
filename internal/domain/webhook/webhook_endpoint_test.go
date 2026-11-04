package webhook

import "testing"

// TestWebhookURL_Anchored is the regression test for the SSRF allowlist:
// the pattern must be anchored on the host boundary so prefix tricks like
// http://localhost.evil.com cannot match the loopback carve-out.
func TestWebhookURL_Anchored(t *testing.T) {
	tests := []struct {
		name string
		url  string
		ok   bool
	}{
		{"https any host", "https://example.com/x", true},
		{"https bare host", "https://example.com", true},
		{"http loopback", "http://localhost/x", true},
		{"http loopback with port", "http://127.0.0.1:8080/x", true},
		{"http localhost with port", "http://localhost:9000", true},
		{"loopback prefix trick", "http://localhost.evil.com/x", false},
		{"loopback prefix trick with path", "http://127.0.0.1.evil.com/x", false},
		{"cloud metadata IP", "http://169.254.169.254/latest", false},
		{"plain http remote host", "http://example.com/x", false},
		{"ftp scheme", "ftp://x", false},
		{"missing scheme slash", "https:evil.com", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsValidWebhookURL(tt.url); got != tt.ok {
				t.Errorf("IsValidWebhookURL(%q) = %v, want %v", tt.url, got, tt.ok)
			}
		})
	}
}

// TestValidateCreate_RejectsSSRFURL ensures the anchored pattern is enforced
// at create/update validation time.
func TestValidateCreate_RejectsSSRFURL(t *testing.T) {
	input := CreateWebhookInput{
		ContextID: "11111111-1111-1111-1111-111111111111",
		Name:      "hook",
		URL:       "http://localhost.evil.com/x",
		Events:    []string{"test.ping"},
	}
	if err := input.ValidateCreate(); err == nil {
		t.Error("ValidateCreate accepted a loopback-prefix URL, want rejection")
	}
}