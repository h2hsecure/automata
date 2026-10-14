package cicd_test

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hekemen/automata/cicd/support"
	mailsmtp "github.com/hekemen/automata/internal/infrastructure/email/smtp"
	"github.com/hekemen/automata/internal/infrastructure/queue"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// dialerAdapter wraps support.SMTPDialer to satisfy the smtp.Dialer interface.
// *support.SMTPClient already satisfies smtp.Client (Mail/Rcpt/Data/Close).
type dialerAdapter struct {
	d *support.SMTPDialer
}

func (a *dialerAdapter) Dial() (mailsmtp.Client, error) { return a.d.Dial() }

var _ = Describe("Email Delivery Pipeline", func() {

	AfterEach(func() {
		_, err := pool.Exec(ctx, "DELETE FROM email_jobs")
		Expect(err).NotTo(HaveOccurred())
		mockSMTP.Clear()
	})

	insertJob := func(contextID, to, subject, body, htmlBody string, maxRetries int) {
		query := `
			INSERT INTO email_jobs (id, context_id, to_addresses, subject, body, html_body, attempts, max_retries, next_retry, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, 0, $7, NOW(), NOW())`
		_, err := pool.Exec(ctx, query,
			uuid.NewString(), contextID, fmt.Sprintf("{%s}", to),
			subject, body, htmlBody, maxRetries)
		Expect(err).NotTo(HaveOccurred())
	}

	waitQueueDrained := func() {
		Eventually(func() int {
			var n int
			Expect(pool.QueryRow(ctx, "SELECT COUNT(*) FROM email_jobs").Scan(&n)).To(Succeed())
			return n
		}, 2*time.Second, 100*time.Millisecond).Should(BeZero())
	}

	It("delivers a queued email through the SMTP mailer", func() {
		contextID := support.NewTestContextID()
		to := "deliver-test@example.com"
		insertJob(contextID, to, "Delivery Test", "plain body", "", 3)

		mailer := mailsmtp.NewMailerWithDialer(
			mailsmtp.Config{Host: "mock", From: "noreply@automata.local"},
			&dialerAdapter{d: support.NewSMTPDialer(mockSMTP)})

		queueCtx, queueCancel := context.WithCancel(ctx)
		defer queueCancel()
		queue.NewEmailQueue(pool).StartWorker(queueCtx, mailer)

		waitQueueDrained()

		Expect(mockSMTP.HasEmailTo(to)).To(BeTrue())

		last, ok := mockSMTP.LastEmail()
		Expect(ok).To(BeTrue())
		Expect(last.Subject).To(Equal("delivery test")) // mock lowercases header values
		Expect(last.Body).To(ContainSubstring("plain body"))
	})

	It("delivers multipart when html body is present", func() {
		contextID := support.NewTestContextID()
		to := "deliver-multipart@example.com"
		insertJob(contextID, to, "Multipart Test", "plain body", "<h1>Test</h1>", 3)

		mailer := mailsmtp.NewMailerWithDialer(
			mailsmtp.Config{Host: "mock", From: "noreply@automata.local"},
			&dialerAdapter{d: support.NewSMTPDialer(mockSMTP)})

		queueCtx, queueCancel := context.WithCancel(ctx)
		defer queueCancel()
		queue.NewEmailQueue(pool).StartWorker(queueCtx, mailer)

		waitQueueDrained()

		last, ok := mockSMTP.LastEmail()
		Expect(ok).To(BeTrue())
		// The mock captures the payload after the first blank line, which
		// includes the multipart part headers.
		Expect(last.Body).To(ContainSubstring("Content-Type: text/html"))
		Expect(last.Body).To(ContainSubstring("<h1>Test</h1>"))
	})

	It("discards unconfigured SMTP jobs after max retries (fast discard)", func() {
		contextID := support.NewTestContextID()
		to := "discard@example.com"
		insertJob(contextID, to, "Discard Test", "body", "", 1)

		mailer := mailsmtp.NewMailerWithDialer(
			mailsmtp.Config{}, // unconfigured: Send fails without dialing
			&dialerAdapter{d: support.NewSMTPDialer(mockSMTP)})

		queueCtx, queueCancel := context.WithCancel(ctx)
		defer queueCancel()
		queue.NewEmailQueue(pool).StartWorker(queueCtx, mailer)

		waitQueueDrained()

		Expect(mockSMTP.Count()).To(BeZero())
	})

	It("delivers a large HTML body (~100KB)", func() {
		contextID := support.NewTestContextID()
		to := "deliver-large@example.com"
		body := strings.Repeat("A", 50000)
		htmlBody := "<html>" + strings.Repeat("B", 99000) + "</html>"
		insertJob(contextID, to, "Large Test", body, htmlBody, 3)

		mailer := mailsmtp.NewMailerWithDialer(
			mailsmtp.Config{Host: "mock", From: "noreply@automata.local"},
			&dialerAdapter{d: support.NewSMTPDialer(mockSMTP)})

		queueCtx, queueCancel := context.WithCancel(ctx)
		defer queueCancel()
		queue.NewEmailQueue(pool).StartWorker(queueCtx, mailer)

		waitQueueDrained()

		last, ok := mockSMTP.LastEmail()
		Expect(ok).To(BeTrue())
		Expect(len(last.Body)).To(BeNumerically(">=", 100000))
		Expect(last.Body).To(ContainSubstring("BBBB"))
	})
})