package cicd_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/hekemen/automata/cicd/support"
	bhandler "github.com/hekemen/automata/internal/adapter/api/handler"
	mailsmtp "github.com/hekemen/automata/internal/infrastructure/email/smtp"
	"github.com/hekemen/automata/internal/infrastructure/queue"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func setupEmailQueueEngine() *gin.Engine {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	h := bhandler.NewEmailQueueHandler(queue.NewEmailQueue(pool))
	admin := engine.Group("/api/admin")
	admin.GET("/contexts/:id/email-queue", h.List)
	admin.POST("/contexts/:id/email-queue/:jobId/retry", h.Retry)
	admin.DELETE("/contexts/:id/email-queue/:jobId", h.Cancel)
	admin.POST("/contexts/:id/email-queue/test", h.TestSend)
	return engine
}

type apiQueueItem struct {
	ID         string     `json:"id"`
	ContextID  string     `json:"context_id"`
	To         []string   `json:"to"`
	Subject    string     `json:"subject"`
	Attempts   int        `json:"attempts"`
	MaxRetries int        `json:"max_retries"`
	Status     string     `json:"status"`
	LastError  string     `json:"last_error"`
	CreatedAt  time.Time  `json:"created_at"`
	FinishedAt *time.Time `json:"finished_at"`
}

type apiQueueListResponse struct {
	Jobs  []apiQueueItem `json:"jobs"`
	Count int            `json:"count"`
}

