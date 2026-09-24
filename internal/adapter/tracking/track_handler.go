package tracking

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/hekemen/automata/internal/domain/tracking"
	trackingUsecase "github.com/hekemen/automata/internal/usecase/tracking"
)

// TrackHandler handles event tracking endpoints.
type TrackHandler struct {
	repo Repository
}

// Repository is the interface for tracking event persistence.
type Repository interface {
	tracking.Repository
}

// Track handles POST /track — single event tracking.
func (h *TrackHandler) Track(c *gin.Context) {
	contextID := ResolveContext(c)
	if contextID == "" {
		return
	}

	var req struct {
		Type       string                 `json:"type"`
		URL        string                 `json:"url"`
		Title      string                 `json:"title"`
		Referrer   string                 `json:"referrer"`
		EventName  string                 `json:"event_name"`
		Properties map[string]interface{} `json:"properties"`
		VisitorID  string                 `json:"visitor_id"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userAgent := c.GetHeader("User-Agent")

	event := &tracking.Event{
		ID:          uuid.New().String(),
		ContextID:    contextID,
		VisitorID:   req.VisitorID,
		Type:        tracking.EventType(req.Type),
		URL:         req.URL,
		Title:       req.Title,
		Referrer:    req.Referrer,
		EventName:   req.EventName,
		Properties:  req.Properties,
		UserAgent:   userAgent,
		CreatedAt:   time.Now(),
	}

	if err := trackingUsecase.TrackEvent(h.repo, contextID, event); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

// TrackBatch handles POST /track/batch — batch event tracking.
func (h *TrackHandler) TrackBatch(c *gin.Context) {
	contextID := ResolveContext(c)
	if contextID == "" {
		return
	}

	var req struct {
		Events []map[string]interface{} `json:"events"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	events := make([]*tracking.Event, 0, len(req.Events))
	for _, e := range req.Events {
		event := &tracking.Event{
			ID:        uuid.New().String(),
			ContextID:  contextID,
			CreatedAt: time.Now(),
		}

		if v, ok := e["type"].(string); ok {
			event.Type = tracking.EventType(v)
		}
		if v, ok := e["url"].(string); ok {
			event.URL = v
		}
		if v, ok := e["title"].(string); ok {
			event.Title = v
		}
		if v, ok := e["referrer"].(string); ok {
			event.Referrer = v
		}
		if v, ok := e["event_name"].(string); ok {
			event.EventName = v
		}
		if v, ok := e["visitor_id"].(string); ok {
			event.VisitorID = v
		}
		if v, ok := e["properties"].(map[string]interface{}); ok {
			event.Properties = v
		}

		events = append(events, event)
	}

	if err := h.repo.CreateEventsBatch(events); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}
