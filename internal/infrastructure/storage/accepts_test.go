package storage

import "testing"

func TestAccepts_EmptyAcceptAllowsEverything(t *testing.T) {
	if !Accepts("anything.xyz", "", nil) {
		t.Fatal("empty accept should allow everything")
	}
	if !Accepts("a.txt", "text/plain", []string{}) {
		t.Fatal("empty accept slice should allow everything")
	}
}

func TestAccepts_ExactMime(t *testing.T) {
	if !Accepts("resume.pdf", "application/pdf", []string{"application/pdf"}) {
		t.Fatal("expected application/pdf to match")
	}
	if Accepts("resume.docx", "application/vnd.openxmlformats-officedocument.wordprocessingml.document", []string{"application/pdf"}) {
		t.Fatal("expected non-matching mime to be rejected")
	}
}

func TestAccepts_MimeTypeWildcard(t *testing.T) {
	accept := []string{"image/*"}
	if !Accepts("logo.png", "image/png", accept) {
		t.Fatal("image/* should match image/png")
	}
	if !Accepts("logo.jpeg", "image/jpeg", accept) {
		t.Fatal("image/* should match image/jpeg")
	}
	if Accepts("doc.pdf", "application/pdf", accept) {
		t.Fatal("image/* should not match application/pdf")
	}
}

func TestAccepts_Extension(t *testing.T) {
	accept := []string{".pdf"}
	if !Accepts("resume.pdf", "", accept) {
		t.Fatal("extension .pdf should match resume.pdf even without mime")
	}
	if Accepts("resume.txt", "text/plain", accept) {
		t.Fatal("extension .pdf should not match resume.txt")
	}
}

func TestAccepts_ExtensionWithoutDot(t *testing.T) {
	accept := []string{"pdf"}
	if !Accepts("resume.pdf", "application/pdf", accept) {
		t.Fatal("extension pdf (no dot) should match resume.pdf")
	}
}

func TestAccepts_MultipleOptions(t *testing.T) {
	accept := []string{"image/png", ".pdf"}
	if !Accepts("a.png", "image/png", accept) {
		t.Fatal("should match first option")
	}
	if !Accepts("b.pdf", "application/pdf", accept) {
		t.Fatal("should match second option")
	}
	if Accepts("c.docx", "application/vnd.openxmlformats-officedocument.wordprocessingml.document", accept) {
		t.Fatal("should reject unmatched file")
	}
}