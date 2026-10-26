package cicd_test

import (
	"context"
	"net/http"
	"time"

	"github.com/hekemen/automata/cicd/support"
	"github.com/hekemen/automata/internal/infrastructure/queue"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var (
	whServer     *support.MockWebhookServer
	workerCtx    context.Context
	workerCancel context.CancelFunc
	workerDone   chan struct{}
)

var _ = BeforeEach(func() {
	whServer = support.NewMockWebhookServer()
	workerCtx, workerCancel = context.WithCancel(context.Background())
	workerDone = nil
})

var _ = Describe("Webhook Delivery Worker", func() {

	AfterEach(func() {
		if workerDone != nil {
			workerCancel()
			<-workerDone
		}
		_, err := pool.Exec(ctx, "DELETE FROM webhook_deliveries")
		Expect(err).NotTo(HaveOccurred())
		whServer.Clear()
	})

	// startWorker runs the worker at a 1s poll interval (test seam) and
	// records completion so AfterEach can stop it deterministically.
	startWorker := func() {
		workerDone = make(chan struct{})
		w := queue.NewWebhookWorkerWithInterval(queue.NewWebhookQueue(pool), time.Second)
		go func() {
			w.Start(workerCtx)
			close(workerDone)
		}()
	}

	It("delivers a queued webhook with the standard headers", func() {
		contextID := support.NewTestContextID()
		formID := support.NewTestUUID()
		payload := map[string]interface{}{"contact_email": "worker-test@example.com"}
		_, err := queue.NewWebhookQueue(pool).Enqueue(ctx, whServer.URL(), payload, contextID, formID)
		Expect(err).To(Succeed())

		startWorker()

		var status string
		Eventually(func() string {
			err := pool.QueryRow(ctx, "SELECT status FROM webhook_deliveries WHERE context_id = $1", contextID).Scan(&status)
			Expect(err).NotTo(HaveOccurred())
			return status
		}, 10*time.Second, 200*time.Millisecond).Should(Equal("success"))

		Expect(whServer.Count()).To(Equal(1))
		req, ok := whServer.LastRequest()
		Expect(ok).To(BeTrue())
		// Header names are captured in canonical MIME form (RFC 7230: case-insensitive).
		Expect(req.Headers["X-Webhook-Source"]).To(Equal("automata"))
		Expect(req.Headers["X-Context-Id"]).To(Equal(contextID))
		Expect(req.Headers["X-Form-Id"]).To(Equal(formID))
		Expect(req.Payload["contact_email"]).To(Equal("worker-test@example.com"))
	})

	It("schedules a retry when the target returns 5xx or is unreachable", func() {
		contextID := support.NewTestContextID()
		formID := support.NewTestUUID()
		whServer.SetResponseStatusCode(http.StatusServiceUnavailable)
		_, err := queue.NewWebhookQueue(pool).Enqueue(ctx, whServer.URL(), map[string]interface{}{"t": 1}, contextID, formID)
		Expect(err).To(Succeed())
		_, err = queue.NewWebhookQueue(pool).Enqueue(ctx, "http://127.0.0.1:1", map[string]interface{}{"t": 2}, contextID, formID)
		Expect(err).To(Succeed())

		startWorker()

		Eventually(func() int {
			var n int
			Expect(pool.QueryRow(ctx,
				"SELECT COUNT(*) FROM webhook_deliveries WHERE context_id = $1 AND status = 'retrying'",
				contextID).Scan(&n)).To(Succeed())
			return n
		}, 10*time.Second, 200*time.Millisecond).Should(Equal(2))

		var attempts int
		Expect(pool.QueryRow(ctx,
			"SELECT attempts FROM webhook_deliveries WHERE context_id = $1 AND url = $2",
			contextID, whServer.URL()).Scan(&attempts)).To(Succeed())
		Expect(attempts).To(Equal(1))
	})

	It("marks a delivery failed after max retries", func() {
		contextID := support.NewTestContextID()
		formID := support.NewTestUUID()
		whServer.SetResponseStatusCode(http.StatusServiceUnavailable)
		// Pre-seed at attempts = max_retries - 1 so the next failure is terminal.
		_, err := pool.Exec(ctx, `
			INSERT INTO webhook_deliveries (id, context_id, form_id, url, payload, status, attempts, max_retries, next_retry, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, 'pending', 2, 3, NOW(), NOW(), NOW())`,
			support.NewTestUUID(), contextID, formID, whServer.URL(), []byte(`{"t":3}`))
		Expect(err).NotTo(HaveOccurred())

		startWorker()

		var status string
		Eventually(func() string {
			err := pool.QueryRow(ctx, "SELECT status FROM webhook_deliveries WHERE context_id = $1", contextID).Scan(&status)
			Expect(err).NotTo(HaveOccurred())
			return status
		}, 10*time.Second, 200*time.Millisecond).Should(Equal("failed"))

		var errMsg string
		Expect(pool.QueryRow(ctx, "SELECT error_msg FROM webhook_deliveries WHERE context_id = $1", contextID).Scan(&errMsg)).To(Succeed())
		Expect(errMsg).To(ContainSubstring("unexpected status: 503"))
	})
})