package snippet

import (
	"strings"
	"testing"
)

func TestGeneratePageConfigSnippet(t *testing.T) {
	js, err := GeneratePageConfigSnippet("ctx-123")
	if err != nil {
		t.Fatalf("GeneratePageConfigSnippet() error = %v", err)
	}
	for _, want := range []string{
		`"ctx-123"`,
		`/page/config/ctx-123`,
		`/page/decision/ctx-123`,
		`_getCookie("automata_visitor")`,
		`ab_`,
		`SameSite=Lax`,
		`2147483000`,
		`"Content-Type":"text/plain"`,
		`location.href`,
	} {
		if !strings.Contains(js, want) {
			t.Errorf("GeneratePageConfigSnippet() missing %q", want)
		}
	}
	if strings.Contains(js, "/banner/") {
		t.Error("GeneratePageConfigSnippet() must not reference /banner/ endpoints")
	}
}

func TestGeneratePageConfigSnippet_PositionBranches(t *testing.T) {
	js, err := GeneratePageConfigSnippet("ctx-1")
	if err != nil {
		t.Fatal(err)
	}
	checks := []struct {
		pos  string
		want []string
	}{
		{"top", []string{`if(p==="top")return "position:fixed;top:0;left:0;right:0;z-index:2147483000;"`}},
		{"bottom", []string{`if(p==="bottom")return "position:fixed;bottom:0;left:0;right:0;z-index:2147483000;"`}},
		{"left", []string{`if(p==="left")return "position:fixed;left:0;top:0;bottom:0;width:320px;z-index:2147483000;"`}},
		{"right", []string{`if(p==="right")return "position:fixed;right:0;top:0;bottom:0;width:320px;z-index:2147483000;"`}},
		{"full", []string{`if(p==="full")return "position:fixed;top:0;left:0;right:0;bottom:0;z-index:2147483000;background:rgba(15,15,15,0.92);display:flex;align-items:center;justify-content:center;"`}},
	}
	for _, c := range checks {
		for _, want := range c.want {
			if !strings.Contains(js, want) {
				t.Errorf("snippet missing CSS branch for %s: %q", c.pos, want)
			}
		}
	}
}

func TestGeneratePageConfigSnippet_UnknownPositionSkipped(t *testing.T) {
	js, err := GeneratePageConfigSnippet("ctx-1")
	if err != nil {
		t.Fatal(err)
	}
	// positionCss returns "" (falsy) for anything not in the five positions.
	if !strings.Contains(js, `return"";`) {
		t.Error("snippet missing falsy return for unknown positions")
	}
	// renderPage skips falsy CSS (covers empty AND unknown positions).
	if !strings.Contains(js, `if(!positionCss(p.position))return;`) {
		t.Error("snippet missing unknown-position skip guard")
	}
	// Pages with an existing decision cookie are skipped.
	if !strings.Contains(js, `if(_getCookie("ab_"+p.id))return;`) {
		t.Error("snippet missing decided-page skip")
	}
	// Root element carries data-position per spec.
	if !strings.Contains(js, `data-position`) {
		t.Error("snippet missing data-position attribute")
	}
	// Wrapper renamed from data-automata-banner.
	if !strings.Contains(js, `data-automata-page`) {
		t.Error("snippet missing data-automata-page attribute")
	}
}

func TestGeneratePageConfigSnippet_NullSafeAndDecisionBody(t *testing.T) {
	js, err := GeneratePageConfigSnippet("ctx-1")
	if err != nil {
		t.Fatal(err)
	}
	// Null buttons are safe to iterate (spec: buttons may be null/absent).
	if !strings.Contains(js, `p.buttons||[]`) {
		t.Error("snippet missing null-safe buttons iteration")
	}
	// Decisions are posted as text/plain with the page decision body.
	if !strings.Contains(js, `body:JSON.stringify({page_id:p.id,decision:btn.action,visitor_id:visitorId})`) {
		t.Error("snippet missing decision POST body")
	}
	// Only dismiss actions remove the wrapper (full pages with link-only buttons stay).
	if !strings.Contains(js, `if(btn.action==="dismiss_accept"||btn.action==="dismiss_decline")`) {
		t.Error("snippet missing dismiss-action guard")
	}
	// Inlined page content.
	if !strings.Contains(js, `p.content+(p.page?p.page:"")`) {
		t.Error("snippet missing inlined page content")
	}
	// Navigates for hrefs starting with "/".
	if !strings.Contains(js, `btn.href&&btn.href.indexOf("/")==0`) {
		t.Error("snippet missing href navigation guard")
	}
}

func TestGeneratePageConfigSnippetValidation(t *testing.T) {
	if _, err := GeneratePageConfigSnippet(""); err == nil {
		t.Error("expected error for empty contextID")
	}
}
