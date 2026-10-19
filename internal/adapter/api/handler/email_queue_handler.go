package handler

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/hekemen/automata/internal/infrastructure/queue"
)

// emailQueueItem is the unified JSON shape for pending jobs and history rows.
type emailQueueItem struct {
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

type EmailQueueHandler struct {
	q queue.Queue
}

func NewEmailQueueHandler(q queue.Queue) *EmailQueueHandler {
	return &EmailQueueHandler{q: q}
}

// List handles GET /api/admin/contexts/:id/email-queue — returns pending
// jobs (default) or sent/failed history for a context.
func (h *EmailQueueHandler) List(c *gin.Context) {
	contextID := c.Param("id")
	if _, err := uuid.Parse(contextID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid context ID"})
		return
	}

	status := c.DefaultQuery("status", "pending")
	switch status {
	case "pending", "sent", "failed":
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "status must be one of: pending, sent, failed"})
		return
	}

	limit := 100
	if v := c.Query("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid limit"})
			return
		}
		limit = n
	}
	if limit > 500 {
		limit = 500
	}

	items := []emailQueueItem{}
	if status == "pending" {
		jobs, err := h.q.ListPending(contextID, limit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		for _, j := range jobs {
			items = append(items, emailQueueItem{
				ID:         j.ID,
				ContextID:  j.ContextID,
				To:         j.To,
				Subject:    j.Subject,
				Attempts:   j.Attempts,
				MaxRetries: j.MaxRetries,
				Status:     "pending",
				CreatedAt:  j.CreatedAt,
			})
		}
	} else {
		history, err := h.q.ListHistory(contextID, status, limit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		for _, row := range history {
			items = append(items, emailQueueItem{
				ID:         row.ID,
				ContextID:  row.ContextID,
				To:         row.To,
				Subject:    row.Subject,
				Attempts:   row.Attempts,
				Status:     row.Status,
				LastError:  row.LastError,
				CreatedAt:  row.CreatedAt,
				FinishedAt: &row.FinishedAt,
			})
		}
	}

	c.JSON(http.StatusOK, gin.H{"jobs": items, "count": len(items)})
}

// Retry handles POST /api/admin/contexts/:id/email-queue/:jobId/retry —
// re-enqueues a failed job.
func (h *EmailQueueHandler) Retry(c *gin.Context) {
	contextID := c.Param("id")
	if _, err := uuid.Parse(contextID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid context ID"})
		return
	}
	jobID := c.Param("jobId")
	if err := h.q.Retry(contextID, jobID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"id": jobID})
}

// Cancel handles DELETE /api/admin/contexts/:id/email-queue/:jobId —
// removes a pending job.
func (h *EmailQueueHandler) Cancel(c *gin.Context) {
	contextID := c.Param("id")
	if _, err := uuid.Parse(contextID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid context ID"})
		return
	}
	jobID := c.Param("jobId")
	if err := h.q.Cancel(contextID, jobID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

// TestSend handles POST /api/admin/contexts/:id/email-queue/test — enqueues
// a test email to the given address.
func (h *EmailQueueHandler) TestSend(c *gin.Context) {
	contextID := c.Param("id")
	if _, err := uuid.Parse(contextID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid context ID"})
		return
	}
	var req struct {
		To string `json:"to"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if strings.TrimSpace(req.To) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "to is required"})
		return
	}

	job := &queue.EmailJob{
		ContextID:  contextID,
		To:         []string{req.To},
		Subject:    "Automata SMTP test",
		Body:       "This is a test email from Automata.",
		MaxRetries: 3,
	}
	if err := h.q.Enqueue(job); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"job_id": job.ID})
}
