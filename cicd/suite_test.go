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
	tenant string
)

var _ = BeforeSuite(func() {
	var err error
	db, err = support.SetupTestDB(GinkgoT())
	Expect(err).NotTo(HaveOccurred())
	ctx = context.Background()

	err = db.RunMigrations(ctx)
	Expect(err).NotTo(HaveOccurred())
})

var _ = AfterSuite(func() {
	db.Close()
})

func TestCICD(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "CICD Integration Test Suite")
}
