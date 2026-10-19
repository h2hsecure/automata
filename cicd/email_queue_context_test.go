package cicd_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/hekemen/automata/cicd/support"
	mailsmtp "github.com/hekemen/automata/internal/infrastructure/email/smtp"
	"github.com/hekemen/automata/internal/infrastructure/queue"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// recordingFailingMailer always fails and records the context IDs it was
// called with, so tests can verify per-context batch selection.
type recordingFailingMailer struct {
	mu     sync.Mutex
	called []string
}

func (m *recordingFailingMailer) Send(contextID string, to []string, subject, body, htmlBody string) error {
	m.mu.Lock()
	m.called = append(m.called, contextID)
	m.mu.Unlock()
	return errors.New("smtp down")
}

func (m *recordingFailingMailer) callsFor(contextID string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := 0
	for _, c := range m.called {
		if c == contextID {
			n++
		}
	}
	return n
}

var _ = Describe("Email Queue Context Isolation and Management", func() {

	AfterEach(func() {
		_, err := pool.Exec(ctx, "DELETE FROM email_jobs")
		Expect(err).NotTo(HaveOccurred())
		_, err = pool.Exec(ctx, "DELETE FROM email_history")
		Expect(err).NotTo(HaveOccurred())
	})

	insertJob := func(contextID, to, subject, body string, maxRetries int) string {
		jobID := uuid.NewString()
		query := `
			INSERT INTO email_jobs (id, context_id, to_addresses, subject, body, html_body, attempts, max_retries, next_retry, created_at)
			VALUES ($1, $2, $3, $4, $5, '', 0, $6, NOW(), NOW())`
		_, err := pool.Exec(ctx, query, jobID, contextID, fmt.Sprintf("{%s}", to), subject, body, maxRetries)
		Expect(err).NotTo(HaveOccurred())
		return jobID
	}

	insertHistory := func(contextID, to, subject, body, status, lastError string, attempts int, finishedAt time.Time) string {
		id := uuid.NewString()
		query := `
			INSERT INTO email_history (id, context_id, to_addresses, subject, body, html_body, attempts, status, last_error, created_at, finished_at)
			VALUES ($1, $2, $3, $4, $5, '', $6, $7, $8, NOW(), $9)`
		_, err := pool.Exec(ctx, query, id, contextID, fmt.Sprintf("{%s}", to), subject, body, attempts, status, lastError, finishedAt)
		Expect(err).NotTo(HaveOccurred())
		return id
	}

	countJobs := func(contextID string) int {
		var n int
		Expect(pool.QueryRow(ctx, "SELECT COUNT(*) FROM email_jobs WHERE context_id = $1", contextID).Scan(&n)).To(Succeed())
		return n
	}

	waitContextDrained := func(contextID string) {
		Eventually(func() int {
			return countJobs(contextID)
		}, 2*time.Second, 100*time.Millisecond).Should(BeZero())
	}

	It("records sent history and deletes the job on success", func() {
		contextID := support.NewTestContextID()
		localMock := support.NewMockSMTP()
		Expect(localMock.Start()).To(Succeed())
		defer localMock.Clear()

		jobID := insertJob(contextID, "ctx-send@example.com", "Ctx Send", "body", 3)

		queueCtx, queueCancel := context.WithCancel(ctx)
		defer queueCancel()
		queue.NewEmailQueue(pool).StartWorker(queueCtx, localMock)

		waitContextDrained(contextID)

		var status string
		var historyContext string
		Expect(pool.QueryRow(ctx, "SELECT status, context_id FROM email_history WHERE id = $1", jobID).Scan(&status, &historyContext)).To(Succeed())
		Expect(status).To(Equal("sent"))
		Expect(historyContext).To(Equal(contextID))

		Expect(localMock.Count()).To(Equal(1))
		emails := localMock.Emails()
		Expect(emails).To(HaveLen(1))
		Expect(emails[0].ContextID).To(Equal(contextID))
		Expect(emails[0].To).To(ContainElement("ctx-send@example.com"))
	})

	It("records failed history with last_error after max retries", func() {
		contextID := support.NewTestContextID()
		localMock := support.NewMockSMTP()
		Expect(localMock.Start()).To(Succeed())
		defer localMock.Clear()

		mailer := mailsmtp.NewMailerWithDialer(
			mailsmtp.Config{}, // unconfigured: Send fails without dialing
			&dialerAdapter{d: support.NewSMTPDialer(localMock)},
		)

		jobID := insertJob(contextID, "ctx-fail@example.com", "Ctx Fail", "body", 1)

		queueCtx, queueCancel := context.WithCancel(ctx)
		defer queueCancel()
		queue.NewEmailQueue(pool).StartWorker(queueCtx, mailer)

		waitContextDrained(contextID)

		var status, lastError string
		Expect(pool.QueryRow(ctx, "SELECT status, last_error FROM email_history WHERE id = $1", jobID).Scan(&status, &lastError)).To(Succeed())
		Expect(status).To(Equal("failed"))
		Expect(lastError).NotTo(BeEmpty())

		Expect(localMock.Count()).To(BeZero())
	})

	It("caps a single context at 3 jobs per batch (starvation guard)", func() {
		contextA := support.NewTestContextID()
		contextB := support.NewTestContextID()

		for i := 0; i < 10; i++ {
			insertJob(contextA, fmt.Sprintf("a%d@example.com", i), "A", "body", 3)
		}
		insertJob(contextB, "b@example.com", "B", "body", 3)

		mailer := &recordingFailingMailer{}

		queueCtx, queueCancel := context.WithCancel(ctx)
		queue.NewEmailQueue(pool).StartWorker(queueCtx, mailer)
		// Wait for the first batch to complete (B gets attempted), then stop
		// before the next tick so exactly one batch is observed.
		Eventually(func() int { return mailer.callsFor(contextB) }, 1*time.Second, 10*time.Millisecond).Should(Equal(1))
		queueCancel()

		// B's single job must have been attempted (not starved by A).
		Expect(mailer.callsFor(contextB)).To(Equal(1))
		// A is capped at 3 jobs in the single batch.
		Expect(mailer.callsFor(contextA)).To(Equal(3))

		// Nothing completed, so all jobs remain (with backoff).
		Expect(countJobs(contextA)).To(Equal(10))
		Expect(countJobs(contextB)).To(Equal(1))

		var historyCount int
		Expect(pool.QueryRow(ctx, "SELECT COUNT(*) FROM email_history").Scan(&historyCount)).To(Succeed())
		Expect(historyCount).To(BeZero())
	})

	It("Retry re-enqueues a failed job with attempts reset and removes the history row", func() {
		contextID := support.NewTestContextID()
		q := queue.NewEmailQueue(pool)

		historyID := insertHistory(contextID, "retry@example.com", "Retry Me", "body", "failed", "smtp down", 1, time.Now())

		Expect(q.Retry(contextID, historyID)).To(Succeed())

		var historyCount int
		Expect(pool.QueryRow(ctx, "SELECT COUNT(*) FROM email_history WHERE id = $1", historyID).Scan(&historyCount)).To(Succeed())
		Expect(historyCount).To(BeZero())

		var attempts int
		var jobContext string
		var subject string
		Expect(pool.QueryRow(ctx, "SELECT attempts, context_id, subject FROM email_jobs WHERE id = $1", historyID).Scan(&attempts, &jobContext, &subject)).To(Succeed())
		Expect(attempts).To(BeZero())
		Expect(jobContext).To(Equal(contextID))
		Expect(subject).To(Equal("Retry Me"))

		Expect(q.Retry(contextID, uuid.NewString())).To(HaveOccurred())
	})

	It("Cancel removes a pending job and errors on missing ids", func() {
		contextID := support.NewTestContextID()
		q := queue.NewEmailQueue(pool)

		job := &queue.EmailJob{
			ContextID: contextID,
			To:        []string{"cancel@example.com"},
			Subject:   "Cancel Me",
			Body:      "body",
		}
		Expect(q.Enqueue(job)).To(Succeed())

		Expect(q.Cancel(contextID, job.ID)).To(Succeed())
		Expect(countJobs(contextID)).To(BeZero())

		Expect(q.Cancel(contextID, uuid.NewString())).To(HaveOccurred())
	})

	It("PruneHistory removes only history older than the cutoff", func() {
		contextID := support.NewTestContextID()
		q := queue.NewEmailQueue(pool)

		oldID := insertHistory(contextID, "old@example.com", "Old", "body", "sent", "", 1, time.Now().Add(-40*24*time.Hour))
		newID := insertHistory(contextID, "new@example.com", "New", "body", "sent", "", 1, time.Now())

		removed, err := q.PruneHistory(time.Now().Add(-30 * 24 * time.Hour))
		Expect(err).NotTo(HaveOccurred())
		Expect(removed).To(BeEquivalentTo(1))

		var oldCount, newCount int
		Expect(pool.QueryRow(ctx, "SELECT COUNT(*) FROM email_history WHERE id = $1", oldID).Scan(&oldCount)).To(Succeed())
		Expect(pool.QueryRow(ctx, "SELECT COUNT(*) FROM email_history WHERE id = $1", newID).Scan(&newCount)).To(Succeed())
		Expect(oldCount).To(BeZero())
		Expect(newCount).To(Equal(1))
	})

	It("ListPending and ListHistory are scoped to the requested context", func() {
		contextA := support.NewTestContextID()
		contextB := support.NewTestContextID()
		q := queue.NewEmailQueue(pool)

		insertJob(contextA, "a1@example.com", "A1", "body", 3)
		insertJob(contextA, "a2@example.com", "A2", "body", 3)
		insertJob(contextB, "b1@example.com", "B1", "body", 3)

		insertHistory(contextA, "a1@example.com", "A1", "body", "sent", "", 1, time.Now())
		insertHistory(contextB, "b1@example.com", "B1", "body", "failed", "boom", 1, time.Now())

		pendingA, err := q.ListPending(contextA, 10)
		Expect(err).NotTo(HaveOccurred())
		Expect(pendingA).To(HaveLen(2))
		for _, j := range pendingA {
			Expect(j.ContextID).To(Equal(contextA))
		}

		pendingB, err := q.ListPending(contextB, 10)
		Expect(err).NotTo(HaveOccurred())
		Expect(pendingB).To(HaveLen(1))
		Expect(pendingB[0].ContextID).To(Equal(contextB))

		historyA, err := q.ListHistory(contextA, "", 10)
		Expect(err).NotTo(HaveOccurred())
		Expect(historyA).To(HaveLen(1))
		Expect(historyA[0].Status).To(Equal("sent"))

		sentA, err := q.ListHistory(contextA, "sent", 10)
		Expect(err).NotTo(HaveOccurred())
		Expect(sentA).To(HaveLen(1))

		failedA, err := q.ListHistory(contextA, "failed", 10)
		Expect(err).NotTo(HaveOccurred())
		Expect(failedA).To(BeEmpty())

		historyB, err := q.ListHistory(contextB, "", 10)
		Expect(err).NotTo(HaveOccurred())
		Expect(historyB).To(HaveLen(1))
		Expect(historyB[0].Status).To(Equal("failed"))
	})
})
