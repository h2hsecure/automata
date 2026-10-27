package storage

import (
	"path"
	"strings"
)

// ExtOf returns the extension of filename (including the leading dot),
// or "" when there is none. Leading-dot files like ".hidden" have no
// extension.
func ExtOf(filename string) string {
	base := path.Base(filename)
	idx := strings.LastIndex(base, ".")
	if idx <= 0 || idx == len(base)-1 {
		return ""
	}
	return base[idx:]
}

// Accepts reports whether a file matches any entry of accept. Each entry
// is a MIME type ("application/pdf"), a MIME wildcard ("image/*"), or an
// extension (".pdf" or "pdf"). An empty accept list accepts everything.
func Accepts(filename, mime string, accept []string) bool {
	if len(accept) == 0 {
		return true
	}
	ext := ExtOf(filename)
	for _, raw := range accept {
		p := strings.TrimSpace(raw)
		if p == "" {
			continue
		}
		if p == "*" {
			return true
		}
		if strings.Contains(p, "/") {
			// MIME pattern: exact type or "type/*" wildcard.
			if strings.HasSuffix(p, "/*") {
				if strings.HasPrefix(mime, strings.TrimSuffix(p, "*")) {
					return true
				}
			} else if strings.EqualFold(mime, p) {
				return true
			}
			continue
		}
		// Extension pattern, with or without a leading dot.
		want := p
		if !strings.HasPrefix(want, ".") {
			want = "." + want
		}
		if strings.EqualFold(ext, want) {
			return true
		}
	}
	return false
}