var _ = Describe("Email Queue API", func() {
	var (
		w      *httptest.ResponseRecorder
		engine *gin.Engine
		q      queue.Queue
	)

	BeforeEach(func() {
		w = httptest.NewRecorder()
		q = queue.NewEmailQueue(pool)
		engine = setupEmailQueueEngine()
	})

	AfterEach(func() {
		_, err := pool.Exec(ctx, "DELETE FROM email_jobs")
		Expect(err).NotTo(HaveOccurred())
		_, err = pool.Exec(ctx, "DELETE FROM email_history")
		Expect(err).NotTo(HaveOccurred())
	})

	insertJob := func(contextID, to, subject, body string, maxRetries int) string {
		id := uuid.NewString()
		query := `
			INSERT INTO email_jobs (id, context_id, to_addresses, subject, body, html_body, attempts, max_retries, next_retry, created_at)
			VALUES ($1, $2, $3, $4, $5, '', 0, $6, NOW(), NOW())`
		_, err := pool.Exec(ctx, query, id, contextID, fmt.Sprintf("{%s}", to), subject, body, maxRetries)
		Expect(err).NotTo(HaveOccurred())
		return id
	}

	insertFailedHistory := func(contextID, to, subject, body, lastError string) string {
		id := uuid.NewString()
		query := `
			INSERT INTO email_history (id, context_id, to_addresses, subject, body, html_body, attempts, status, last_error, created_at, finished_at)
			VALUES ($1, $2, $3, $4, $5, '', 1, 'failed', $6, NOW(), NOW())`
		_, err := pool.Exec(ctx, query, id, contextID, fmt.Sprintf("{%s}", to), subject, body, lastError)
		Expect(err).NotTo(HaveOccurred())
		return id
	}

	countPending := func(contextID string) int {
		var n int
		Expect(pool.QueryRow(ctx, "SELECT COUNT(*) FROM email_jobs WHERE context_id = $1", contextID).Scan(&n)).To(Succeed())
		return n
	}

	waitDrained := func(contextID string) {
		Eventually(func() int {
			return countPending(contextID)
		}, 2*time.Second, 100*time.Millisecond).Should(BeZero())
	}

	listQueue := func(contextID, status string) apiQueueListResponse {
		path := fmt.Sprintf("/api/admin/contexts/%s/email-queue", contextID)
		if status != "" {
			path += "?status=" + status
		}
		rw := httptest.NewRecorder()
		req := httptest.NewRequest("GET", path, nil)
		engine.ServeHTTP(rw, req)
		Expect(rw.Code).To(Equal(http.StatusOK))
		var resp apiQueueListResponse
		Expect(json.Unmarshal(rw.Body.Bytes(), &resp)).To(Succeed())
		return resp
	}

	It("lists pending jobs after enqueue", func() {
		contextID := support.NewTestContextID()
		job := &queue.EmailJob{
			ContextID: contextID,
			To:        []string{"pending@example.com"},
			Subject:   "Pending Job",
			Body:      "body",
		}
		Expect(q.Enqueue(job)).To(Succeed())

		resp := listQueue(contextID, "")
		Expect(resp.Count).To(Equal(1))
		Expect(resp.Jobs).To(HaveLen(1))
		Expect(resp.Jobs[0].ID).To(Equal(job.ID))
		Expect(resp.Jobs[0].ContextID).To(Equal(contextID))
		Expect(resp.Jobs[0].To).To(ContainElement("pending@example.com"))
		Expect(resp.Jobs[0].Subject).To(Equal("Pending Job"))
		Expect(resp.Jobs[0].Status).To(Equal("pending"))
		Expect(resp.Jobs[0].MaxRetries).To(Equal(3))
		Expect(resp.Jobs[0].FinishedAt).To(BeNil())
	})

	It("lists sent history after the worker processes a job", func() {
		contextID := support.NewTestContextID()
		localMock := support.NewMockSMTP()
		Expect(localMock.Start()).To(Succeed())
		defer localMock.Clear()

		jobID := insertJob(contextID, "sent@example.com", "Sent Job", "body", 3)

		queueCtx, queueCancel := context.WithCancel(ctx)
		defer queueCancel()
		queue.NewEmailQueue(pool).StartWorker(queueCtx, localMock)

		waitDrained(contextID)

		resp := listQueue(contextID, "sent")
		Expect(resp.Count).To(Equal(1))
		Expect(resp.Jobs[0].ID).To(Equal(jobID))
		Expect(resp.Jobs[0].Status).To(Equal("sent"))
		Expect(resp.Jobs[0].FinishedAt).NotTo(BeNil())
	})

	It("lists failed history with last_error after max retries", func() {
		contextID := support.NewTestContextID()
		localMock := support.NewMockSMTP()
		Expect(localMock.Start()).To(Succeed())
		defer localMock.Clear()

		mailer := mailsmtp.NewMailerWithDialer(
			mailsmtp.Config{}, // unconfigured: Send fails without dialing
			&dialerAdapter{d: support.NewSMTPDialer(localMock)},
		)

		jobID := insertJob(contextID, "failed@example.com", "Failed Job", "body", 1)

		queueCtx, queueCancel := context.WithCancel(ctx)
		defer queueCancel()
		queue.NewEmailQueue(pool).StartWorker(queueCtx, mailer)

		waitDrained(contextID)

		resp := listQueue(contextID, "failed")
		Expect(resp.Count).To(Equal(1))
		Expect(resp.Jobs[0].ID).To(Equal(jobID))
		Expect(resp.Jobs[0].Status).To(Equal("failed"))
		Expect(resp.Jobs[0].LastError).NotTo(BeEmpty())
		Expect(resp.Jobs[0].FinishedAt).NotTo(BeNil())
	})

	It("rejects an invalid status with 400", func() {
		contextID := support.NewTestContextID()
		req := httptest.NewRequest("GET",
			fmt.Sprintf("/api/admin/contexts/%s/email-queue?status=bogus", contextID), nil)
		engine.ServeHTTP(w, req)
		Expect(w.Code).To(Equal(http.StatusBadRequest))
	})

	It("rejects an invalid context ID with 400", func() {
		req := httptest.NewRequest("GET", "/api/admin/contexts/not-a-uuid/email-queue", nil)
		engine.ServeHTTP(w, req)
		Expect(w.Code).To(Equal(http.StatusBadRequest))
	})

	It("retries a failed job and it reappears as pending", func() {
		contextID := support.NewTestContextID()
		historyID := insertFailedHistory(contextID, "retry@example.com", "Retry Job", "body", "smtp down")

		req := httptest.NewRequest("POST",
			fmt.Sprintf("/api/admin/contexts/%s/email-queue/%s/retry", contextID, historyID), nil)
		engine.ServeHTTP(w, req)
		Expect(w.Code).To(Equal(http.StatusOK))
		Expect(w.Body.String()).To(ContainSubstring(historyID))

		resp := listQueue(contextID, "")
		Expect(resp.Count).To(Equal(1))
		Expect(resp.Jobs[0].ID).To(Equal(historyID))
		Expect(resp.Jobs[0].Status).To(Equal("pending"))
	})

	It("returns 400 when retrying a job with a malformed ID", func() {
		contextID := support.NewTestContextID()
		req := httptest.NewRequest("POST",
			fmt.Sprintf("/api/admin/contexts/%s/email-queue/nope/retry", contextID), nil)
		engine.ServeHTTP(w, req)
		Expect(w.Code).To(Equal(http.StatusBadRequest))
	})

	It("returns 404 when retrying a missing job", func() {
		contextID := support.NewTestContextID()
		missingID := support.NewTestUUID()
		req := httptest.NewRequest("POST",
			fmt.Sprintf("/api/admin/contexts/%s/email-queue/%s/retry", contextID, missingID), nil)
		engine.ServeHTTP(w, req)
		Expect(w.Code).To(Equal(http.StatusNotFound))
	})

	It("cancels a pending job and it disappears", func() {
		contextID := support.NewTestContextID()
		job := &queue.EmailJob{
			ContextID: contextID,
			To:        []string{"cancel@example.com"},
			Subject:   "Cancel Job",
			Body:      "body",
		}
		Expect(q.Enqueue(job)).To(Succeed())

		req := httptest.NewRequest("DELETE",
			fmt.Sprintf("/api/admin/contexts/%s/email-queue/%s", contextID, job.ID), nil)
		engine.ServeHTTP(w, req)
		Expect(w.Code).To(Equal(http.StatusNoContent))

		resp := listQueue(contextID, "")
		Expect(resp.Count).To(BeZero())
	})

	It("returns 400 when cancelling a job with a malformed ID", func() {
		contextID := support.NewTestContextID()
		req := httptest.NewRequest("DELETE",
			fmt.Sprintf("/api/admin/contexts/%s/email-queue/nope", contextID), nil)
		engine.ServeHTTP(w, req)
		Expect(w.Code).To(Equal(http.StatusBadRequest))
	})

	It("returns 404 when cancelling a missing job", func() {
		contextID := support.NewTestContextID()
		missingID := support.NewTestUUID()
		req := httptest.NewRequest("DELETE",
			fmt.Sprintf("/api/admin/contexts/%s/email-queue/%s", contextID, missingID), nil)
		engine.ServeHTTP(w, req)
		Expect(w.Code).To(Equal(http.StatusNotFound))
	})

	It("enqueues a test email and it appears in pending", func() {
		contextID := support.NewTestContextID()
		req := httptest.NewRequest("POST",
			fmt.Sprintf("/api/admin/contexts/%s/email-queue/test", contextID),
			bytes.NewBufferString(`{"to":"test-recv@example.com"}`))
		req.Header.Set("Content-Type", "application/json")
		engine.ServeHTTP(w, req)
		Expect(w.Code).To(Equal(http.StatusAccepted))
		Expect(w.Body.String()).To(ContainSubstring("job_id"))

		resp := listQueue(contextID, "")
		Expect(resp.Count).To(Equal(1))
		Expect(resp.Jobs[0].Subject).To(Equal("Automata SMTP test"))
		Expect(resp.Jobs[0].To).To(ContainElement("test-recv@example.com"))
		Expect(resp.Jobs[0].MaxRetries).To(Equal(3))
	})

	It("rejects a test email with an empty to address", func() {
		contextID := support.NewTestContextID()
		req := httptest.NewRequest("POST",
			fmt.Sprintf("/api/admin/contexts/%s/email-queue/test", contextID),
			bytes.NewBufferString(`{"to":""}`))
		req.Header.Set("Content-Type", "application/json")
		engine.ServeHTTP(w, req)
		Expect(w.Code).To(Equal(http.StatusBadRequest))
	})
})
