package main

import (
	"fmt"

	"github.com/h2hsecure/automata/internal/domain/form"
	"github.com/h2hsecure/automata/internal/infrastructure/storage"
)

// uploadFileResolver resolves an upload token (stored file name) into a
// FileUpload record for form submissions.
type uploadFileResolver struct {
	store *storage.FileStore
}

func (r uploadFileResolver) Resolve(contextID, token string) (form.FileUpload, error) {
	if r.store == nil {
		return form.FileUpload{}, fmt.Errorf("file uploads are not configured")
	}
	f, err := r.store.Open(token)
	if err != nil {
		return form.FileUpload{}, fmt.Errorf("open upload: %w", err)
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return form.FileUpload{}, fmt.Errorf("stat upload: %w", err)
	}
	meta, err := r.store.Meta(token)
	if err != nil {
		// Meta sidecar is optional: fall back to empty name/type.
		meta = storage.Meta{}
	}
	return form.FileUpload{
		Name: meta.Name,
		Size: st.Size(),
		Type: meta.Type,
		URL:  token,
	}, nil
}