// Package storage provides file storage on the local filesystem, rooted at
// a configurable directory outside the repository. Files are stored under
// <root>/<contextID>/<random><ext> with a JSON metadata sidecar so original
// names and MIME types survive the random rename.
package storage

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// FileStore stores files under a root directory.
type FileStore struct {
	root string
}

// Meta is the per-file metadata written alongside each stored file.
type Meta struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

var (
	contextIDRe = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
	extRe       = regexp.MustCompile(`^[A-Za-z0-9]+$`)
	nameRe      = regexp.MustCompile(`^[A-Za-z0-9_-]+/[a-f0-9]{16,}\.[A-Za-z0-9]+$`)
)

// New creates a FileStore rooted at dir, creating it if needed.
func New(dir string) (*FileStore, error) {
	if dir == "" {
		return nil, fmt.Errorf("storage: root dir is required")
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("storage: create root: %w", err)
	}
	return &FileStore{root: dir}, nil
}

// Store writes data under a random name in contextID's directory and
// returns the stored name (<contextID>/<random><ext>). ext includes the
// leading dot.
func (s *FileStore) Store(contextID, ext string, data []byte) (string, error) {
	return s.store(contextID, ext, "", "", data)
}

// StoreMeta is like Store but also writes a metadata sidecar recording the
// original file name and MIME type.
func (s *FileStore) StoreMeta(contextID, ext, name, mime string, data []byte) (string, error) {
	return s.store(contextID, ext, name, mime, data)
}

func (s *FileStore) store(contextID, ext, name, mime string, data []byte) (string, error) {
	if !contextIDRe.MatchString(contextID) {
		return "", fmt.Errorf("storage: invalid context id %q", contextID)
	}
	random, err := randomHex(16)
	if err != nil {
		return "", fmt.Errorf("storage: generate name: %w", err)
	}
	ext, err = normalizeExt(ext)
	if err != nil {
		return "", err
	}
	stored := filepath.ToSlash(filepath.Join(contextID, random+ext))
	full, err := s.safePath(stored)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		return "", fmt.Errorf("storage: create dir: %w", err)
	}
	if err := os.WriteFile(full, data, 0o640); err != nil {
		return "", fmt.Errorf("storage: write file: %w", err)
	}
	if name != "" || mime != "" {
		meta, _ := json.Marshal(Meta{Name: name, Type: mime})
		if err := os.WriteFile(full+".meta", meta, 0o640); err != nil {
			return "", fmt.Errorf("storage: write meta: %w", err)
		}
	}
	return stored, nil
}

// normalizeExt validates an extension (single extension, no dots) and
// returns it with a leading dot.
func normalizeExt(ext string) (string, error) {
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	body := strings.TrimPrefix(ext, ".")
	if body == "" || !extRe.MatchString(body) {
		return "", fmt.Errorf("storage: invalid extension %q", ext)
	}
	return "." + body, nil
}

// Open opens a stored file by name. Traversal names are rejected.
func (s *FileStore) Open(name string) (*os.File, error) {
	full, err := s.safePath(name)
	if err != nil {
		return nil, err
	}
	return os.Open(full)
}

// Path returns the absolute filesystem path of a stored file, validating the
// name against traversal. Use with c.File / http.ServeFile.
func (s *FileStore) Path(name string) (string, error) {
	return s.safePath(name)
}

// Meta reads the metadata sidecar for a stored file.
func (s *FileStore) Meta(name string) (Meta, error) {
	full, err := s.safePath(name)
	if err != nil {
		return Meta{}, err
	}
	data, err := os.ReadFile(full + ".meta")
	if err != nil {
		return Meta{}, fmt.Errorf("storage: read meta: %w", err)
	}
	var m Meta
	if err := json.Unmarshal(data, &m); err != nil {
		return Meta{}, fmt.Errorf("storage: parse meta: %w", err)
	}
	return m, nil
}

// Remove deletes a stored file and its metadata sidecar.
func (s *FileStore) Remove(name string) error {
	full, err := s.safePath(name)
	if err != nil {
		return err
	}
	if err := os.Remove(full); err != nil {
		return fmt.Errorf("storage: remove file: %w", err)
	}
	_ = os.Remove(full + ".meta")
	return nil
}

// safePath validates name and returns its absolute path, refusing anything
// outside the root.
func (s *FileStore) safePath(name string) (string, error) {
	if !nameRe.MatchString(name) {
		return "", fmt.Errorf("storage: invalid name %q", name)
	}
	root, err := filepath.Abs(s.root)
	if err != nil {
		return "", fmt.Errorf("storage: resolve root: %w", err)
	}
	full := filepath.Join(root, filepath.FromSlash(name))
	if full != root && !strings.HasPrefix(full, root+string(filepath.Separator)) {
		return "", fmt.Errorf("storage: name escapes root: %q", name)
	}
	return full, nil
}

func randomHex(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}