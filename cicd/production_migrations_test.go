package cicd_test

import (
	"fmt"

	"github.com/hekemen/automata/cicd/support"
	banner_repo "github.com/hekemen/automata/internal/infrastructure/banner/repo"
	"github.com/hekemen/automata/internal/infrastructure/contact"
	"github.com/hekemen/automata/internal/infrastructure/database"
	email_repo "github.com/hekemen/automata/internal/infrastructure/email/repo"
	form_repo "github.com/hekemen/automata/internal/infrastructure/form/repo"
	settings_repo "github.com/hekemen/automata/internal/infrastructure/settings/repo"
	tracking_repo "github.com/hekemen/automata/internal/infrastructure/tracking/repo"
	webhook_repo "github.com/hekemen/automata/internal/infrastructure/webhook/repo"
	"github.com/hekemen/automata/internal/infrastructure/queue"
	"github.com/jackc/pgx/v5/pgxpool"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// runProductionMigrations runs the same migration sequence that
// cmd/automata/main.go runs at startup, in the same order. Keep this helper
// in sync with main.go: this spec is the guard against migrations that the
// cicd suite applies (via its directory list) but production startup never
// runs.
func runProductionMigrations(pool *pgxpool.Pool) error {
	if err := database.RunMigrations(pool); err != nil {
		return fmt.Errorf("core: %w", err)
	}
	if err := tracking_repo.RunMigrations(pool); err != nil {
		return fmt.Errorf("tracking: %w", err)
	}
	if err := form_repo.RunMigrations(pool); err != nil {
		return fmt.Errorf("form: %w", err)
	}
	if err := contact.RunMigrations(pool); err != nil {
		return fmt.Errorf("contact: %w", err)
	}
	if err := banner_repo.RunMigrations(pool); err != nil {
		return fmt.Errorf("banner: %w", err)
	}
	if err := email_repo.RunMigrations(pool); err != nil {
		return fmt.Errorf("email: %w", err)
	}
	if err := webhook_repo.RunMigrations(pool); err != nil {
		return fmt.Errorf("webhook: %w", err)
	}
	if err := queue.RunMigrations(pool); err != nil {
		return fmt.Errorf("queue: %w", err)
	}
	if err := database.RunUserContextMigrations(pool); err != nil {
		return fmt.Errorf("user-context: %w", err)
	}
	if err := settings_repo.RunMigrations(pool); err != nil {
		return fmt.Errorf("settings: %w", err)
	}
	return nil
}

var _ = Describe("Production startup migrations", func() {
	It("creates the delivery queue tables (email_jobs, webhook_deliveries)", func() {
		fresh, err := support.SetupTestDB(GinkgoT())
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(fresh.Close)

		Expect(runProductionMigrations(fresh.Pool)).To(Succeed())

		for _, table := range []string{"email_jobs", "webhook_deliveries", "global_settings"} {
			var count int
			err := fresh.Pool.QueryRow(ctx,
				"SELECT COUNT(*) FROM information_schema.tables WHERE table_name = $1", table).Scan(&count)
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(1), "table %s missing after production migration sequence", table)
		}
	})
})