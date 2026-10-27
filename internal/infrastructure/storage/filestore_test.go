package storage

import (
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
)

func newTestStore(t *testing.T) *FileStore {
	t.Helper()
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return s
}

func TestStore_WritesFileAndReturnsStableName(t *testing.T) {
	s := newTestStore(t)

	name, err := s.Store("ctx-1", ".pdf", []byte("hello"))
	if err != nil {
		t.Fatalf("Store: %v", err)
	}

	// Name must be <contextID>/<random><ext>.
	dir, file := path.Split(name)
	if dir != "ctx-1/" {
		t.Fatalf("expected dir ctx-1/, got %q", dir)
	}
	if len(file) <= 4+4 { // <uuid hex 16+> + ext
		t.Fatalf("expected uuid-prefixed file, got %q", file)
	}

	data, err := os.ReadFile(filepath.Join(s.root, name))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != "hello" {
		t.Fatalf("expected hello, got %q", data)
	}
}

func TestOpen_ReadsBackStoredContent(t *testing.T) {
	s := newTestStore(t)
	name, err := s.Store("ctx-1", ".txt", []byte("data"))
	if err != nil {
		t.Fatalf("Store: %v", err)
	}

	f, err := s.Open(name)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer f.Close()

	buf := make([]byte, 4)
	if _, err := f.Read(buf); err != nil {
		t.Fatalf("Read: %v", err)
	}
	if string(buf) != "data" {
		t.Fatalf("expected data, got %q", buf)
	}
}

func TestMeta_ReturnsOriginalNameAndType(t *testing.T) {
	s := newTestStore(t)
	name, err := s.StoreMeta("ctx-1", ".pdf", "resume.pdf", "application/pdf", []byte("x"))
	if err != nil {
		t.Fatalf("StoreMeta: %v", err)
	}

	meta, err := s.Meta(name)
	if err != nil {
		t.Fatalf("Meta: %v", err)
	}
	if meta.Name != "resume.pdf" || meta.Type != "application/pdf" {
		t.Fatalf("unexpected meta: %+v", meta)
	}
}

func TestRemove_DeletesFile(t *testing.T) {
	s := newTestStore(t)
	name, err := s.Store("ctx-1", ".png", []byte("img"))
	if err != nil {
		t.Fatalf("Store: %v", err)
	}
	if err := s.Remove(name); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, err := s.Open(name); err == nil {
		t.Fatal("expected Open to fail after Remove")
	}
}

func TestStore_RejectsPathTraversalInContextID(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.Store("../etc", ".txt", []byte("x")); err == nil {
		t.Fatal("expected error for traversal contextID")
	}
	if _, err := s.Store("a/b", ".txt", []byte("x")); err == nil {
		t.Fatal("expected error for nested contextID")
	}
	if _, err := s.Store("", ".txt", []byte("x")); err == nil {
		t.Fatal("expected error for empty contextID")
	}
}

func TestStore_RejectsUnsafeExtension(t *testing.T) {
	s := newTestStore(t)
	for _, ext := range []string{"", ".a/b", "../x", ".a.b.c", "pdf.txt"} {
		if _, err := s.Store("ctx-1", ext, []byte("x")); err == nil {
			t.Fatalf("expected error for extension %q", ext)
		}
	}
}

func TestOpen_RejectsTraversalName(t *testing.T) {
	s := newTestStore(t)
	for _, name := range []string{"../../etc/passwd", "ctx-1/..%2fetc", "ctx-1/a/b.txt", "ctx-1", ""} {
		if _, err := s.Open(name); err == nil {
			t.Fatalf("expected error for name %q", name)
		}
	}
}

func TestStore_UniqueNamesForSameContext(t *testing.T) {
	s := newTestStore(t)
	a, err := s.Store("ctx-1", ".txt", []byte("a"))
	if err != nil {
		t.Fatalf("Store: %v", err)
	}
	b, err := s.Store("ctx-1", ".txt", []byte("b"))
	if err != nil {
		t.Fatalf("Store: %v", err)
	}
	if a == b {
		t.Fatalf("expected unique names, both %q", a)
	}
}

func TestPath_ReturnsAbsolutePathWithinRoot(t *testing.T) {
	s := newTestStore(t)
	name, err := s.Store("ctx-1", ".txt", []byte("data"))
	if err != nil {
		t.Fatalf("Store: %v", err)
	}

	p, err := s.Path(name)
	if err != nil {
		t.Fatalf("Path: %v", err)
	}
	root, _ := filepath.Abs(s.root)
	if !strings.HasPrefix(p, root+string(filepath.Separator)) {
		t.Fatalf("expected path under root %s, got %s", root, p)
	}
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("stat resolved path: %v", err)
	}
}

func TestPath_RejectsTraversalName(t *testing.T) {
	s := newTestStore(t)
	for _, name := range []string{"../../etc/passwd", "ctx-1/a/b.txt", ""} {
		if _, err := s.Path(name); err == nil {
			t.Fatalf("expected error for name %q", name)
		}
	}
}