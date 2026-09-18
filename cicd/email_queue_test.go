package cicd_test

import (
	"context"
	"fmt"
	"time"

	"github.com/hekemen/automata/cicd/support"
	"github.com/hekemen/automata/internal/infrastructure/queue"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var (
	mockSMTP *support.MockSMTP
)

var _ = BeforeEach(func() {
	mockSMTP = support.NewMockSMTP()
	mockSMTP.Start()
	pool = db.Pool
})

var _ = Describe("Email Queue Integration Tests", func() {


	AfterEach(func() {
		// Clean up email_jobs table
		_, err := pool.Exec(ctx, "DELETE FROM email_jobs")
		Expect(err).NotTo(HaveOccurred())
		mockSMTP.Clear()
	})

	Describe("Email job enqueueing", func() {
		It("should enqueue an email job with pending status", func() {
			tenantID := support.NewTestTenantID()
			jobData := support.NewTestEmailJob(tenantID)

			toStr := fmt.Sprintf("{%s}", jobData["to_addresses"].([]string)[0])

			query := `
				INSERT INTO email_jobs (id, tenant_id, to_addresses, subject, body, html_body, attempts, max_retries, next_retry, created_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
			`

			_, err := pool.Exec(ctx, query,
				jobData["id"],
				jobData["tenant_id"],
				toStr,
				jobData["subject"],
				jobData["body"],
				jobData["html_body"],
				jobData["attempts"],
				jobData["max_retries"],
				jobData["next_retry"],
				jobData["created_at"],
			)
			Expect(err).NotTo(HaveOccurred())

			// Verify job was inserted
			var count int
			err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM email_jobs").Scan(&count)
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(1))
		})

		It("should enqueue email with multiple recipients", func() {
			tenantID := support.NewTestTenantID()
			jobData := support.NewTestEmailJob(tenantID)
			jobData["to_addresses"] = []string{"user1@example.com", "user2@example.com", "user3@example.com"}

			toStr := "{user1@example.com,user2@example.com,user3@example.com}"

			query := `
				INSERT INTO email_jobs (id, tenant_id, to_addresses, subject, body, html_body, attempts, max_retries, next_retry, created_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
			`

			_, err := pool.Exec(ctx, query,
				jobData["id"],
				jobData["tenant_id"],
				toStr,
				jobData["subject"],
				jobData["body"],
				jobData["html_body"],
				jobData["attempts"],
				jobData["max_retries"],
				jobData["next_retry"],
				jobData["created_at"],
			)
			Expect(err).NotTo(HaveOccurred())

			// Verify job was inserted
			var count int
			err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM email_jobs").Scan(&count)
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(1))
		})
	})

	Describe("Email worker processing", func() {
		It("should process pending email and mark as sent", func() {
			tenantID := support.NewTestTenantID()
			jobData := support.NewTestEmailJob(tenantID)

			toStr := fmt.Sprintf("{%s}", jobData["to_addresses"].([]string)[0])

			// Insert job
			query := `
				INSERT INTO email_jobs (id, tenant_id, to_addresses, subject, body, html_body, attempts, max_retries, next_retry, created_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
			`

			_, err := pool.Exec(ctx, query,
				jobData["id"],
				jobData["tenant_id"],
				toStr,
				jobData["subject"],
				jobData["body"],
				jobData["html_body"],
				jobData["attempts"],
				jobData["max_retries"],
				jobData["next_retry"],
				jobData["created_at"],
			)
			Expect(err).NotTo(HaveOccurred())

			// Create queue and process
			emailQueue := queue.NewEmailQueue(pool)
			workerCtx, workerCancel := context.WithCancel(context.Background())
			defer workerCancel()
			emailQueue.StartWorker(workerCtx, mockSMTP)

			// Wait for processing
			time.Sleep(500 * time.Millisecond)
			workerCancel()

			// Verify job was processed (deleted from queue)
			var count int
			err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM email_jobs").Scan(&count)
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(0))

			// Verify email was captured by mock SMTP
			Expect(mockSMTP.Count()).To(Equal(1))
			Expect(mockSMTP.HasEmailTo("recipient@example.com")).To(BeTrue())
		})

		It("should retry failed emails up to max_attempts", func() {
			tenantID := support.NewTestTenantID()
			jobData := support.NewTestEmailJob(tenantID)
			jobData["attempts"] = 2
			jobData["max_retries"] = 3

			toStr := fmt.Sprintf("{%s}", jobData["to_addresses"].([]string)[0])

			// Insert job with 2 attempts
			query := `
				INSERT INTO email_jobs (id, tenant_id, to_addresses, subject, body, html_body, attempts, max_retries, next_retry, created_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
			`

			_, err := pool.Exec(ctx, query,
				jobData["id"],
				jobData["tenant_id"],
				toStr,
				jobData["subject"],
				jobData["body"],
				jobData["html_body"],
				jobData["attempts"],
				jobData["max_retries"],
				jobData["next_retry"],
				jobData["created_at"],
			)
			Expect(err).NotTo(HaveOccurred())

			// Verify job is in queue
			var count int
			err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM email_jobs").Scan(&count)
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(1))
		})

		It("should mark email as permanently failed after max_attempts exceeded", func() {
			tenantID := support.NewTestTenantID()
			jobData := support.NewTestEmailJob(tenantID)
			jobData["attempts"] = 3
			jobData["max_retries"] = 3

			toStr := fmt.Sprintf("{%s}", jobData["to_addresses"].([]string)[0])

			// Insert job with max attempts
			query := `
				INSERT INTO email_jobs (id, tenant_id, to_addresses, subject, body, html_body, attempts, max_retries, next_retry, created_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
			`

			_, err := pool.Exec(ctx, query,
				jobData["id"],
				jobData["tenant_id"],
				toStr,
				jobData["subject"],
				jobData["body"],
				jobData["html_body"],
				jobData["attempts"],
				jobData["max_retries"],
				jobData["next_retry"],
				jobData["created_at"],
			)
			Expect(err).NotTo(HaveOccurred())

			// Job should be in queue but will be removed when processed
			var count int
			err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM email_jobs").Scan(&count)
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(1))
		})
	})

	Describe("Mock SMTP capture", func() {
		It("should capture sent emails in mock SMTP", func() {
			tenantID := support.NewTestTenantID()
			jobData := support.NewTestEmailJob(tenantID)

			toStr := fmt.Sprintf("{%s}", jobData["to_addresses"].([]string)[0])

			// Insert job
			query := `
				INSERT INTO email_jobs (id, tenant_id, to_addresses, subject, body, html_body, attempts, max_retries, next_retry, created_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
			`

			_, err := pool.Exec(ctx, query,
				jobData["id"],
				jobData["tenant_id"],
				toStr,
				jobData["subject"],
				jobData["body"],
				jobData["html_body"],
				jobData["attempts"],
				jobData["max_retries"],
				jobData["next_retry"],
				jobData["created_at"],
			)
			Expect(err).NotTo(HaveOccurred())

			// Create queue and process
			emailQueue := queue.NewEmailQueue(pool)
			workerCtx, workerCancel := context.WithCancel(context.Background())
			defer workerCancel()
			emailQueue.StartWorker(workerCtx, mockSMTP)

			// Wait for processing
			time.Sleep(500 * time.Millisecond)
			workerCancel()

			// Verify email was captured
			Expect(mockSMTP.Count()).To(Equal(1))

			emails := mockSMTP.Emails()
			Expect(emails).To(HaveLen(1))
			Expect(emails[0].To).To(ContainElement("recipient@example.com"))
			Expect(emails[0].Subject).To(Equal("Test Email"))
			Expect(emails[0].Body).To(Equal("Test email body"))
		})

		It("should capture email with HTML body", func() {
			tenantID := support.NewTestTenantID()
			jobData := support.NewTestEmailJob(tenantID)
			jobData["html_body"] = "<html><body><h1>Test</h1></body></html>"

			toStr := fmt.Sprintf("{%s}", jobData["to_addresses"].([]string)[0])

			// Insert job
			query := `
				INSERT INTO email_jobs (id, tenant_id, to_addresses, subject, body, html_body, attempts, max_retries, next_retry, created_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
			`

			_, err := pool.Exec(ctx, query,
				jobData["id"],
				jobData["tenant_id"],
				toStr,
				jobData["subject"],
				jobData["body"],
				jobData["html_body"],
				jobData["attempts"],
				jobData["max_retries"],
				jobData["next_retry"],
				jobData["created_at"],
			)
			Expect(err).NotTo(HaveOccurred())

			// Create queue and process
			emailQueue := queue.NewEmailQueue(pool)
			workerCtx, workerCancel := context.WithCancel(context.Background())
			defer workerCancel()
			emailQueue.StartWorker(workerCtx, mockSMTP)

			// Wait for processing
			time.Sleep(500 * time.Millisecond)
			workerCancel()

			// Verify HTML body was captured
			Expect(mockSMTP.Count()).To(Equal(1))
			emails := mockSMTP.Emails()
			Expect(emails[0].HTMLBody).To(ContainSubstring("<h1>Test</h1>"))
		})
	})
})
