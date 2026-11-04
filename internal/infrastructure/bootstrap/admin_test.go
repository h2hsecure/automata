package bootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/h2hsecure/automata/internal/infrastructure/context/repo"
	"github.com/h2hsecure/automata/internal/infrastructure/database"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// TestWriteCredentialsFile_ModeAndContent verifies the secret file is
// owner-only and carries both credentials.
func TestWriteCredentialsFile_ModeAndContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "creds")
	if err := writeCredentialsFile(path, "admin@example.com", "s3cret-pw"); err != nil {
		t.Fatalf("writeCredentialsFile() error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if !strings.Contains(string(data), "email=admin@example.com") ||
		!strings.Contains(string(data), "password=s3cret-pw") {
		t.Errorf("file content = %q, want email+password lines", data)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Errorf("file mode = %o, want 0600", perm)
	}
}

// newTestPool spins up a disposable Postgres with all migrations applied.
func newTestPool(t *testing.T) *pgxpool.Pool {
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
	if err := database.RunMigrations(pool); err != nil {
		t.Fatalf("migrations: %v", err)
	}
	if err := database.RunUserContextMigrations(pool); err != nil {
		t.Fatalf("user-context migrations: %v", err)
	}
	return pool
}

// stdoutGuard redirects os.Stdout and, at test end, fails the test if
// anything was written to it (credentials must never reach stdout).
type stdoutGuard struct {
	r *os.File
}

func (g *stdoutGuard) install(t *testing.T) {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	g.r = r
	t.Cleanup(func() {
		os.Stdout = old
		_ = w.Close()
		if data := readAllBytes(g.r); len(data) != 0 {
			t.Errorf("stdout was written during test: %q", data)
		}
	})
}

func readAllBytes(r *os.File) []byte {
	var out []byte
	buf := make([]byte, 4096)
	for {
		n, err := r.Read(buf)
		out = append(out, buf[:n]...)
		if err != nil {
			return out // EOF, closed pipe, or read error: stop
		}
	}
}

// TestRun_GeneratedPasswordGoesToSecretFileNotStdout verifies that a
// generated bootstrap password lands in the 0600 secret file and never in
// stdout.
func TestRun_GeneratedPasswordGoesToSecretFileNotStdout(t *testing.T) {
	pool := newTestPool(t)
	t.Chdir(t.TempDir()) // keep the .admin_initialized marker out of the repo

	secret := filepath.Join(t.TempDir(), "admin-creds")
	t.Setenv("AUTOMATA_ADMIN_SECRET_FILE", secret)
	t.Setenv("AUTOMATA_ADMIN_EMAIL", "boot@example.com")
	t.Setenv("AUTOMATA_ADMIN_PASSWORD", "")

	g := &stdoutGuard{}
	g.install(t)

	if err := Run(pool, repo.NewUserPostgresRepo(pool)); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	data, err := os.ReadFile(secret)
	if err != nil {
		t.Fatalf("secret file not written: %v", err)
	}
	if !strings.Contains(string(data), "email=boot@example.com") {
		t.Errorf("secret file missing email line: %q", data)
	}
	if !strings.Contains(string(data), "password=") {
		t.Errorf("secret file missing password line: %q", data)
	}

	info, err := os.Stat(secret)
	if err != nil {
		t.Fatalf("stat secret: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Errorf("secret mode = %o, want 0600", perm)
	}
}

// TestRun_ProvidedPasswordWritesNoSecretFile verifies that an operator
// supplied password produces no secret file (the operator already knows it)
// and no stdout output.
func TestRun_ProvidedPasswordWritesNoSecretFile(t *testing.T) {
	pool := newTestPool(t)
	wd := t.TempDir()
	t.Chdir(wd)

	secret := filepath.Join(wd, "admin-creds")
	t.Setenv("AUTOMATA_ADMIN_SECRET_FILE", secret)
	t.Setenv("AUTOMATA_ADMIN_PASSWORD", "operator-known-pw")

	g := &stdoutGuard{}
	g.install(t)

	if err := Run(pool, repo.NewUserPostgresRepo(pool)); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if _, err := os.Stat(secret); !os.IsNotExist(err) {
		t.Errorf("secret file must not exist for operator-supplied password, err=%v", err)
	}
}