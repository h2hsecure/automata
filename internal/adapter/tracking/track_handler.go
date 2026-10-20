package tracking

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	ct "github.com/hekemen/automata/internal/domain/contact"
	"github.com/hekemen/automata/internal/domain/tracking"
	trackingUsecase "github.com/hekemen/automata/internal/usecase/tracking"
	"github.com/rs/zerolog/log"
)

// TrackHandler handles event tracking endpoints.
type TrackHandler struct {
	repo        Repository
	contactRepo ContactRepository
}

// Repository is the interface for tracking event persistence.
type Repository interface {
	tracking.Repository
}

// ContactRepository is the interface for contact persistence used by tracking
// (hidden visitor contacts + page_visit activities).
type ContactRepository interface {
	ct.Repository
}

// Track handles POST /track — single event tracking.
func (h *TrackHandler) Track(c *gin.Context) {
	var req struct {
		ContextID  string                 `json:"contextId"`
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

	// Prefer a valid contextId in the body (sent by the snippet); otherwise
	// fall back to header/subdomain resolution.
	contextID := ""
	if _, err := uuid.Parse(req.ContextID); err == nil {
		contextID = req.ContextID
	} else {
		contextID = ResolveContext(c)
		if contextID == "" {
			return
		}
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
		IP:          c.ClientIP(),
		CreatedAt:   time.Now(),
	}

	if err := trackingUsecase.TrackEvent(h.contactRepo, h.repo, contextID, event); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

// TrackBatch handles POST /track/batch — batch event tracking. Each event is
// routed through TrackEvent, so pageviews also produce hidden visitor
// contacts and page_visit activities. Events with an empty type (after the
// page_view→pageview normalization) are rejected.
func (h *TrackHandler) TrackBatch(c *gin.Context) {
	var req struct {
		Events []map[string]interface{} `json:"events"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Events may carry their own valid contextId; header/subdomain resolution
	// is used as a fallback only when needed.
	var fallbackContextID string
	fallbackResolved := false

	accepted, rejected := 0, 0
	for _, e := range req.Events {
		contextID := ""
		if v, ok := e["contextId"].(string); ok {
			if _, err := uuid.Parse(v); err == nil {
				contextID = v
			}
		}
		if contextID == "" {
			if !fallbackResolved {
				fallbackContextID = ResolveContext(c)
				fallbackResolved = true
				if fallbackContextID == "" {
					return
				}
			}
			contextID = fallbackContextID
		}

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

		if event.Type == "" {
			rejected++
			continue
		}

		if err := trackingUsecase.TrackEvent(h.contactRepo, h.repo, contextID, event); err != nil {
			log.Warn().Err(err).Str("context_id", contextID).Msg("track batch: event failed")
			rejected++
			continue
		}
		accepted++
	}

	c.JSON(http.StatusOK, gin.H{"accepted": accepted, "rejected": rejected})
}
