package snippet

import (
	"strings"
	"testing"
)

const testContextID = "e90a16c1-2178-4bb5-81f9-9c710d170b11"

func TestGenerate_CompactObfuscatedSnippet(t *testing.T) {
	js, err := Generate(testContextID, "http://track.example.com", nil)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	for _, want := range []string{
		`"http://track.example.com"`,
		`"/track/` + testContextID + `"`,
		`"pv"`,
		"text/plain",
		"document.title",
	} {
		if !strings.Contains(js, want) {
			t.Errorf("Generate() missing %q", want)
		}
	}
	for _, banned := range []string{
		"window.Automata",
		"contextId",
		"visitor_id",
		"pageTitle",
		"automata_visitor",
		"application/json",
	} {
		if strings.Contains(js, banned) {
			t.Errorf("Generate() must not contain %q", banned)
		}
	}
}

func TestGenerate_CookieNameStableAndPerContext(t *testing.T) {
	a1 := "11111111-1111-4111-8111-111111111111"
	a2 := "22222222-2222-4222-8222-222222222222"
	if cookieName(a1) != cookieName(a1) {
		t.Error("cookieName is not stable for the same context")
	}
	if cookieName(a1) == cookieName(a2) {
		t.Error("cookieName collides across contexts")
	}
	if !strings.HasPrefix(cookieName(a1), "a") {
		t.Errorf("cookieName = %q, want opaque short name with 'a' prefix", cookieName(a1))
	}
}

func TestGenerate_RequiresContextID(t *testing.T) {
	if _, err := Generate("", "http://track.example.com", nil); err == nil {
		t.Error("expected error for empty contextID")
	}
}

func TestGenerate_KeepsBatchOptions(t *testing.T) {
	js, err := Generate(testContextID, "http://track.example.com", map[string]interface{}{
		"batchSize":     float64(5),
		"batchInterval": float64(10),
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	for _, want := range []string{">=5", "10*1000"} {
		if !strings.Contains(js, want) {
			t.Errorf("Generate() missing batch option %q", want)
		}
	}
}