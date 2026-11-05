package contact_test

import (
	"context"
	"testing"
	"time"

	"github.com/h2hsecure/automata/internal/infrastructure/contact"
	"github.com/jackc/pgx/v5/pgxpool"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

var _ = Describe("Contact Migration", func() {
	var (
		ctx    context.Context
		pool   *pgxpool.Pool
		cancel context.CancelFunc
	)

	BeforeEach(func() {
		ctx = context.Background()
		var container *postgres.PostgresContainer
		var err error
		container, err = postgres.Run(
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
		Expect(err).NotTo(HaveOccurred())

		url, err := container.ConnectionString(ctx, "sslmode=disable")
		Expect(err).NotTo(HaveOccurred())

		pool, err = pgxpool.New(ctx, url)
		Expect(err).NotTo(HaveOccurred())
		Expect(pool.Ping(ctx)).To(Succeed())

		cancel = func() {
			_ = container.Terminate(ctx)
			pool.Close()
		}
	})

	AfterEach(func() {
		if cancel != nil {
			cancel()
		}
	})

	Describe("RunMigrations", func() {
		It("creates all tables", func() {
			err := contact.RunMigrations(pool)
			Expect(err).NotTo(HaveOccurred())

			var count int
			err = pool.QueryRow(ctx, `
				SELECT COUNT(*) FROM information_schema.tables
				WHERE table_name IN ('contacts', 'contact_tags', 'contact_tag_memberships', 'contact_field_definitions')
			`).Scan(&count)
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(4))
		})

		It("creates all indexes", func() {
			err := contact.RunMigrations(pool)
			Expect(err).NotTo(HaveOccurred())

			var count int
			err = pool.QueryRow(ctx, `
				SELECT COUNT(*) FROM pg_indexes
				WHERE tablename IN ('contacts', 'contact_tags', 'contact_tag_memberships', 'contact_field_definitions')
			`).Scan(&count)
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(BeNumerically(">", 0))
		})

		It("handles idempotent re-runs", func() {
			err := contact.RunMigrations(pool)
			Expect(err).NotTo(HaveOccurred())

			err = contact.RunMigrations(pool)
			Expect(err).NotTo(HaveOccurred())
		})

		It("dedupes visitor contacts and enforces a unique index", func() {
			err := contact.RunMigrations(pool)
			Expect(err).NotTo(HaveOccurred())

			// Simulate a historical database: drop the unique index so
			// duplicate visitor contacts can exist before the dedupe runs.
			_, err = pool.Exec(ctx, "DROP INDEX IF EXISTS idx_contacts_visitor_unique")
			Expect(err).NotTo(HaveOccurred())

			var contextID string
			err = pool.QueryRow(ctx, "SELECT gen_random_uuid()::text").Scan(&contextID)
			Expect(err).NotTo(HaveOccurred())

			// Seed three duplicate visitor contacts (oldest first) plus
			// dependent rows pointing at the two younger duplicates.
			var oldest, mid, young string
			for _, ts := range []string{
				"NOW() - interval '3 days'",
				"NOW() - interval '2 days'",
				"NOW() - interval '1 day'",
			} {
				_, err = pool.Exec(ctx, `
					INSERT INTO contacts (context_id, source, source_id, hidden, created_at, updated_at)
					VALUES ($1, 'visitor', 'visitor-abc', TRUE, `+ts+`, `+ts+`)
				`, contextID)
				Expect(err).NotTo(HaveOccurred())
			}
			rows, err := pool.Query(ctx,
				"SELECT id::text FROM contacts WHERE context_id=$1 AND source_id='visitor-abc' ORDER BY created_at, id", contextID)
			Expect(err).NotTo(HaveOccurred())
			defer rows.Close()
			for _, id := range []any{&oldest, &mid, &young} {
				Expect(rows.Next()).To(BeTrue())
				Expect(rows.Scan(id)).To(Succeed())
			}
			Expect(rows.Err()).ToNot(HaveOccurred())

			_, err = pool.Exec(ctx, `
				INSERT INTO contact_activities (contact_id, context_id, type)
				VALUES ($1, $2, 'page_visit'), ($3, $2, 'page_visit')
			`, mid, contextID, young)
			Expect(err).NotTo(HaveOccurred())

			var tagID string
			err = pool.QueryRow(ctx, `
				INSERT INTO contact_tags (context_id, name) VALUES ($1, 'vip') RETURNING id::text
			`, contextID).Scan(&tagID)
			Expect(err).NotTo(HaveOccurred())
			_, err = pool.Exec(ctx, `
				INSERT INTO contact_tag_memberships (contact_id, tag_id) VALUES ($1, $2)
			`, young, tagID)
			Expect(err).NotTo(HaveOccurred())

			_, err = pool.Exec(ctx, `
				INSERT INTO contact_cookie_mappings (contact_id, context_id, cookie, source)
				VALUES ($1, $2, 'cookie-1', 'web')
			`, mid, contextID)
			Expect(err).NotTo(HaveOccurred())

			// Minimal stand-in for tracking_visitors (owned by the tracking
			// package's migration in production) to exercise the VARCHAR
			// contact_id reattach path.
			_, err = pool.Exec(ctx, `
				CREATE TABLE tracking_visitors (
					id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
					context_id UUID NOT NULL,
					cookie_value TEXT,
					contact_id VARCHAR(128)
				)
			`)
			Expect(err).NotTo(HaveOccurred())
			_, err = pool.Exec(ctx, `
				INSERT INTO tracking_visitors (context_id, cookie_value, contact_id)
				VALUES ($1, 'visitor-abc', $2)
			`, contextID, mid)
			Expect(err).NotTo(HaveOccurred())

			// Re-run the migration: it must dedupe.
			err = contact.RunMigrations(pool)
			Expect(err).NotTo(HaveOccurred())

			var remaining int
			err = pool.QueryRow(ctx, `
				SELECT COUNT(*) FROM contacts
				WHERE context_id = $1 AND source = 'visitor' AND source_id = 'visitor-abc'
			`, contextID).Scan(&remaining)
			Expect(err).NotTo(HaveOccurred())
			Expect(remaining).To(Equal(1))

			var keptID string
			err = pool.QueryRow(ctx, `
				SELECT id::text FROM contacts
				WHERE context_id = $1 AND source = 'visitor' AND source_id = 'visitor-abc'
			`, contextID).Scan(&keptID)
			Expect(err).NotTo(HaveOccurred())
			Expect(keptID).To(Equal(oldest), "oldest duplicate must survive")

			// Dependents must be reattached to the kept contact.
			var depCount int
			err = pool.QueryRow(ctx, `
				SELECT COUNT(*) FROM contact_activities
				WHERE context_id = $1 AND contact_id <> $2
			`, contextID, keptID).Scan(&depCount)
			Expect(err).NotTo(HaveOccurred())
			Expect(depCount).To(Equal(0), "all activities must point at the kept contact")

			err = pool.QueryRow(ctx, `
				SELECT COUNT(*) FROM contact_tag_memberships WHERE tag_id = $1 AND contact_id <> $2
			`, tagID, keptID).Scan(&depCount)
			Expect(err).NotTo(HaveOccurred())
			Expect(depCount).To(Equal(0), "tag memberships must point at the kept contact")

			err = pool.QueryRow(ctx, `
				SELECT COUNT(*) FROM contact_cookie_mappings WHERE context_id = $1 AND contact_id <> $2
			`, contextID, keptID).Scan(&depCount)
			Expect(err).NotTo(HaveOccurred())
			Expect(depCount).To(Equal(0), "cookie mappings must point at the kept contact")

			err = pool.QueryRow(ctx, `
				SELECT COUNT(*) FROM tracking_visitors WHERE context_id = $1 AND contact_id <> $2
			`, contextID, keptID).Scan(&depCount)
			Expect(err).NotTo(HaveOccurred())
			Expect(depCount).To(Equal(0), "tracking_visitors must point at the kept contact")

			// The unique index must now reject a second insert.
			_, err = pool.Exec(ctx, `
				INSERT INTO contacts (context_id, source, source_id, hidden)
				VALUES ($1, 'visitor', 'visitor-abc', TRUE)
			`, contextID)
			Expect(err).To(HaveOccurred())

			// Migration must stay idempotent after the dedupe.
			err = contact.RunMigrations(pool)
			Expect(err).NotTo(HaveOccurred())
		})

		It("dedupe does not fail on a PK collision when the kept contact already has the same tag", func() {
			err := contact.RunMigrations(pool)
			Expect(err).NotTo(HaveOccurred())

			// Simulate a historical database: drop the unique index so
			// duplicate visitor contacts can exist before the dedupe runs.
			_, err = pool.Exec(ctx, "DROP INDEX IF EXISTS idx_contacts_visitor_unique")
			Expect(err).NotTo(HaveOccurred())

			var contextID string
			err = pool.QueryRow(ctx, "SELECT gen_random_uuid()::text").Scan(&contextID)
			Expect(err).NotTo(HaveOccurred())

			var oldest, mid, young string
			for _, ts := range []string{
				"NOW() - interval '3 days'",
				"NOW() - interval '2 days'",
				"NOW() - interval '1 day'",
			} {
				_, err = pool.Exec(ctx, `
					INSERT INTO contacts (context_id, source, source_id, hidden, created_at, updated_at)
					VALUES ($1, 'visitor', 'visitor-pk', TRUE, `+ts+`, `+ts+`)
				`, contextID)
				Expect(err).NotTo(HaveOccurred())
			}
			rows, err := pool.Query(ctx,
				"SELECT id::text FROM contacts WHERE context_id=$1 AND source_id='visitor-pk' ORDER BY created_at, id", contextID)
			Expect(err).NotTo(HaveOccurred())
			defer rows.Close()
			for _, id := range []any{&oldest, &mid, &young} {
				Expect(rows.Next()).To(BeTrue())
				Expect(rows.Scan(id)).To(Succeed())
			}
			Expect(rows.Err()).ToNot(HaveOccurred())

			// Both the kept contact and a duplicate carry the same tag:
			// the naive reattach UPDATE would violate PK (contact_id, tag_id).
			var tagID string
			err = pool.QueryRow(ctx, `
				INSERT INTO contact_tags (context_id, name) VALUES ($1, 'vip') RETURNING id::text
			`, contextID).Scan(&tagID)
			Expect(err).NotTo(HaveOccurred())
			_, err = pool.Exec(ctx, `
				INSERT INTO contact_tag_memberships (contact_id, tag_id) VALUES ($1, $2), ($3, $2)
			`, oldest, tagID, young)
			Expect(err).NotTo(HaveOccurred())

			// (A cookie-mapping collision is unseedable: the PK
			// (context_id, cookie) means the kept contact and a duplicate
			// can never both hold the same (context, cookie) row, so no
			// guard is needed on contact_cookie_mappings.)

			// Re-run the migration: the PK collision must not fail the
			// migration set. Conflicting dependent rows are already on the
			// kept contact and are dropped with the duplicate.
			err = contact.RunMigrations(pool)
			Expect(err).NotTo(HaveOccurred())

			var keptID string
			err = pool.QueryRow(ctx, `
				SELECT id::text FROM contacts
				WHERE context_id = $1 AND source = 'visitor' AND source_id = 'visitor-pk'
			`, contextID).Scan(&keptID)
			Expect(err).NotTo(HaveOccurred())
			Expect(keptID).To(Equal(oldest), "oldest duplicate must survive")

			var tagCount int
			err = pool.QueryRow(ctx, `
				SELECT COUNT(*) FROM contact_tag_memberships WHERE tag_id = $1 AND contact_id = $2
			`, tagID, keptID).Scan(&tagCount)
			Expect(err).NotTo(HaveOccurred())
			Expect(tagCount).To(Equal(1), "kept contact keeps a single membership for the tag")
		})

		It("creates unique constraint on context+email", func() {
			err := contact.RunMigrations(pool)
			Expect(err).NotTo(HaveOccurred())

			// Generate a single context_id to use for both inserts
			var contextID string
			err = pool.QueryRow(ctx, "SELECT gen_random_uuid()::text").Scan(&contextID)
			Expect(err).NotTo(HaveOccurred())

			// Insert first contact with email
			_, err = pool.Exec(ctx, `
				INSERT INTO contacts (context_id, email, first_name)
				VALUES ($1, 'test@example.com', 'Test')
			`, contextID)
			Expect(err).NotTo(HaveOccurred())

			// Try to insert duplicate email in same context — should fail
			_, err = pool.Exec(ctx, `
				INSERT INTO contacts (context_id, email, first_name)
				VALUES ($1, 'test@example.com', 'Duplicate')
			`, contextID)
			Expect(err).To(HaveOccurred())
		})
	})
})

func TestContactMigration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Contact Migration Suite")
}
