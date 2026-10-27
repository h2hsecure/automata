package storage

import "testing"

func TestExtOf(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"resume.pdf", ".pdf"},
		{"noext", ""},
		{"archive.tar.gz", ".gz"},
		{".hidden", ""},
	}
	for _, c := range cases {
		if got := ExtOf(c.in); got != c.want {
			t.Fatalf("ExtOf(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}