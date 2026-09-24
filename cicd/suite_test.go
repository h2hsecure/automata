package cicd_test

import (
	"context"

	"github.com/hekemen/automata/cicd/support"
	"github.com/jackc/pgx/v5/pgxpool"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"testing"
)

var (
	db     *support.TestDB
	ctx    context.Context
	pool   *pgxpool.Pool
	contextID string
)

var _ = BeforeSuite(func() {
	var err error
	db, err = support.SetupTestDB(GinkgoT())
	Expect(err).NotTo(HaveOccurred())
	ctx = context.Background()
	pool = db.Pool

	err = db.RunMigrations(ctx)
	Expect(err).NotTo(HaveOccurred())
})

var _ = AfterSuite(func() {
	db.Close()
})

var _ = AfterEach(func() {
	// Clean up contexts table between tests to avoid count mismatches
	if pool != nil {
		_, _ = pool.Exec(ctx, "DELETE FROM contexts")
	}
})

func TestCICD(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "CICD Integration Test Suite")
}
