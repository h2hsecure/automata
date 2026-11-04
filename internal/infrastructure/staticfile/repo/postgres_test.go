package repo

import (
	"testing"
	"time"

	"github.com/h2hsecure/automata/internal/domain/staticfile"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func newRepoTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := t.Context()
	container, err := postgres.Run(
		ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	if err != nil {
		t.Fatalf("start postgres: %v", err)
	}
	t.Cleanup(func() { _ = container.Terminate(ctx) })

	url, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("connection string: %v", err)
	}
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := RunMigrations(pool); err != nil {
		t.Fatalf("migrations: %v", err)
	}
	return pool
}

func seedStaticFiles(t *testing.T, pool *pgxpool.Pool, r staticfile.Repository, ctxID string) {
	t.Helper()
	ctx := t.Context()
	names := []string{"a.png", "b.png", "c.png", "d.png", "e.png"}
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i, n := range names {
		f := &staticfile.StaticFile{ContextID: ctxID, Name: n, Token: "token-" + n, Mime: "image/png", Size: int64(i + 1)}
		if err := r.Create(f); err != nil {
			t.Fatalf("create %s: %v", n, err)
		}
		// Deterministic distinct timestamps so ordering is stable.
		if _, err := pool.Exec(ctx,
			"UPDATE static_files SET created_at = $2 WHERE context_id = $1 AND name = $3",
			ctxID, base.Add(time.Duration(i)*time.Hour), n,
		); err != nil {
			t.Fatalf("set created_at for %s: %v", n, err)
		}
	}
}

// TestList_LimitOffset verifies pagination semantics: newest-first order,
// limit/offset slicing, default limit, max-limit clamp, negative offset.
func TestList_LimitOffset(t *testing.T) {
	pool := newRepoTestPool(t)
	r := New(pool)
	ctxID := "00000000-0000-0000-0000-000000000001"
	seedStaticFiles(t, pool, r, ctxID)

	// e.png has the newest created_at, so it must come first.
	got, err := r.List(ctxID, 2, 0)
	if err != nil {
		t.Fatalf("List(2,0) error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("List(2,0) returned %d rows, want 2", len(got))
	}
	if got[0].Name != "e.png" || got[1].Name != "d.png" {
		t.Errorf("List(2,0) = [%s, %s], want [e.png, d.png] (newest first)", got[0].Name, got[1].Name)
	}

	got, err = r.List(ctxID, 2, 2)
	if err != nil {
		t.Fatalf("List(2,2) error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("List(2,2) returned %d rows, want 2", len(got))
	}
	if got[0].Name != "c.png" || got[1].Name != "b.png" {
		t.Errorf("List(2,2) = [%s, %s], want [c.png, b.png]", got[0].Name, got[1].Name)
	}

	// Default limit (<= 0) returns everything up to the default of 100.
	got, err = r.List(ctxID, 0, 0)
	if err != nil {
		t.Fatalf("List(0,0) error: %v", err)
	}
	if len(got) != 5 {
		t.Fatalf("List(0,0) returned %d rows, want 5", len(got))
	}

	// Limit beyond the max is clamped; all 5 rows still returned.
	got, err = r.List(ctxID, 100000, 0)
	if err != nil {
		t.Fatalf("List(100000,0) error: %v", err)
	}
	if len(got) != 5 {
		t.Fatalf("List(100000,0) returned %d rows, want 5", len(got))
	}

	// Negative offset behaves like 0.
	got, err = r.List(ctxID, 2, -5)
	if err != nil {
		t.Fatalf("List(2,-5) error: %v", err)
	}
	if len(got) != 2 || got[0].Name != "e.png" {
		t.Errorf("List(2,-5) = %d rows, first=%s, want 2 rows starting at e.png", len(got), got[0].Name)
	}
}

// TestNormalizeListParams covers the clamp boundaries without a database.
func TestNormalizeListParams(t *testing.T) {
	cases := []struct {
		name               string
		limit, offset      int
		wantLimit, wantOff int
	}{
		{"zero limit defaults", 0, 0, listDefaultLimit, 0},
		{"negative limit defaults", -3, 0, listDefaultLimit, 0},
		{"limit capped at max", listMaxLimit + 1, 0, listMaxLimit, 0},
		{"limit at max passes through", listMaxLimit, 0, listMaxLimit, 0},
		{"negative offset clamped", 10, -1, 10, 0},
		{"positive offset passes through", 10, 42, 10, 42},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			l, o := NormalizeListParams(tc.limit, tc.offset)
			if l != tc.wantLimit || o != tc.wantOff {
				t.Errorf("NormalizeListParams(%d, %d) = (%d, %d), want (%d, %d)", tc.limit, tc.offset, l, o, tc.wantLimit, tc.wantOff)
			}
		})
	}
}