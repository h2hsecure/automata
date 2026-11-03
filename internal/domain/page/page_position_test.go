package page_test

import (
	"strings"
	"testing"

	"github.com/h2hsecure/automata/internal/domain/page"
)

func TestValidatePositions(t *testing.T) {
	valid := []page.Position{"", "top", "bottom", "left", "right", "full"}
	for _, pos := range valid {
		p := &page.Page{Slug: "s", Title: "t", Position: pos}
		if err := p.Validate(); err != nil {
			t.Errorf("position %q should be valid, got %v", pos, err)
		}
	}
	invalid := []page.Position{"fullpage", "topp", "BOTTOM"}
	for _, pos := range invalid {
		p := &page.Page{Slug: "s", Title: "t", Position: pos}
		if err := p.Validate(); err == nil || !strings.Contains(err.Error(), "position") {
			t.Errorf("position %q should be invalid with 'position' in error, got %v", pos, err)
		}
	}
}

func TestPositionValid(t *testing.T) {
	if !page.Position("top").Valid() {
		t.Error("Position(\"top\").Valid() should be true")
	}
	if page.Position("fullpage").Valid() {
		t.Error("Position(\"fullpage\").Valid() should be false")
	}
}
