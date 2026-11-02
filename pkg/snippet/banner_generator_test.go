package snippet

import (
	"strings"
	"testing"
)

func TestGenerateBannerConfigSnippet(t *testing.T) {
	js, err := GenerateBannerConfigSnippet("ctx-123")
	if err != nil {
		t.Fatalf("GenerateBannerConfigSnippet() error = %v", err)
	}
	for _, want := range []string{
		`"ctx-123"`,
		`/banner/config/ctx-123`,
		`/banner/decision/ctx-123`,
		`_getCookie("automata_visitor")`,
		`ab_`,
		`SameSite=Lax`,
		`2147483000`,
		`fullpage`,
		`dismisses`,
		`"Content-Type":"text/plain"`,
		`location.href`,
	} {
		if !strings.Contains(js, want) {
			t.Errorf("GenerateBannerConfigSnippet() missing %q", want)
		}
	}
}

func TestGenerateBannerConfigSnippet_SkipsDecidedAndLegacy(t *testing.T) {
	js, err := GenerateBannerConfigSnippet("ctx-1")
	if err != nil {
		t.Fatal(err)
	}
	// Banners with an existing decision cookie are skipped.
	if !strings.Contains(js, `if(_getCookie("ab_"+b.id))return;`) {
		t.Errorf("snippet missing decided-banner skip")
	}
	// Banners without a position (legacy) are skipped.
	if !strings.Contains(js, `if(!b.position)return;`) {
		t.Errorf("snippet missing legacy-banner skip")
	}
	// Decisions are posted as text/plain to avoid a CORS preflight.
	if !strings.Contains(js, `body:JSON.stringify({banner_id:b.id,action:btn.action,visitor_id:visitorId})`) {
		t.Errorf("snippet missing decision POST body")
	}
}

func TestGenerateBannerConfigSnippetValidation(t *testing.T) {
	if _, err := GenerateBannerConfigSnippet(""); err == nil {
		t.Error("expected error for empty contextID")
	}
}
